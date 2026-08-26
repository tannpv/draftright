package email

import (
	"context"
	"testing"

	sqlc "github.com/tannpv/draftright-rewrite/internal/shared/pg/sqlc"
	"github.com/tannpv/emailkit"
)

// PgRepo is handed straight to emailkit as the Store — no adapter type. This
// fails to compile the moment a method name or signature drifts, which is the
// point: the drift would otherwise surface as a runtime wiring error in main.
func TestPgRepoSatisfiesEmailkitStore(t *testing.T) {
	var _ emailkit.Store = (*PgRepo)(nil)
}

// fakeQuerier is the single pgQuerier fake for this package: it CAPTURES the
// params PgRepo builds (so field-by-field mapping can be asserted) and lets a
// test configure what GetEmailSettings returns (so PgRepo.Credentials can be
// driven through the real adapter). One fake rather than one per test file —
// two fakes of the same six-method interface would drift the moment the
// interface gains a method.
type fakeQuerier struct {
	settingsRow sqlc.GetEmailSettingsRow
	settingsErr error

	insertLog  []sqlc.InsertEmailLogParams
	markParams []sqlc.MarkEmailByProviderIDParams
	suppress   []sqlc.SuppressEmailParams
}

func (*fakeQuerier) IsEmailSuppressed(context.Context, string) (bool, error) { return false, nil }

func (f *fakeQuerier) InsertEmailLog(_ context.Context, arg sqlc.InsertEmailLogParams) error {
	f.insertLog = append(f.insertLog, arg)
	return nil
}

func (f *fakeQuerier) GetEmailSettings(context.Context) (sqlc.GetEmailSettingsRow, error) {
	return f.settingsRow, f.settingsErr
}

func (*fakeQuerier) GetEmailTemplateByKey(context.Context, string) (sqlc.GetEmailTemplateByKeyRow, error) {
	return sqlc.GetEmailTemplateByKeyRow{}, nil
}

func (f *fakeQuerier) MarkEmailByProviderID(_ context.Context, arg sqlc.MarkEmailByProviderIDParams) error {
	f.markParams = append(f.markParams, arg)
	return nil
}

func (f *fakeQuerier) SuppressEmail(_ context.Context, arg sqlc.SuppressEmailParams) error {
	f.suppress = append(f.suppress, arg)
	return nil
}

// TestPgRepo_LogSendFieldMapping pins every field PgRepo copies from
// emailkit.SendRecord onto sqlc.InsertEmailLogParams. Both structs are all-
// string/*string, so a transposition (To into Subject, ProviderID into Error)
// compiles, type-checks, and satisfies emailkit.Store — every audit row would
// be silently wrong and TestPgRepoSatisfiesEmailkitStore could not tell. The
// values below are deliberately distinct per field so a swap cannot pass.
func TestPgRepo_LogSendFieldMapping(t *testing.T) {
	providerID, sendErr := "provider-id-value", "error-value"
	rec := emailkit.SendRecord{
		To:         "to-value@example.com",
		Type:       "type-value",
		Subject:    "subject-value",
		Status:     "status-value",
		ProviderID: &providerID,
		Error:      &sendErr,
	}

	f := &fakeQuerier{}
	if err := NewPgRepo(f).LogSend(context.Background(), rec); err != nil {
		t.Fatalf("LogSend: %v", err)
	}
	if len(f.insertLog) != 1 {
		t.Fatalf("want one InsertEmailLog call, got %d", len(f.insertLog))
	}
	got := f.insertLog[0]

	for _, c := range []struct{ field, got, want string }{
		{"ToEmail", got.ToEmail, rec.To},
		{"EmailType", got.EmailType, rec.Type},
		{"Subject", got.Subject, rec.Subject},
		{"Status", got.Status, rec.Status},
	} {
		if c.got != c.want {
			t.Errorf("InsertEmailLogParams.%s = %q, want %q", c.field, c.got, c.want)
		}
	}
	for _, c := range []struct {
		field     string
		got, want *string
	}{
		{"ProviderID", got.ProviderID, rec.ProviderID},
		{"Error", got.Error, rec.Error},
	} {
		if c.got == nil || *c.got != *c.want {
			t.Errorf("InsertEmailLogParams.%s = %v, want %q", c.field, c.got, *c.want)
		}
	}
}

