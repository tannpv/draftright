package email

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/tannpv/draftright-rewrite/internal/shared"
)

// suppressor is the consumer-side port the webhook needs: reflect a
// delivery event onto email_logs (MarkByProviderID) and grow the
// suppression list (Suppress). *PgRepo satisfies it.
type suppressor interface {
	MarkByProviderID(ctx context.Context, id, status string, reason *string) error
	Suppress(ctx context.Context, email, reason string) error
}

// WebhookHandler receives Resend delivery events (Svix-signed) and
// reflects them onto email_logs.status + the suppression list. Hard
// bounces and spam complaints stop us from emailing that address again.
// Ports the NestJS EmailWebhookController 1:1.
type WebhookHandler struct {
	svc    suppressor
	secret string
}

// NewWebhookHandler wires the suppressor port + the RESEND_WEBHOOK_SECRET.
func NewWebhookHandler(svc suppressor, secret string) *WebhookHandler {
	return &WebhookHandler{svc: svc, secret: secret}
}

// webhookEvent mirrors the Resend payload shape the controller reads.
type webhookEvent struct {
	Type string `json:"type"`
	Data struct {
		EmailID string          `json:"email_id"`
		To      json.RawMessage `json:"to"` // string OR []string
		Reason  string          `json:"reason"`
		Bounce  struct {
			Type    string `json:"type"`
			SubType string `json:"subType"`
			Message string `json:"message"`
		} `json:"bounce"`
	} `json:"data"`
}

// Handle is POST /webhooks/resend. The router MUST mount this WITHOUT any
// body-consuming middleware (we read the raw body here for signature
// verification) and WITHOUT RequireAuth (public webhook).
func (h *WebhookHandler) Handle(w http.ResponseWriter, r *http.Request) {
	raw, err := io.ReadAll(r.Body)
	if err != nil {
		// Fail closed: an unreadable body can't be signature-verified, so
		// return the exact same 400 we return for a bad signature.
		shared.WriteError(w, r, "invalid-input", "Invalid webhook signature")
		return
	}

	if h.secret == "" || !verify(h.secret, r.Header, raw) {
		shared.WriteError(w, r, "invalid-input", "Invalid webhook signature")
		return
	}

	var event webhookEvent
	if err := json.Unmarshal(raw, &event); err != nil {
		shared.WriteError(w, r, "invalid-input", "Invalid payload")
		return
	}

	ctx := r.Context()
	id := event.Data.EmailID
	to := firstRecipient(event.Data.To)

	switch event.Type {
	case "email.delivered":
		if id != "" {
			_ = h.svc.MarkByProviderID(ctx, id, "delivered", nil)
		}
	case "email.bounced":
		// bounce.message || data.reason || 'bounced'
		reason := event.Data.Bounce.Message
		if reason == "" {
			reason = event.Data.Reason
		}
		if reason == "" {
			reason = "bounced"
		}
		if id != "" {
			reasonPtr := reason
			_ = h.svc.MarkByProviderID(ctx, id, "bounced", &reasonPtr)
		}
		// Only PERMANENT/hard bounces suppress — a transient bounce
		// (full mailbox, greylisting) must not lock a real user out.
		kind := strings.ToLower(event.Data.Bounce.Type + " " + event.Data.Bounce.SubType)
		if to != "" && (strings.Contains(kind, "permanent") || strings.Contains(kind, "hard")) {
			_ = h.svc.Suppress(ctx, to, "bounced")
		}
	case "email.complained":
		if id != "" {
			reasonPtr := "Recipient marked as spam"
			_ = h.svc.MarkByProviderID(ctx, id, "complained", &reasonPtr)
		}
		if to != "" {
			_ = h.svc.Suppress(ctx, to, "complained")
		}
	default:
		// ignore sent / opened / clicked / delivery_delayed
	}

	shared.WriteJSON(w, http.StatusOK, struct {
		Received bool `json:"received"`
	}{Received: true})
}

// firstRecipient mirrors Node: `Array.isArray(to) ? to[0] : to`. The
// Resend payload sends `to` as a string or a string array.
func firstRecipient(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var arr []string
	if err := json.Unmarshal(raw, &arr); err == nil {
		if len(arr) > 0 {
			return arr[0]
		}
		return ""
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s
	}
	return ""
}

// verify ports the controller's Svix signature check exactly. Node does
// NOT reject a non-whsec_ secret — it strips an optional whsec_ prefix
// then base64-decodes the remainder as the HMAC key (a bad key simply
// fails the comparison). Returns false only when no svix-id /
// svix-timestamp / svix-signature header is present, or no part matches.
func verify(secret string, hdr http.Header, body []byte) bool {
	id := hdr.Get("svix-id")
	ts := hdr.Get("svix-timestamp")
	sigHeader := hdr.Get("svix-signature")
	if id == "" || ts == "" || sigHeader == "" {
		return false
	}
	key, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(secret, "whsec_"))
	if err != nil {
		return false
	}
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(id + "." + ts + "." + string(body)))
	expected := []byte(base64.StdEncoding.EncodeToString(mac.Sum(nil)))

	// The header is a space-separated list of `v1,<base64sig>` entries.
	for _, part := range strings.Split(sigHeader, " ") {
		idx := strings.IndexByte(part, ',')
		if idx < 0 {
			continue
		}
		sig := []byte(part[idx+1:])
		if len(sig) == len(expected) && hmac.Equal(sig, expected) {
			return true
		}
	}
	return false
}
