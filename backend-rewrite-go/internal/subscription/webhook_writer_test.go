package subscription

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	sqlc "github.com/tannpv/draftright-rewrite/internal/shared/pg/sqlc"
)

type fakeWebhookQ struct {
	cancelledUser string
	inserted      sqlc.InsertGrantedSubscriptionParams
	extendRows    int64
	findRow       sqlc.FindByStoreRefRow
	findErr       error
}

func (f *fakeWebhookQ) CancelActiveSubsByUser(_ context.Context, id pgtype.UUID) error {
	f.cancelledUser = uuidStr(id)
	return nil
}
func (f *fakeWebhookQ) InsertGrantedSubscription(_ context.Context, a sqlc.InsertGrantedSubscriptionParams) (sqlc.Subscription, error) {
	f.inserted = a
	return sqlc.Subscription{}, nil
}
func (f *fakeWebhookQ) StampStoreRefByReference(context.Context, sqlc.StampStoreRefByReferenceParams) (int64, error) {
	return 1, nil
}
func (f *fakeWebhookQ) ExtendByStoreRef(context.Context, sqlc.ExtendByStoreRefParams) (int64, error) {
	return f.extendRows, nil
}
func (f *fakeWebhookQ) CancelByStoreRef(context.Context, sqlc.CancelByStoreRefParams) (int64, error) {
	return 1, nil
}
func (f *fakeWebhookQ) ExpireByStoreRef(context.Context, sqlc.ExpireByStoreRefParams) (int64, error) {
	return 1, nil
}
func (f *fakeWebhookQ) FindByStoreRef(context.Context, sqlc.FindByStoreRefParams) (sqlc.FindByStoreRefRow, error) {
	return f.findRow, f.findErr
}

func uuidStr(id pgtype.UUID) string {
	v, _ := id.Value()
	s, _ := v.(string)
	return s
}

func mustUUID(s string) pgtype.UUID {
	var u pgtype.UUID
	if err := u.Scan(s); err != nil {
		panic(err)
	}
	return u
}

func TestWebhookWriter_GrantCancelsThenInserts(t *testing.T) {
	q := &fakeWebhookQ{}
	w := NewWebhookWriter(q)
	exp := time.Unix(1798761600, 0).UTC()
	if err := w.Grant(context.Background(), "11111111-1111-1111-1111-111111111111",
		"22222222-2222-2222-2222-222222222222", "stripe", &exp); err != nil {
		t.Fatal(err)
	}
	if q.cancelledUser == "" {
		t.Fatal("expected prior active subs cancelled before insert")
	}
	if !q.inserted.ExpiresAt.Valid || !q.inserted.ExpiresAt.Time.Equal(exp) {
		t.Fatalf("insert expires_at wrong: %+v", q.inserted.ExpiresAt)
	}
}

func TestWebhookWriter_FindByStoreRef(t *testing.T) {
	q := &fakeWebhookQ{findRow: sqlc.FindByStoreRefRow{
		UserID:    mustUUID("33333333-3333-3333-3333-333333333333"),
		UserEmail: "a@b.com", UserName: "Ann", PlanName: "Pro",
	}}
	w := NewWebhookWriter(q)
	sub, err := w.FindByStoreRef(context.Background(), "lemonsqueezy", "99")
	if err != nil {
		t.Fatal(err)
	}
	if sub == nil || sub.UserEmail != "a@b.com" || sub.PlanName != "Pro" {
		t.Fatalf("bad sub: %+v", sub)
	}
}

func TestWebhookWriter_FindByStoreRef_NoneIsNil(t *testing.T) {
	w := NewWebhookWriter(&fakeWebhookQ{findErr: pgx.ErrNoRows})
	sub, err := w.FindByStoreRef(context.Background(), "stripe", "x")
	if err != nil || sub != nil {
		t.Fatalf("want nil,nil; got %+v,%v", sub, err)
	}
}
