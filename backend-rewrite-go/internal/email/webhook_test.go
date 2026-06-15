package email

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// markCall + suppressCall capture what the handler asked the port to do.
type markCall struct {
	id, status string
	reason     *string
}
type suppressCall struct{ email, reason string }

type fakeSuppressor struct {
	marks    []markCall
	suppress []suppressCall
	markErr  error
	suppErr  error
}

func (f *fakeSuppressor) MarkByProviderID(_ context.Context, id, status string, reason *string) error {
	f.marks = append(f.marks, markCall{id: id, status: status, reason: reason})
	return f.markErr
}
func (f *fakeSuppressor) Suppress(_ context.Context, email, reason string) error {
	f.suppress = append(f.suppress, suppressCall{email: email, reason: reason})
	return f.suppErr
}

const testSecret = "whsec_MfKQ9r8GKYqrTwjUPD8ILPZIo2LaLaSw" // base64 body after prefix

// signedRequest builds a POST with a valid Svix signature for body.
func signedRequest(t *testing.T, secret, body string) *http.Request {
	t.Helper()
	const id, ts = "msg_2abc", "1700000000"
	key, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(secret, "whsec_"))
	if err != nil {
		t.Fatalf("decode secret: %v", err)
	}
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(id + "." + ts + "." + body))
	sig := base64.StdEncoding.EncodeToString(mac.Sum(nil))
	r := httptest.NewRequest(http.MethodPost, "/webhooks/resend", strings.NewReader(body))
	r.Header.Set("svix-id", id)
	r.Header.Set("svix-timestamp", ts)
	r.Header.Set("svix-signature", "v1,"+sig)
	return r
}

func serve(h *WebhookHandler, r *http.Request) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	h.Handle(w, r)
	return w
}

func decodeErr(t *testing.T, w *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &m); err != nil {
		t.Fatalf("decode body %q: %v", w.Body.String(), err)
	}
	return m
}

func TestWebhook_BadSignature(t *testing.T) {
	cases := []struct {
		name   string
		secret string
		body   string
		mutate func(*http.Request)
	}{
		{name: "empty secret", secret: "", body: `{}`},
		{name: "non-whsec secret", secret: "nope", body: `{}`},
		{
			name:   "wrong signature",
			secret: testSecret,
			body:   `{}`,
			mutate: func(r *http.Request) { r.Header.Set("svix-signature", "v1,deadbeef") },
		},
		{
			name:   "missing svix headers",
			secret: testSecret,
			body:   `{}`,
			mutate: func(r *http.Request) {
				r.Header.Del("svix-id")
				r.Header.Del("svix-timestamp")
				r.Header.Del("svix-signature")
			},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			h := NewWebhookHandler(&fakeSuppressor{}, c.secret)
			r := signedRequest(t, testSecret, c.body)
			if c.mutate != nil {
				c.mutate(r)
			}
			w := serve(h, r)
			if w.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400", w.Code)
			}
			if got := decodeErr(t, w)["error"]; got != "Invalid webhook signature" {
				t.Fatalf("error = %q, want %q", got, "Invalid webhook signature")
			}
		})
	}
}

func TestWebhook_MalformedJSON(t *testing.T) {
	h := NewWebhookHandler(&fakeSuppressor{}, testSecret)
	w := serve(h, signedRequest(t, testSecret, `{not json`))
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", w.Code)
	}
	if got := decodeErr(t, w)["error"]; got != "Invalid payload" {
		t.Fatalf("error = %q, want %q", got, "Invalid payload")
	}
}

