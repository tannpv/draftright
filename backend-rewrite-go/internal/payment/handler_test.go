package payment

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	auth "github.com/tannpv/draftright-rewrite/internal/platform/auth"
	"github.com/tannpv/draftright-rewrite/internal/shared"
)

func TestStatusView_NotFoundJSON(t *testing.T) {
	b, _ := json.Marshal(StatusView{notFound: true})
	if string(b) != `{"status":"not_found"}` {
		t.Fatalf("not-found JSON = %s", b)
	}
}

func TestStatusView_FoundJSON_FieldOrder(t *testing.T) {
	name := "Pro"
	ts := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	iso := shared.ISOMillis(ts)
	v := StatusView{
		Status: "completed", Method: "stripe", Amount: 900, Currency: "USD",
		ReferenceCode: "DR-PRO-ABCD1234", PlanName: &name, CompletedAt: &iso, ExpiresAt: nil,
	}
	b, _ := json.Marshal(v)
	want := `{"status":"completed","method":"stripe","amount":900,"currency":"USD","reference_code":"DR-PRO-ABCD1234","plan_name":"Pro","completed_at":"` + iso + `","expires_at":null}`
	if string(b) != want {
		t.Fatalf("found JSON =\n%s\nwant\n%s", b, want)
	}
}

func TestPaymentRow_MarshalJSON_FieldOrder(t *testing.T) {
	ts := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	iso := shared.ISOMillis(ts)
	pr := PaymentRow{
		ID: "id1", UserID: "u1", PlanID: "p1", Amount: 900, Currency: "USD",
		Method: "stripe", Status: "completed", ReferenceCode: "DR-PRO-X",
		CreatedAt: ts, UpdatedAt: ts,
	}
	b, _ := json.Marshal(pr)
	want := `{"id":"id1","user_id":"u1","plan_id":"p1","amount":900,"currency":"USD","method":"stripe","status":"completed","provider_ref":null,"reference_code":"DR-PRO-X","qr_data":null,"notes":null,"expires_at":null,"completed_at":null,"created_at":"` + iso + `","updated_at":"` + iso + `","plan":null}`
	if string(b) != want {
		t.Fatalf("payment row JSON =\n%s\nwant\n%s", b, want)
	}
}

func TestMethodsHandler(t *testing.T) {
	h := NewHandler(NewService(fakeRepo{}, fakeSettings{csv: "stripe", found: true}, "", nil, nil, nil, nil))
	rec := httptest.NewRecorder()
	h.Methods(rec, httptest.NewRequest(http.MethodGet, "/payment/methods", nil))
	if rec.Code != 200 {
		t.Fatalf("status = %d", rec.Code)
	}
	if rec.Body.String() != "{\"methods\":[\"stripe\"]}\n" {
		t.Fatalf("body = %q", rec.Body.String())
	}
}

func statusReq(ref string) *http.Request {
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("ref", ref)
	req := httptest.NewRequest(http.MethodGet, "/payment/status/"+ref, nil)
	return req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
}

func TestStatusHandler_NotFound(t *testing.T) {
	h := NewHandler(NewService(fakeRepo{status: nil}, fakeSettings{}, "", nil, nil, nil, nil))
	rec := httptest.NewRecorder()
	h.Status(rec, statusReq("DR-PRO-NOPE"))
	if rec.Code != 200 {
		t.Fatalf("status = %d", rec.Code)
	}
	if rec.Body.String() != "{\"status\":\"not_found\"}\n" {
		t.Fatalf("body = %q", rec.Body.String())
	}
}

func TestStatusHandler_Found(t *testing.T) {
	name := "Pro"
	h := NewHandler(NewService(fakeRepo{status: &StatusRow{
		Status: "completed", Method: "stripe", Amount: 900, Currency: "USD",
		ReferenceCode: "DR-PRO-ABCD1234", PlanName: &name,
	}}, fakeSettings{}, "", nil, nil, nil, nil))
	rec := httptest.NewRecorder()
	h.Status(rec, statusReq("DR-PRO-ABCD1234"))
	if rec.Code != 200 {
		t.Fatalf("status = %d", rec.Code)
	}
	var got map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got["status"] != "completed" || got["reference_code"] != "DR-PRO-ABCD1234" {
		t.Fatalf("body = %s", rec.Body.String())
	}
}

func TestHistoryHandler_Empty(t *testing.T) {
	h := NewHandler(NewService(fakeRepo{hist: nil}, fakeSettings{}, "", nil, nil, nil, nil))
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/payment/history", nil)
	req = req.WithContext(shared.ContextWithClaims(req.Context(), &auth.Claims{Sub: "u1"}))
	h.History(rec, req)
	if rec.Code != 200 {
		t.Fatalf("status = %d", rec.Code)
	}
	if rec.Body.String() != "[]\n" {
		t.Fatalf("empty history must be [], got %q", rec.Body.String())
	}
}

func TestHistoryHandler_MissingClaims(t *testing.T) {
	h := NewHandler(NewService(fakeRepo{}, fakeSettings{}, "", nil, nil, nil, nil))
	rec := httptest.NewRecorder()
	h.History(rec, httptest.NewRequest(http.MethodGet, "/payment/history", nil))
	if rec.Code != 500 {
		t.Fatalf("missing claims → 500, got %d", rec.Code)
	}
}
