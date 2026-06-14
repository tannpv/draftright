package exttoken_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/tannpv/draftright-rewrite/internal/exttoken"
	sqlc "github.com/tannpv/draftright-rewrite/internal/shared/pg/sqlc"
)

const (
	uidA = "11111111-1111-1111-1111-111111111111"
	uidB = "22222222-2222-2222-2222-222222222222"
	devA = "33333333-3333-3333-3333-333333333333"
	tokA = "44444444-4444-4444-4444-444444444444"
)

func mustPgUUID(t *testing.T, s string) pgtype.UUID {
	t.Helper()
	var u pgtype.UUID
	if err := u.Scan(s); err != nil {
		t.Fatalf("scan %s: %v", s, err)
	}
	return u
}

// fakeQ records the params it was called with and returns canned rows.
type fakeQ struct {
	revokeDevParams sqlc.RevokeActiveTokensForDeviceParams
	revokeDevErr    error

	insertParams sqlc.InsertExtensionTokenParams
	insertRow    sqlc.InsertExtensionTokenRow
	insertErr    error

	listParam pgtype.UUID
	listRows  []sqlc.ListActiveTokensRow
	listErr   error

	revokeIDParams sqlc.RevokeTokenByIDParams
	revokeIDErr    error

	findHash string
	findRow  sqlc.FindActiveTokenByHashRow
	findErr  error

	touchID  pgtype.UUID
	touchErr error
}

func (f *fakeQ) RevokeActiveTokensForDevice(ctx context.Context, arg sqlc.RevokeActiveTokensForDeviceParams) error {
	f.revokeDevParams = arg
	return f.revokeDevErr
}

func (f *fakeQ) InsertExtensionToken(ctx context.Context, arg sqlc.InsertExtensionTokenParams) (sqlc.InsertExtensionTokenRow, error) {
	f.insertParams = arg
	return f.insertRow, f.insertErr
}

func (f *fakeQ) ListActiveTokens(ctx context.Context, userID pgtype.UUID) ([]sqlc.ListActiveTokensRow, error) {
	f.listParam = userID
	return f.listRows, f.listErr
}

func (f *fakeQ) RevokeTokenByID(ctx context.Context, arg sqlc.RevokeTokenByIDParams) error {
	f.revokeIDParams = arg
	return f.revokeIDErr
}

func (f *fakeQ) FindActiveTokenByHash(ctx context.Context, tokenHash string) (sqlc.FindActiveTokenByHashRow, error) {
	f.findHash = tokenHash
	return f.findRow, f.findErr
}

func (f *fakeQ) TouchTokenLastUsed(ctx context.Context, id pgtype.UUID) error {
	f.touchID = id
	return f.touchErr
}

func TestRevokeActiveForDevice_PassesParams(t *testing.T) {
	f := &fakeQ{}
	r := exttoken.NewRepo(f)
	if err := r.RevokeActiveForDevice(context.Background(), uidA, devA); err != nil {
		t.Fatal(err)
	}
	if uuid.UUID(f.revokeDevParams.UserID.Bytes).String() != uidA ||
		uuid.UUID(f.revokeDevParams.DeviceID.Bytes).String() != devA {
		t.Fatalf("bad params: %+v", f.revokeDevParams)
	}
}

func TestInsert_PassesParamsAndMapsRow(t *testing.T) {
	created := time.Date(2026, 6, 14, 10, 0, 0, 0, time.UTC)
	f := &fakeQ{insertRow: sqlc.InsertExtensionTokenRow{
		ID:         mustPgUUID(t, tokA),
		Scopes:     []string{"rewrite"},
		DeviceID:   mustPgUUID(t, devA),
		DeviceName: "mobile",
		CreatedAt:  pgtype.Timestamp{Time: created, Valid: true},
	}}
	r := exttoken.NewRepo(f)
	row, err := r.Insert(context.Background(), uidA, "hashhash", devA, "mobile", []string{"rewrite"})
	if err != nil {
		t.Fatal(err)
	}
	// params
	if uuid.UUID(f.insertParams.UserID.Bytes).String() != uidA ||
		f.insertParams.TokenHash != "hashhash" ||
		uuid.UUID(f.insertParams.DeviceID.Bytes).String() != devA ||
		f.insertParams.DeviceName != "mobile" ||
		len(f.insertParams.Scopes) != 1 || f.insertParams.Scopes[0] != "rewrite" {
		t.Fatalf("bad insert params: %+v", f.insertParams)
	}
	// mapped row
	if row.ID != tokA || row.DeviceID != devA || row.DeviceName != "mobile" ||
		len(row.Scopes) != 1 || row.Scopes[0] != "rewrite" {
		t.Fatalf("bad row: %+v", row)
	}
	if !row.CreatedAt.Equal(created) {
		t.Fatalf("bad created_at: %v", row.CreatedAt)
	}
	if row.LastUsedAt != nil || row.RevokedAt != nil {
		t.Fatalf("expected nil last_used/revoked, got %+v", row)
	}
}

