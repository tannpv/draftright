package extraction

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// stubExtractor satisfies the handler's consumer-side extractor port, so the
// LLM never runs in tests.
type stubExtractor struct {
	resp Response
	err  error
	got  struct {
		text  string
		kinds []EntityKind
	}
}

func (s *stubExtractor) Extract(_ context.Context, text string, kinds []EntityKind) (Response, error) {
	s.got.text, s.got.kinds = text, kinds
	return s.resp, s.err
}

func newTestHandler(e extractor) *Handler { return &Handler{svc: e} }

func TestExtract_TextTooLong400(t *testing.T) {
	st := &stubExtractor{}
	h := newTestHandler(st)
	rec := httptest.NewRecorder()
	long := strings.Repeat("a", 8001)
	req := httptest.NewRequest(http.MethodPost, "/extract", strings.NewReader(`{"text":"`+long+`"}`))
	h.Extract(rec, req)

	if rec.Code != 400 {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if body["code"] != "invalid-input" {
		t.Fatalf("code = %v, want invalid-input", body["code"])
	}
	if body["error"] != "text must be shorter than or equal to 8000 characters" {
		t.Fatalf("error = %q, want length message", body["error"])
	}
	if st.got.text != "" {
		t.Error("invalid input must not reach the service")
	}
}

func TestExtract_BadKind400(t *testing.T) {
	st := &stubExtractor{}
	h := newTestHandler(st)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/extract",
		strings.NewReader(`{"text":"hi","kinds":["phone","bogus"]}`))
	h.Extract(rec, req)

	if rec.Code != 400 {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if body["code"] != "invalid-input" {
		t.Fatalf("code = %v, want invalid-input", body["code"])
	}
	want := "each value in kinds must be one of the following values: phone, email, url, otp, creditCard, address, personName, dateTime, bankAccount"
	if body["error"] != want {
		t.Fatalf("error = %q, want @IsEnum each message %q", body["error"], want)
	}
}

func TestExtract_MissingText400(t *testing.T) {
	// Node ValidationPipe on a missing required `text`: @MaxLength reports first,
	// then @IsString, joined by AllExceptionsFilter with ". ". Verified by running
	// class-validator against ExtractRequestDto: body {} →
	// ["text must be shorter than or equal to 8000 characters","text must be a string"].
	st := &stubExtractor{}
	h := newTestHandler(st)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/extract", strings.NewReader(`{}`))
	h.Extract(rec, req)

	if rec.Code != 400 {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if body["code"] != "invalid-input" {
		t.Fatalf("code = %v, want invalid-input", body["code"])
	}
	want := "text must be shorter than or equal to 8000 characters. text must be a string"
	if body["error"] != want {
		t.Fatalf("error = %q, want %q", body["error"], want)
	}
	if st.got.text != "" {
		t.Error("invalid input must not reach the service")
	}
}

func TestExtract_NonStringText400(t *testing.T) {
	// {"text":123} is well-formed JSON but `text` is not a string. Node parses
	// the body fine then the pipe emits the same two-message join as a missing
	// text. Go must NOT collapse this to the generic "Invalid request body".
	st := &stubExtractor{}
	h := newTestHandler(st)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/extract", strings.NewReader(`{"text":123}`))
	h.Extract(rec, req)

	if rec.Code != 400 {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if body["code"] != "invalid-input" {
		t.Fatalf("code = %v, want invalid-input", body["code"])
	}
	want := "text must be shorter than or equal to 8000 characters. text must be a string"
	if body["error"] != want {
		t.Fatalf("error = %q, want %q", body["error"], want)
	}
	if st.got.text != "" {
		t.Error("invalid input must not reach the service")
	}
}

func TestExtract_UnknownKey400(t *testing.T) {
	// Node global pipe (whitelist + forbidNonWhitelisted) → 400 on an unknown
	// body property. The Go rewrite module collapses DisallowUnknownFields to the
	// generic invalid-input 400; match that established idiom (status, not the
	// class-validator "property X should not exist" message).
	st := &stubExtractor{}
	h := newTestHandler(st)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/extract", strings.NewReader(`{"text":"hi","bogus":1}`))
	h.Extract(rec, req)

	if rec.Code != 400 {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if body["code"] != "invalid-input" {
		t.Fatalf("code = %v, want invalid-input", body["code"])
	}
	if st.got.text != "" {
		t.Error("unknown-field body must not reach the service")
	}
}

func TestExtract_HappyPath200(t *testing.T) {
	st := &stubExtractor{resp: Response{
		Entities:   []Entity{{Kind: KindAddress, Value: "123 Main", Display: "123 Main", Start: 0, End: 8, Confidence: 0.9}},
		Provider:   "ollama",
		TokensUsed: 7,
	}}
	h := newTestHandler(st)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/extract",
		strings.NewReader(`{"text":"123 Main","kinds":["address"]}`))
	h.Extract(rec, req)

	if rec.Code != 200 {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	raw := rec.Body.String()
	// Node key order: entities, provider, tokensUsed.
	assertOrder(t, raw, "entities", "provider", "tokensUsed")
	var body map[string]any
	if err := json.Unmarshal([]byte(raw), &body); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if body["provider"] != "ollama" || body["tokensUsed"] != float64(7) {
		t.Fatalf("body = %v", body)
	}
	ents, ok := body["entities"].([]any)
	if !ok || len(ents) != 1 {
		t.Fatalf("entities = %v, want 1 element", body["entities"])
	}
	// The handler must forward the parsed kinds to the service.
	if len(st.got.kinds) != 1 || st.got.kinds[0] != KindAddress {
		t.Fatalf("service got kinds = %v, want [address]", st.got.kinds)
	}
}

func TestExtract_EmptyEntitiesSerializeAsArray(t *testing.T) {
	// Service returns no entities → JSON must be "entities":[] (not null).
	st := &stubExtractor{resp: Response{Entities: []Entity{}, Provider: "ollama", TokensUsed: 0}}
	h := newTestHandler(st)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/extract", strings.NewReader(`{"text":"hi"}`))
	h.Extract(rec, req)

	if rec.Code != 200 {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `"entities":[]`) {
		t.Fatalf("entities must serialize as [], got %s", rec.Body.String())
	}
}

func TestExtract_NoDefaultProvider400(t *testing.T) {
	st := &stubExtractor{err: ErrNoDefaultProvider}
	h := newTestHandler(st)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/extract", strings.NewReader(`{"text":"hi"}`))
	h.Extract(rec, req)

	if rec.Code != 400 {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if body["code"] != "invalid-input" {
		t.Fatalf("code = %v, want invalid-input", body["code"])
	}
	if body["error"] != "No default AI provider configured" {
		t.Fatalf("error = %q, want provider message", body["error"])
	}
}

func TestExtract_ServiceErrorOpaque500(t *testing.T) {
	st := &stubExtractor{err: context.DeadlineExceeded}
	h := newTestHandler(st)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/extract", strings.NewReader(`{"text":"hi"}`))
	h.Extract(rec, req)

	if rec.Code != 500 {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if body["code"] != "internal" {
		t.Fatalf("code = %v, want internal", body["code"])
	}
	// Must never leak err.Error() (e.g. "context deadline exceeded").
	if strings.Contains(body["error"].(string), "deadline") {
		t.Fatalf("500 leaked underlying error: %v", body["error"])
	}
}

// assertOrder fails unless the JSON keys appear in raw in the given order.
func assertOrder(t *testing.T, raw string, keys ...string) {
	t.Helper()
	prev := -1
	for _, k := range keys {
		idx := strings.Index(raw, `"`+k+`"`)
		if idx < 0 {
			t.Fatalf("key %q missing from body: %s", k, raw)
		}
		if idx <= prev {
			t.Fatalf("key %q out of order in body: %s", k, raw)
		}
		prev = idx
	}
}

// #59 — top-level `null` body: Node's strict body-parser 400s before the DTO
// pipe; the DisallowUnknownFields decoder no-ops it, so the null guard must.
func TestExtract_NullBody400(t *testing.T) {
	st := &stubExtractor{}
	h := newTestHandler(st)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/extract", strings.NewReader("null"))
	h.Extract(rec, req)
	if rec.Code != 400 {
		t.Fatalf("status = %d, want 400; body=%s", rec.Code, rec.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if body["error"] != `Unexpected token 'n', "null" is not valid JSON` {
		t.Fatalf("error = %v, want null body message", body["error"])
	}
	if body["code"] != "invalid-input" {
		t.Fatalf("code = %v, want invalid-input", body["code"])
	}
	if st.got.text != "" {
		t.Error("null body must not reach the service")
	}
}
