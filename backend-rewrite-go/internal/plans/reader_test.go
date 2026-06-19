package plans_test

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/tannpv/draftright-rewrite/internal/plans"
	sqlc "github.com/tannpv/draftright-rewrite/internal/shared/pg/sqlc"
)

type fakeQ struct {
	row  sqlc.FindFreePlanRow
	err  error
	rows []sqlc.ListActivePlansRow
}

func (f fakeQ) FindFreePlan(context.Context) (sqlc.FindFreePlanRow, error) { return f.row, f.err }

func (f fakeQ) ListActivePlans(context.Context) ([]sqlc.ListActivePlansRow, error) {
	return f.rows, f.err
}

func TestFindFreePlan(t *testing.T) {
	var id pgtype.UUID
	_ = id.Scan("11111111-1111-1111-1111-111111111111")
	r := plans.NewReader(fakeQ{row: sqlc.FindFreePlanRow{ID: id, Name: "Free", DailyLimit: 10}})
	p, err := r.FindFreePlan(context.Background())
	if err != nil || p.ID == "" || p.Name != "Free" || p.DailyLimit != 10 {
		t.Fatalf("got %+v err %v", p, err)
	}
}