func TestWebhook_Delivered(t *testing.T) {
	f := &fakeSuppressor{}
	h := NewWebhookHandler(f, testSecret)
	body := `{"type":"email.delivered","data":{"email_id":"e_123","to":["a@x.com"]}}`
	w := serve(h, signedRequest(t, testSecret, body))
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	var resp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp["received"] != true {
		t.Fatalf("received = %v, want true", resp["received"])
	}
	if len(f.marks) != 1 {
		t.Fatalf("marks = %d, want 1", len(f.marks))
	}
	m := f.marks[0]
	if m.id != "e_123" || m.status != "delivered" || m.reason != nil {
		t.Fatalf("mark = %+v, want {e_123 delivered <nil>}", m)
	}
	if len(f.suppress) != 0 {
		t.Fatalf("unexpected suppress: %+v", f.suppress)
	}
}

func TestWebhook_BouncedPermanent(t *testing.T) {
	f := &fakeSuppressor{}
	h := NewWebhookHandler(f, testSecret)
	body := `{"type":"email.bounced","data":{"email_id":"e_9","to":"hard@x.com","bounce":{"type":"Permanent","subType":"General","message":"mailbox does not exist"}}}`
	w := serve(h, signedRequest(t, testSecret, body))
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	if len(f.marks) != 1 {
		t.Fatalf("marks = %d, want 1", len(f.marks))
	}
	m := f.marks[0]
	if m.id != "e_9" || m.status != "bounced" || m.reason == nil || *m.reason != "mailbox does not exist" {
		t.Fatalf("mark = %+v reason=%v, want bounced/mailbox does not exist", m, m.reason)
	}
	if len(f.suppress) != 1 {
		t.Fatalf("suppress = %d, want 1", len(f.suppress))
	}
	if s := f.suppress[0]; s.email != "hard@x.com" || s.reason != "bounced" {
		t.Fatalf("suppress = %+v, want {hard@x.com bounced}", s)
	}
}

func TestWebhook_BouncedTransientNoSuppress(t *testing.T) {
	f := &fakeSuppressor{}
	h := NewWebhookHandler(f, testSecret)
	body := `{"type":"email.bounced","data":{"email_id":"e_4","to":"soft@x.com","bounce":{"type":"Transient","subType":"MailboxFull","message":"mailbox full"}}}`
	w := serve(h, signedRequest(t, testSecret, body))
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	if len(f.marks) != 1 {
		t.Fatalf("marks = %d, want 1", len(f.marks))
	}
	if m := f.marks[0]; m.status != "bounced" || m.reason == nil || *m.reason != "mailbox full" {
		t.Fatalf("mark = %+v reason=%v", m, m.reason)
	}
	if len(f.suppress) != 0 {
		t.Fatalf("transient bounce must not suppress, got %+v", f.suppress)
	}
}

func TestWebhook_Complained(t *testing.T) {
	f := &fakeSuppressor{}
	h := NewWebhookHandler(f, testSecret)
	body := `{"type":"email.complained","data":{"email_id":"e_7","to":["spam@x.com"]}}`
	w := serve(h, signedRequest(t, testSecret, body))
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	if len(f.marks) != 1 {
		t.Fatalf("marks = %d, want 1", len(f.marks))
	}
	if m := f.marks[0]; m.id != "e_7" || m.status != "complained" || m.reason == nil || *m.reason != "Recipient marked as spam" {
		t.Fatalf("mark = %+v reason=%v", m, m.reason)
	}
	if len(f.suppress) != 1 {
		t.Fatalf("suppress = %d, want 1", len(f.suppress))
	}
	if s := f.suppress[0]; s.email != "spam@x.com" || s.reason != "complained" {
		t.Fatalf("suppress = %+v, want {spam@x.com complained}", s)
	}
}

func TestWebhook_IgnoredEventType(t *testing.T) {
	f := &fakeSuppressor{}
	h := NewWebhookHandler(f, testSecret)
	body := `{"type":"email.delivery_delayed","data":{"email_id":"e_x","to":["q@x.com"]}}`
	w := serve(h, signedRequest(t, testSecret, body))
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	if len(f.marks) != 0 || len(f.suppress) != 0 {
		t.Fatalf("no-op event mutated state: marks=%+v suppress=%+v", f.marks, f.suppress)
	}
}