func TestListActive_MapsRows(t *testing.T) {
	created := time.Date(2026, 6, 14, 10, 0, 0, 0, time.UTC)
	last := time.Date(2026, 6, 14, 11, 0, 0, 0, time.UTC)
	f := &fakeQ{listRows: []sqlc.ListActiveTokensRow{{
		ID:         mustPgUUID(t, tokA),
		Scopes:     []string{"rewrite"},
		DeviceID:   mustPgUUID(t, devA),
		DeviceName: "keyboard",
		LastUsedAt: pgtype.Timestamp{Time: last, Valid: true},
		CreatedAt:  pgtype.Timestamp{Time: created, Valid: true},
	}}}
	r := exttoken.NewRepo(f)
	rows, err := r.ListActive(context.Background(), uidA)
	if err != nil {
		t.Fatal(err)
	}
	if uuid.UUID(f.listParam.Bytes).String() != uidA {
		t.Fatalf("bad list param: %v", f.listParam)
	}
	if len(rows) != 1 {
		t.Fatalf("want 1 row, got %d", len(rows))
	}
	got := rows[0]
	if got.ID != tokA || got.DeviceID != devA || got.DeviceName != "keyboard" {
		t.Fatalf("bad row: %+v", got)
	}
	if got.LastUsedAt == nil || !got.LastUsedAt.Equal(last) {
		t.Fatalf("bad last_used: %v", got.LastUsedAt)
	}
	if !got.CreatedAt.Equal(created) || got.RevokedAt != nil {
		t.Fatalf("bad ts: %+v", got)
	}
}

func TestListActive_EmptyNonNil(t *testing.T) {
	r := exttoken.NewRepo(&fakeQ{listRows: nil})
	rows, err := r.ListActive(context.Background(), uidA)
	if err != nil {
		t.Fatal(err)
	}
	if rows == nil {
		t.Fatal("want non-nil empty slice")
	}
	if len(rows) != 0 {
		t.Fatalf("want 0 rows, got %d", len(rows))
	}
}

func TestRevokeByID_PassesParams(t *testing.T) {
	f := &fakeQ{}
	r := exttoken.NewRepo(f)
	if err := r.RevokeByID(context.Background(), tokA, uidA); err != nil {
		t.Fatal(err)
	}
	if uuid.UUID(f.revokeIDParams.ID.Bytes).String() != tokA ||
		uuid.UUID(f.revokeIDParams.UserID.Bytes).String() != uidA {
		t.Fatalf("bad params: %+v", f.revokeIDParams)
	}
}

func TestFindActiveByHash_Found(t *testing.T) {
	f := &fakeQ{findRow: sqlc.FindActiveTokenByHashRow{
		ID:     mustPgUUID(t, tokA),
		UserID: mustPgUUID(t, uidB),
		Scopes: []string{"rewrite"},
	}}
	r := exttoken.NewRepo(f)
	at, err := r.FindActiveByHash(context.Background(), "somehash")
	if err != nil {
		t.Fatal(err)
	}
	if f.findHash != "somehash" {
		t.Fatalf("bad hash param: %q", f.findHash)
	}
	if at == nil {
		t.Fatal("want non-nil active token")
	}
	if at.ID != tokA || at.UserID != uidB || len(at.Scopes) != 1 || at.Scopes[0] != "rewrite" {
		t.Fatalf("bad active token: %+v", at)
	}
}

func TestFindActiveByHash_NoRow_Nil(t *testing.T) {
	r := exttoken.NewRepo(&fakeQ{findErr: pgx.ErrNoRows})
	at, err := r.FindActiveByHash(context.Background(), "somehash")
	if err != nil || at != nil {
		t.Fatalf("want nil,nil got %+v,%v", at, err)
	}
}

func TestFindActiveByHash_OtherErr_Propagates(t *testing.T) {
	r := exttoken.NewRepo(&fakeQ{findErr: context.DeadlineExceeded})
	at, err := r.FindActiveByHash(context.Background(), "somehash")
	if err == nil || at != nil {
		t.Fatalf("want err+nil, got %+v,%v", at, err)
	}
}

func TestTouchLastUsed_PassesID(t *testing.T) {
	f := &fakeQ{}
	r := exttoken.NewRepo(f)
	if err := r.TouchLastUsed(context.Background(), tokA); err != nil {
		t.Fatal(err)
	}
	if uuid.UUID(f.touchID.Bytes).String() != tokA {
		t.Fatalf("bad id: %v", f.touchID)
	}
}

// *Repo must satisfy the consumer-side Querier contract.
var _ exttoken.Querier = (*fakeQ)(nil)
