package parity

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	platformauth "github.com/tannpv/draftright-rewrite/internal/platform/auth"
	"github.com/tannpv/draftright-rewrite/internal/shared"
)

// stubRewriter satisfies the handler's consumer-side port.
type stubRewriter struct {
	out any
	err error
	got struct {
		userID, text, tone, target, source, inputKind string
	}
	trialOut struct {
		text, tone, clientIp, target, source, inputKind string
	}
}

func (s *stubRewriter) Rewrite(_ context.Context, userID, text, tone, target, source, inputKind string) (any, error) {
	s.got.userID, s.got.text, s.got.tone, s.got.target, s.got.source, s.got.inputKind = userID, text, tone, target, source, inputKind
	return s.out, s.err
}

// trialGot records what the Trial path passed (clientIp instead of userID).
func (s *stubRewriter) TrialRewrite(_ context.Context, text, tone, clientIp, target, source, inputKind string) (any, error) {
	s.trialOut.text, s.trialOut.tone, s.trialOut.clientIp, s.trialOut.target, s.trialOut.source, s.trialOut.inputKind = text, tone, clientIp, target, source, inputKind
	return s.out, s.err
}

func withClaims(req *http.Request, userID string) *http.Request {
	return req.WithContext(shared.ContextWithClaims(req.Context(), &platformauth.Claims{Sub: userID}))
}

func newReq(body string) *http.Request {
	return withClaims(
		httptest.NewRequest(http.MethodPost, "/rewrite", strings.NewReader(body)),
		"user-uuid",
	)
}

func TestHandlerRewrite_Valid201(t *testing.T) {
	st := &stubRewriter{out: rewriteEnvelope{RewrittenText: "Hello.", UsageToday: 1, DailyLimit: 500}}
	h := &Handler{svc: st}
	rec := httptest.NewRecorder()
	h.Rewrite(rec, newReq(`{"text":"hi","tone":"polished"}`))

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201", rec.Code)
	}
	if st.got.userID != "user-uuid" || st.got.text != "hi" || st.got.tone != "polished" {
		t.Fatalf("service got %+v", st.got)
	}
	want := `{"rewritten_text":"Hello.","usage_today":1,"daily_limit":500}`
	if strings.TrimSpace(rec.Body.String()) != want {
		t.Fatalf("body = %s, want %s", rec.Body.String(), want)
	}
}

func TestHandlerRewrite_BadTone400(t *testing.T) {
	st := &stubRewriter{}
	h := &Handler{svc: st}
	rec := httptest.NewRecorder()
	h.Rewrite(rec, newReq(`{"text":"hi","tone":"nope"}`))

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
	var body map[string]any
	json.Unmarshal(rec.Body.Bytes(), &body)
	if body["code"] != "invalid-input" {
		t.Fatalf("code = %v, want invalid-input", body["code"])
	}
	want := "tone must be one of the following values: simple, natural, polished, concise, technical, claude, grammar_check, translate"
	if body["error"] != want {
		t.Fatalf("error = %q, want %q", body["error"], want)
	}
	if st.got.text != "" {
		t.Error("invalid input must not reach the service")
	}
}

func TestHandlerRewrite_Quota429(t *testing.T) {
	st := &stubRewriter{err: ErrQuotaExceeded}
	h := &Handler{svc: st}
	rec := httptest.NewRecorder()
	h.Rewrite(rec, newReq(`{"text":"hi","tone":"polished"}`))

	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("status = %d, want 429", rec.Code)
	}
	var body map[string]any
	json.Unmarshal(rec.Body.Bytes(), &body)
	if body["error"] != "Daily limit reached" {
		t.Fatalf("error = %q, want Daily limit reached", body["error"])
	}
	if body["code"] != "rate-limited" {
		t.Fatalf("code = %v, want rate-limited", body["code"])
	}
	if _, ok := body["request_id"]; !ok {
		t.Error("request_id must be present in the envelope")
	}
	// The dropped extra fields must NOT appear (AllExceptionsFilter strips them).
	if _, ok := body["usage_today"]; ok {
		t.Error("usage_today must not appear in the quota error body")
	}
	if _, ok := body["daily_limit"]; ok {
		t.Error("daily_limit must not appear in the quota error body")
	}
}

func TestHandlerRewrite_ProviderFailed502(t *testing.T) {
	st := &stubRewriter{err: ErrProviderFailed}
	h := &Handler{svc: st}
	rec := httptest.NewRecorder()
	h.Rewrite(rec, newReq(`{"text":"hi","tone":"polished"}`))

	if rec.Code != http.StatusBadGateway {
		t.Fatalf("status = %d, want 502", rec.Code)
	}
	var body map[string]any
	json.Unmarshal(rec.Body.Bytes(), &body)
	if body["error"] != providerUnavailableMsg {
		t.Fatalf("error = %q, want %q", body["error"], providerUnavailableMsg)
	}
	if body["code"] != "provider-failed" {
		t.Fatalf("code = %v, want provider-failed", body["code"])
	}
}

