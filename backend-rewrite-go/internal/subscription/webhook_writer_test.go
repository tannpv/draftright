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
	cancelledUser  string
	inserted       sqlc.InsertGrantedSubscriptionParams
	extendRows     int64
	findRow        sqlc.FindByStoreRefRow
	findErr        error
	stampByUserArg sqlc.StampStoreRefByUserParams
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
func (f *fakeWebhookQ) StampStoreRefByUser(_ context.Context, a sqlc.StampStoreRefByUserParams) (int64, error) {
	f.stampByUserArg = a
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

// TestWebhookWriter_StampStoreRefByUser pins the IAP redeem path's writer:
// matched by user_id + store_type, NO payments join (review C1 — see
// apple_redeem.go / queries_auth.sql StampStoreRefByUser).
func TestWebhookWriter_StampStoreRefByUser(t *testing.T) {
	q := &fakeWebhookQ{}
	w := NewWebhookWriter(q)
	userID := "44444444-4444-4444-4444-444444444444"
	if err := w.StampStoreRefByUser(context.Background(), userID, "apple_iap", "o1"); err != nil {
		t.Fatal(err)
	}
	if uuidStr(q.stampByUserArg.UserID) != userID {
		t.Fatalf("user_id = %q, want %q", uuidStr(q.stampByUserArg.UserID), userID)
	}
	if string(q.stampByUserArg.StoreType) != "apple_iap" {
		t.Fatalf("store_type = %q, want apple_iap", q.stampByUserArg.StoreType)
	}
	if q.stampByUserArg.StoreTransactionID == nil || *q.stampByUserArg.StoreTransactionID != "o1" {
		t.Fatalf("store_transaction_id = %v, want o1", q.stampByUserArg.StoreTransactionID)
	}
}