// TestPgRepo_LogSendNilPointers pins the nil passthrough: a successful send
// carries no Error, a suppressed one carries no ProviderID, and both must
// reach the column as SQL NULL rather than an empty string.
func TestPgRepo_LogSendNilPointers(t *testing.T) {
	f := &fakeQuerier{}
	err := NewPgRepo(f).LogSend(context.Background(), emailkit.SendRecord{
		To: "to@example.com", Type: "t", Subject: "s", Status: "suppressed",
	})
	if err != nil {
		t.Fatalf("LogSend: %v", err)
	}
	if got := f.insertLog[0]; got.ProviderID != nil || got.Error != nil {
		t.Errorf("nil SendRecord pointers must stay nil, got ProviderID=%v Error=%v", got.ProviderID, got.Error)
	}
}

// TestPgRepo_MarkByProviderIDMapping pins the webhook update's three args.
// Same transposition risk as LogSend: id and status are both strings, and
// reason is the only *string, so status and id swapping places compiles.
// The nil-reason case is the COALESCE contract — a delivery event with no
// detail must leave the existing error column untouched, which the query can
// only do if the adapter passes SQL NULL rather than a pointer to "".
func TestPgRepo_MarkByProviderIDMapping(t *testing.T) {
	reason := "reason-value"
	cases := []struct {
		name           string
		id, status     string
		reason         *string
		wantReasonNull bool
	}{
		{"with reason", "provider-id-value", "status-value", &reason, false},
		{"nil reason stays SQL NULL", "provider-id-value", "delivered", nil, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := &fakeQuerier{}
			if err := NewPgRepo(f).MarkByProviderID(context.Background(), c.id, c.status, c.reason); err != nil {
				t.Fatalf("MarkByProviderID: %v", err)
			}
			if len(f.markParams) != 1 {
				t.Fatalf("want one MarkEmailByProviderID call, got %d", len(f.markParams))
			}
			got := f.markParams[0]
			if got.ProviderID == nil || *got.ProviderID != c.id {
				t.Errorf("ProviderID = %v, want %q", got.ProviderID, c.id)
			}
			if got.Status != c.status {
				t.Errorf("Status = %q, want %q", got.Status, c.status)
			}
			switch {
			case c.wantReasonNull && got.Error != nil:
				t.Errorf("Error = %q, want nil", *got.Error)
			case !c.wantReasonNull && (got.Error == nil || *got.Error != *c.reason):
				t.Errorf("Error = %v, want %q", got.Error, *c.reason)
			}
		})
	}
}

// TestPgRepo_SuppressPassesAddressThrough pins that the adapter does NOT
// re-case or re-trim the address: emailkit already lowercases and trims it,
// and a second copy of that policy here is the drift this migration exists to
// remove. A mixed-case, space-padded input must arrive at the query byte-for-
// byte, so any normalisation sneaking back in fails here.
func TestPgRepo_SuppressPassesAddressThrough(t *testing.T) {
	const addr = "  MiXeD@Example.COM  "
	const reason = "reason-value"

	f := &fakeQuerier{}
	if err := NewPgRepo(f).Suppress(context.Background(), addr, reason); err != nil {
		t.Fatalf("Suppress: %v", err)
	}
	if len(f.suppress) != 1 {
		t.Fatalf("want one SuppressEmail call, got %d", len(f.suppress))
	}
	got := f.suppress[0]
	if got.Email != addr {
		t.Errorf("Email = %q, want it passed through unmodified as %q", got.Email, addr)
	}
	if got.Reason == nil || *got.Reason != reason {
		t.Errorf("Reason = %v, want %q", got.Reason, reason)
	}
}