func TestHandlerRewrite_UnknownTone400(t *testing.T) {
	st := &stubRewriter{err: &UnknownToneError{Tone: "weird"}}
	h := &Handler{svc: st}
	rec := httptest.NewRecorder()
	// Valid DTO tone, but the service rejects (faithful to callAI's null path).
	h.Rewrite(rec, newReq(`{"text":"hi","tone":"polished"}`))

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
	var body map[string]any
	json.Unmarshal(rec.Body.Bytes(), &body)
	if body["code"] != "invalid-input" {
		t.Fatalf("code = %v, want invalid-input", body["code"])
	}
	if body["error"] != "Unknown tone: weird" {
		t.Fatalf("error = %q, want Unknown tone: weird", body["error"])
	}
}

func TestHandlerRewrite_MissingClaims500(t *testing.T) {
	st := &stubRewriter{}
	h := &Handler{svc: st}
	rec := httptest.NewRecorder()
	// No claims injected.
	req := httptest.NewRequest(http.MethodPost, "/rewrite", strings.NewReader(`{"text":"hi","tone":"polished"}`))
	h.Rewrite(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
}

func TestHandlerRewrite_DefaultErrorOpaque500(t *testing.T) {
	st := &stubRewriter{err: errors.New("boom internal detail")}
	h := &Handler{svc: st}
	rec := httptest.NewRecorder()
	h.Rewrite(rec, newReq(`{"text":"hi","tone":"polished"}`))

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
	var body map[string]any
	json.Unmarshal(rec.Body.Bytes(), &body)
	if strings.Contains(body["error"].(string), "boom") {
		t.Fatalf("500 leaked underlying error: %v", body["error"])
	}
}

// --- trial endpoint (public, unauthenticated) ---

func newTrialReq(body string) *http.Request {
	return httptest.NewRequest(http.MethodPost, "/rewrite/trial", strings.NewReader(body))
}

func TestHandlerTrial_Valid201(t *testing.T) {
	st := &stubRewriter{out: trialRewriteEnvelope{RewrittenText: "Hello."}}
	h := &Handler{svc: st}
	rec := httptest.NewRecorder()
	h.Trial(rec, newTrialReq(`{"text":"hi","tone":"polished"}`))

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201", rec.Code)
	}
	if st.trialOut.text != "hi" || st.trialOut.tone != "polished" {
		t.Fatalf("service got %+v", st.trialOut)
	}
	want := `{"rewritten_text":"Hello."}`
	if strings.TrimSpace(rec.Body.String()) != want {
		t.Fatalf("body = %s, want %s", rec.Body.String(), want)
	}
}

func TestHandlerTrial_Limit429(t *testing.T) {
	st := &stubRewriter{err: ErrTrialLimit}
	h := &Handler{svc: st}
	rec := httptest.NewRecorder()
	h.Trial(rec, newTrialReq(`{"text":"hi","tone":"polished"}`))

	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("status = %d, want 429", rec.Code)
	}
	var body map[string]any
	json.Unmarshal(rec.Body.Bytes(), &body)
	if body["error"] != "Trial limit reached. Sign up for unlimited rewrites!" {
		t.Fatalf("error = %q", body["error"])
	}
	if body["code"] != "rate-limited" {
		t.Fatalf("code = %v, want rate-limited", body["code"])
	}
	if _, ok := body["request_id"]; !ok {
		t.Error("request_id must be present in the envelope")
	}
}

func TestHandlerTrial_BadTone400(t *testing.T) {
	st := &stubRewriter{}
	h := &Handler{svc: st}
	rec := httptest.NewRecorder()
	h.Trial(rec, newTrialReq(`{"text":"hi","tone":"bogus"}`))

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
	var body map[string]any
	json.Unmarshal(rec.Body.Bytes(), &body)
	if body["code"] != "invalid-input" {
		t.Fatalf("code = %v, want invalid-input", body["code"])
	}
	if st.trialOut.text != "" {
		t.Error("invalid input must not reach the service")
	}
}

func TestClientIP_XForwardedFor(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/rewrite/trial", nil)
	req.Header.Set("X-Forwarded-For", "1.2.3.4, 5.6.7.8")
	if got := clientIP(req); got != "1.2.3.4" {
		t.Fatalf("clientIP = %q, want 1.2.3.4", got)
	}
}

func TestClientIP_RemoteAddrFallback(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/rewrite/trial", nil)
	req.RemoteAddr = "9.9.9.9:5555"
	if got := clientIP(req); got != "9.9.9.9:5555" {
		t.Fatalf("clientIP = %q, want 9.9.9.9:5555", got)
	}
}
