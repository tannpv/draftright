package payment

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	sqlc "github.com/tannpv/draftright-rewrite/internal/shared/pg/sqlc"
)

type fakeQ struct {
	byRef     sqlc.GetPaymentByReferenceRow
	byRefErr  error
	list      []sqlc.ListPaymentsByUserRow
	listErr   error
	plan      sqlc.GetPlanForCheckoutRow
	planErr   error
	user      sqlc.GetUserForCheckoutRow
	userErr   error
	created   sqlc.CreatePaymentRow
	createErr error
	qrErr     error
	failErr   error
}

func (f fakeQ) GetPaymentByReference(ctx context.Context, ref string) (sqlc.GetPaymentByReferenceRow, error) {
	return f.byRef, f.byRefErr
}
func (f fakeQ) ListPaymentsByUser(ctx context.Context, userID pgtype.UUID) ([]sqlc.ListPaymentsByUserRow, error) {
	return f.list, f.listErr
}
func (f fakeQ) GetPlanForCheckout(ctx context.Context, id pgtype.UUID) (sqlc.GetPlanForCheckoutRow, error) {
	return f.plan, f.planErr
}
func (f fakeQ) GetUserForCheckout(ctx context.Context, id pgtype.UUID) (sqlc.GetUserForCheckoutRow, error) {
	return f.user, f.userErr
}
func (f fakeQ) CreatePayment(ctx context.Context, arg sqlc.CreatePaymentParams) (sqlc.CreatePaymentRow, error) {
	return f.created, f.createErr
}
func (f fakeQ) UpdatePaymentQRData(ctx context.Context, arg sqlc.UpdatePaymentQRDataParams) error {
	return f.qrErr
}
func (f fakeQ) MarkPaymentFailed(ctx context.Context, arg sqlc.MarkPaymentFailedParams) error {
	return f.failErr
}

func uuidV(s string) pgtype.UUID { var u pgtype.UUID; _ = u.Scan(s); return u }

func TestRepo_GetByReference_NotFound(t *testing.T) {
	r := NewRepo(fakeQ{byRefErr: pgx.ErrNoRows})
	got, err := r.GetByReference(context.Background(), "DR-PRO-NOPE")
	if err != nil {
		t.Fatalf("ErrNoRows must map to (nil,nil), got err %v", err)
	}
	if got != nil {
		t.Fatalf("not-found must be nil, got %+v", got)
	}
}

func TestRepo_GetByReference_Maps(t *testing.T) {
	name := "Pro Monthly"
	r := NewRepo(fakeQ{byRef: sqlc.GetPaymentByReferenceRow{
		ID: uuidV("11111111-1111-1111-1111-111111111111"), Amount: 900, Currency: "USD",
		Method: "stripe", Status: "completed", ReferenceCode: "DR-PRO-ABCD1234",
		CompletedAt: pgtype.Timestamp{Time: time.Unix(1700000000, 0).UTC(), Valid: true},
		PlanName:    &name,
	}})
	got, err := r.GetByReference(context.Background(), "DR-PRO-ABCD1234")
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != "completed" || got.Amount != 900 || got.Currency != "USD" || got.Method != "stripe" {
		t.Fatalf("scalar mapping wrong: %+v", got)
	}
	if got.PlanName == nil || *got.PlanName != name {
		t.Fatalf("plan_name should be %q, got %v", name, got.PlanName)
	}
	if got.CompletedAt == nil {
		t.Fatal("completed_at should be set")
	}
}

func TestRepo_ListByUser_MapsPlan(t *testing.T) {
	name := "Pro Monthly"
	dl := int32(100)
	active := true
	bp := sqlc.PlansBillingPeriodEnum("monthly")
	r := NewRepo(fakeQ{list: []sqlc.ListPaymentsByUserRow{{
		ID:     uuidV("22222222-2222-2222-2222-222222222222"),
		UserID: uuidV("33333333-3333-3333-3333-333333333333"),
		PlanID: uuidV("44444444-4444-4444-4444-444444444444"),
		Amount: 900, Currency: "USD", Method: "stripe", Status: "completed",
		ReferenceCode: "DR-PRO-ABCD1234",
		CreatedAt:     pgtype.Timestamp{Time: time.Unix(1700000000, 0).UTC(), Valid: true},
		UpdatedAt:     pgtype.Timestamp{Time: time.Unix(1700000000, 0).UTC(), Valid: true},
		PlanPk:        uuidV("44444444-4444-4444-4444-444444444444"),
		PlanName:      &name, PlanDailyLimit: &dl, PlanIsActive: &active, PlanBillingPeriod: &bp,
	}}})
	got, err := r.ListByUser(context.Background(), "33333333-3333-3333-3333-333333333333")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("want 1 row, got %d", len(got))
	}
	row := got[0]
	if row.ReferenceCode != "DR-PRO-ABCD1234" || row.Amount != 900 {
		t.Fatalf("scalar map wrong: %+v", row)
	}
	if row.Plan == nil {
		t.Fatal("plan should be present")
	}
	if row.Plan.Name != name || row.Plan.DailyLimit != 100 || !row.Plan.IsActive || row.Plan.BillingPeriod != "monthly" {
		t.Fatalf("plan map wrong: %+v", row.Plan)
	}
}

func TestRepo_ListByUser_MalformedUUID(t *testing.T) {
	r := NewRepo(fakeQ{})
	got, err := r.ListByUser(context.Background(), "not-a-uuid")
	if err != nil {
		t.Fatalf("malformed id → empty, no error; got %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("want empty slice, got %d", len(got))
	}
}
