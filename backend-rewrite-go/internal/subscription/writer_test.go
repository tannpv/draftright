package subscription_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	sqlc "github.com/tannpv/draftright-rewrite/internal/shared/pg/sqlc"
	"github.com/tannpv/draftright-rewrite/internal/subscription"
)

type fakeWQ struct {
	called        bool
	arg           sqlc.CreateFreeSubscriptionParams
	cancelledUser pgtype.UUID
	granted       sqlc.InsertGrantedSubscriptionParams
	grantRow      sqlc.Subscription
}

func (f *fakeWQ) CreateFreeSubscription(_ context.Context, a sqlc.CreateFreeSubscriptionParams) error {
	f.called = true
	f.arg = a
	return nil
}

func (f *fakeWQ) CancelActiveSubsByUser(_ context.Context, id pgtype.UUID) error {
	f.cancelledUser = id
	return nil
}

func (f *fakeWQ) InsertGrantedSubscription(_ context.Context, a sqlc.InsertGrantedSubscriptionParams) (sqlc.Subscription, error) {
	f.granted = a
	return f.grantRow, nil
}

func TestCreateFree(t *testing.T) {
	q := &fakeWQ{}
	w := subscription.NewWriter(q)
	if err := w.CreateFree(context.Background(),
		"11111111-1111-1111-1111-111111111111",
		"22222222-2222-2222-2222-222222222222"); err != nil {
		t.Fatal(err)
	}
	if !q.called {
		t.Fatal("query not called")
	}
	var want pgtype.UUID
	_ = want.Scan("11111111-1111-1111-1111-111111111111")
	if q.arg.UserID != want {
		t.Fatalf("user id not bound: %v", q.arg.UserID)
	}
}

func mustUUID(s string) pgtype.UUID {
	var u pgtype.UUID
	if err := u.Scan(s); err != nil {
		panic(err)
	}
	return u
}

func TestGrant_CancelsThenInsertsAdminGranted(t *testing.T) {
	started := time.Date(2026, 6, 19, 12, 0, 0, 0, time.UTC)
	created := time.Date(2026, 6, 19, 12, 0, 1, 0, time.UTC)
	updated := time.Date(2026, 6, 19, 12, 0, 2, 0, time.UTC)
	txn := "txn-1"
	q := &fakeWQ{grantRow: sqlc.Subscription{
		ID:                 mustUUID("33333333-3333-3333-3333-333333333333"),
		UserID:             mustUUID("11111111-1111-1111-1111-111111111111"),
		PlanID:             mustUUID("22222222-2222-2222-2222-222222222222"),
		Status:             sqlc.SubscriptionsStatusEnumActive,
		StoreType:          sqlc.SubscriptionsStoreTypeEnumAdminGranted,
		StoreTransactionID: &txn,
		StartedAt:          pgtype.Timestamp{Time: started, Valid: true},
		ExpiresAt:          pgtype.Timestamp{},
		CreatedAt:          pgtype.Timestamp{Time: created, Valid: true},
		UpdatedAt:          pgtype.Timestamp{Time: updated, Valid: true},
	}}
	w := subscription.NewWriter(q)

	got, err := w.Grant(context.Background(),
		"11111111-1111-1111-1111-111111111111",
		"22222222-2222-2222-2222-222222222222", nil)
	if err != nil {
		t.Fatal(err)
	}
	if q.cancelledUser != mustUUID("11111111-1111-1111-1111-111111111111") {
		t.Fatalf("expected active subs cancelled for user, got %v", q.cancelledUser)
	}
	if q.granted.StoreType != sqlc.SubscriptionsStoreTypeEnumAdminGranted {
		t.Fatalf("store_type not admin_granted: %v", q.granted.StoreType)
	}
	if q.granted.ExpiresAt.Valid {
		t.Fatalf("expires_at should be NULL when absent: %+v", q.granted.ExpiresAt)
	}
	if got.ID != "33333333-3333-3333-3333-333333333333" ||
		got.UserID != "11111111-1111-1111-1111-111111111111" ||
		got.PlanID != "22222222-2222-2222-2222-222222222222" ||
		got.Status != "active" || got.StoreType != "admin_granted" ||
		got.StoreTransactionID == nil || *got.StoreTransactionID != "txn-1" ||
		!got.StartedAt.Equal(started) || got.ExpiresAt != nil ||
		!got.CreatedAt.Equal(created) || !got.UpdatedAt.Equal(updated) {
		t.Fatalf("returned row mapped wrong: %+v", got)
	}
}

