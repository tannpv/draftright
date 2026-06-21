package adminauth

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

type fakeAuditRepo struct {
	rows      []AdminUserAuditOut
	total     int
	gotLimit  int
	gotOffset int
}

func (f *fakeAuditRepo) ListAudit(_ context.Context, limit, offset int) ([]AdminUserAuditOut, error) {
	f.gotLimit, f.gotOffset = limit, offset
	return f.rows, nil
}
func (f *fakeAuditRepo) CountAudit(context.Context) (int, error) { return f.total, nil }

func TestAdminAuditHandler_List_ReturnsRowsTotal(t *testing.T) {
	repo := &fakeAuditRepo{
		rows: []AdminUserAuditOut{{
			ID: "a", ActorAdminID: "b", ActorEmail: "x@y.z",
			TargetAdminID: "c", TargetEmail: "p@q.r",
			CreatedAt: time.Date(2026, 6, 21, 12, 0, 0, 0, time.UTC),
		}},
		total: 1,
	}
	h := NewAdminAuditHandler(NewAdminAuditService(repo))

	req := httptest.NewRequest(http.MethodGet, "/admin/admin-user-audit?limit=10&offset=5", nil)
	rec := httptest.NewRecorder()
	h.List(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d want 200", rec.Code)
	}
	var body struct {
		Rows  []map[string]any `json:"rows"`
		Total int              `json:"total"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v (%s)", err, rec.Body)
	}
	if body.Total != 1 || len(body.Rows) != 1 {
		t.Fatalf("got total=%d rows=%d", body.Total, len(body.Rows))
	}
	if repo.gotLimit != 10 || repo.gotOffset != 5 {
		t.Errorf("limit/offset: got %d/%d want 10/5", repo.gotLimit, repo.gotOffset)
	}
}

func TestAdminAuditHandler_List_DefaultsAndCap(t *testing.T) {
	repo := &fakeAuditRepo{}
	h := NewAdminAuditHandler(NewAdminAuditService(repo))

	h.List(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/admin/admin-user-audit", nil))
	if repo.gotLimit != 50 || repo.gotOffset != 0 {
		t.Errorf("defaults: got %d/%d want 50/0", repo.gotLimit, repo.gotOffset)
	}
	h.List(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/admin/admin-user-audit?limit=9999", nil))
	if repo.gotLimit != 100 {
		t.Errorf("cap: got %d want 100", repo.gotLimit)
	}
}
