package email

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/tannpv/draftright-rewrite/internal/shared"
	"github.com/tannpv/emailkit"
)

// A real base64 key, mirroring emailkit's own webhook_test.go fixture:
// verify() strips an optional whsec_ prefix and base64-decodes the
// remainder as the HMAC key.
const webhookTestSecret = "whsec_c3VwZXJzZWNyZXR0ZXN0a2V5MTIzNDU2"

// webhookTestNow freezes the clock so signed requests don't drift out of the
// replay window as wall-clock time passes between test runs.
var webhookTestNow = time.Unix(1_700_000_000, 0)

// signWebhookRequest builds a correctly-signed Resend webhook POST. Ported
// from emailkit's own signedRequest test helper (unexported there, so it
// can't be imported) — this package only needs to construct a request that
// passes verify(), not to exercise verify() itself.
func signWebhookRequest(t *testing.T, secret string, ts time.Time, body string) *http.Request {
	t.Helper()
	id := "msg_test"
	tss := strconv.FormatInt(ts.Unix(), 10)
	key, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(secret, "whsec_"))
	if err != nil {
		t.Fatalf("bad test secret: %v", err)
	}
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(id + "." + tss + "." + body))
	sig := base64.StdEncoding.EncodeToString(mac.Sum(nil))

	req := httptest.NewRequest(http.MethodPost, "/webhooks/resend", strings.NewReader(body))
	req.Header.Set("svix-id", id)
	req.Header.Set("svix-timestamp", tss)
	req.Header.Set("svix-signature", "v1,"+sig)
	return req
}

// frozenWebhookHandler builds a *emailkit.WebhookHandler pinned to
// webhookTestNow, the constructor every WebhookResponder test in this file
// shares.
func frozenWebhookHandler(store emailkit.Store) *emailkit.WebhookHandler {
	return emailkit.NewWebhookHandler(store, webhookTestSecret, emailkit.WithClock(func() time.Time { return webhookTestNow }))
}

// assertOpaqueBody checks the response body never echoes which check failed
// or the underlying error text — the whole point of routing rejections
// through one opaque message (see WebhookResponder's doc comment).
func assertOpaqueBody(t *testing.T, w *httptest.ResponseRecorder, forbidden ...string) {
	t.Helper()
	body := w.Body.String()
	for _, f := range forbidden {
		if strings.Contains(body, f) {
			t.Errorf("response body %q leaks internal detail %q", body, f)
		}
	}
}

func TestWebhookResponder_Success(t *testing.T) {
	store := &recordingStore{}
	body := `{"type":"email.delivered","data":{"email_id":"e_123","to":["a@x.com"]}}`
	req := signWebhookRequest(t, webhookTestSecret, webhookTestNow, body)
	w := httptest.NewRecorder()

	WebhookResponder(frozenWebhookHandler(store))(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body: %s)", w.Code, w.Body.String())
	}
	if got, want := w.Body.String(), `{"received":true}`; got != want {
		t.Fatalf("body = %q, want emailkit's own success body %q", got, want)
	}
}

func TestWebhookResponder_StoreFailure_Is5xx(t *testing.T) {
	store := &recordingStore{markErr: errors.New("connection refused")}
	body := `{"type":"email.delivered","data":{"email_id":"e_123","to":["a@x.com"]}}`
	req := signWebhookRequest(t, webhookTestSecret, webhookTestNow, body)
	w := httptest.NewRecorder()

	WebhookResponder(frozenWebhookHandler(store))(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500 (body: %s)", w.Code, w.Body.String())
	}
	assertOpaqueBody(t, w, "connection refused")
}

func TestWebhookResponder_NonRetryableSentinels_Are4xx(t *testing.T) {
	cases := []struct {
		name string
		req  func(t *testing.T) *http.Request
	}{
		{
			name: "bad signature",
			req: func(t *testing.T) *http.Request {
				req := signWebhookRequest(t, webhookTestSecret, webhookTestNow, `{}`)
				req.Header.Set("svix-signature", "v1,deadbeef")
				return req
			},
		},
		{
			name: "stale timestamp",
			req: func(t *testing.T) *http.Request {
				return signWebhookRequest(t, webhookTestSecret, webhookTestNow.Add(-time.Hour), `{}`)
			},
		},
		{
			name: "malformed payload",
			req: func(t *testing.T) *http.Request {
				return signWebhookRequest(t, webhookTestSecret, webhookTestNow, `{not json`)
			},
		},
	}
	var bodies []string
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			w := httptest.NewRecorder()

			WebhookResponder(frozenWebhookHandler(&recordingStore{}))(w, c.req(t))

			if w.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400 (body: %s)", w.Code, w.Body.String())
			}
			bodies = append(bodies, w.Body.String())
		})
	}
	// The three sentinels are kept distinguishable in the server log (see the
	// WARN assertions this test's log output carries) precisely so an
	// operator can tell a rotated secret from clock skew — but the HTTP
	// response must be byte-identical across all three, or that same
	// discrimination leaks to whoever is probing the endpoint.
	for i := 1; i < len(bodies); i++ {
		if bodies[i] != bodies[0] {
			t.Errorf("response body for %q differs from %q: %q vs %q", cases[i].name, cases[0].name, bodies[i], bodies[0])
		}
	}
}

// TestClassifyWebhookErr_UnrecognisedError_Is5xx is the direct unit test of
// the inverted default: emailkit.WebhookHandler.Handle can only ever return
// its own four sentinels, so there is no way to drive a genuinely unknown
// error through the real Handle. This calls the extracted classifier
// directly with a sentinel emailkit does not define at all, proving the
// default lands on "retryable" rather than "not retryable" for anything this
// package doesn't explicitly recognise.
func TestClassifyWebhookErr_UnrecognisedError_Is5xx(t *testing.T) {
	code, retryable := classifyWebhookErr(errors.New("emailkit: some future sentinel"))
	if !retryable {
		t.Fatalf("retryable = false, want true for an unrecognised error")
	}
	if code != shared.CodeInternal {
		t.Fatalf("code = %q, want %q", code, shared.CodeInternal)
	}
}