func TestGrant_ExpiresAtStoredWhenProvided(t *testing.T) {
	exp := time.Date(2026, 7, 19, 0, 0, 0, 0, time.UTC)
	q := &fakeWQ{grantRow: sqlc.Subscription{
		ID:        mustUUID("33333333-3333-3333-3333-333333333333"),
		UserID:    mustUUID("11111111-1111-1111-1111-111111111111"),
		PlanID:    mustUUID("22222222-2222-2222-2222-222222222222"),
		Status:    sqlc.SubscriptionsStatusEnumActive,
		StoreType: sqlc.SubscriptionsStoreTypeEnumAdminGranted,
		ExpiresAt: pgtype.Timestamp{Time: exp, Valid: true},
	}}
	w := subscription.NewWriter(q)

	got, err := w.Grant(context.Background(),
		"11111111-1111-1111-1111-111111111111",
		"22222222-2222-2222-2222-222222222222", &exp)
	if err != nil {
		t.Fatal(err)
	}
	if !q.granted.ExpiresAt.Valid || !q.granted.ExpiresAt.Time.Equal(exp) {
		t.Fatalf("expires_at not bound: %+v", q.granted.ExpiresAt)
	}
	if got.ExpiresAt == nil || !got.ExpiresAt.Equal(exp) {
		t.Fatalf("returned expires_at wrong: %+v", got.ExpiresAt)
	}
}

// TestGrantedSub_JSONMatchesNode pins the serialized bytes to TypeORM's
// Date.toJSON() format: fixed 3-digit millis with trailing "Z" (NOT Go's
// default microseconds), exact Node entity key order, and expires_at null
// when absent. Sub-second / whole-second timestamps both render ".###Z".
func TestGrantedSub_JSONMatchesNode(t *testing.T) {
	// started_at = whole second (must gain ".000"); created/updated carry
	// sub-second precision (must truncate to 3 digits, NOT 6).
	started := time.Date(2026, 6, 19, 12, 0, 0, 0, time.UTC)
	created := time.Date(2026, 6, 19, 12, 0, 0, 123456000, time.UTC)
	updated := time.Date(2026, 6, 19, 12, 0, 1, 0, time.UTC)
	txn := "txn-1"
	g := subscription.GrantedSub{
		ID:                 "33333333-3333-3333-3333-333333333333",
		UserID:             "11111111-1111-1111-1111-111111111111",
		PlanID:             "22222222-2222-2222-2222-222222222222",
		Status:             "active",
		StoreType:          "admin_granted",
		StoreTransactionID: &txn,
		StartedAt:          started,
		ExpiresAt:          nil,
		CreatedAt:          created,
		UpdatedAt:          updated,
	}
	b, err := json.Marshal(g)
	if err != nil {
		t.Fatal(err)
	}
	want := `{"id":"33333333-3333-3333-3333-333333333333",` +
		`"user_id":"11111111-1111-1111-1111-111111111111",` +
		`"plan_id":"22222222-2222-2222-2222-222222222222",` +
		`"status":"active","store_type":"admin_granted",` +
		`"store_transaction_id":"txn-1",` +
		`"started_at":"2026-06-19T12:00:00.000Z",` +
		`"expires_at":null,` +
		`"created_at":"2026-06-19T12:00:00.123Z",` +
		`"updated_at":"2026-06-19T12:00:01.000Z"}`
	if string(b) != want {
		t.Fatalf("JSON mismatch\n got: %s\nwant: %s", b, want)
	}

	// expires_at present → ISOMillis string, not null.
	exp := time.Date(2026, 7, 19, 0, 0, 0, 0, time.UTC)
	g.ExpiresAt = &exp
	b, err = json.Marshal(g)
	if err != nil {
		t.Fatal(err)
	}
	if !contains(string(b), `"expires_at":"2026-07-19T00:00:00.000Z"`) {
		t.Fatalf("expires_at not rendered as ISOMillis: %s", b)
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
