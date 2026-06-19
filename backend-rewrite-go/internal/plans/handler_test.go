package plans_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/tannpv/draftright-rewrite/internal/plans"
)

type fakeLister struct{ out []plans.PlanEntity }

func (f fakeLister) ListActive(ctx context.Context) ([]plans.PlanEntity, error) { return f.out, nil }

func TestHandler_List_200Array(t *testing.T) {
	h := plans.NewHandler(plans.NewService(fakeLister{out: []plans.PlanEntity{}}))
	rr := httptest.NewRecorder()
	h.List(rr, httptest.NewRequest(http.MethodGet, "/plans", nil))
	if rr.Code != http.StatusOK {
		t.Fatalf("status %d", rr.Code)
	}
	var arr []map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &arr); err != nil {
		t.Fatalf("not an array: %s", rr.Body)
	}
}
