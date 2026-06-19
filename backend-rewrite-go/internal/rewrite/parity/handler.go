// POST /rewrite — authenticated, NON-streaming Node-parity handler (status 201).
//
// This is a PARALLEL, separate handler from the streaming /v1/rewrite handler
// in internal/rewrite/transport/. Mounted under dual-auth (session JWT OR a
// dr_ext_* token with the 'rewrite' scope) by the router task; the handler does
// NOT verify auth itself — claims are already in context.
package parity

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/tannpv/draftright-rewrite/internal/shared"
)

// rewriter is the handler's consumer-side port; *Service satisfies it. Kept on
// the consumer side (CLAUDE.md guardrail) so tests inject a fake without a DB.
type rewriter interface {
	Rewrite(ctx context.Context, userID, text, tone, target, source string) (any, error)
	TrialRewrite(ctx context.Context, text, tone, clientIp, target, source string) (any, error)
}

// Handler serves POST /rewrite.
type Handler struct {
	svc rewriter
}

// NewHandler wires the rewrite service.
func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

// tonesEnvelope pins the GET /rewrite/tones body: {"tones":[…]} — mirrors Node's
// `return { tones: TONES }`.
type tonesEnvelope struct {
	Tones []ToneMeta `json:"tones"`
}

// WriteTones serves GET /rewrite/tones → 200 (NestJS @Get default status). It
// has no Service dependency — it just marshals the static catalog — so main.go
// wires it unconditionally (no DB).
func WriteTones(w http.ResponseWriter, _ *http.Request) {
	shared.WriteJSON(w, http.StatusOK, tonesEnvelope{Tones: Tones})
}

// Rewrite handles POST /rewrite → 201 (NestJS POST default, no @HttpCode).
func (h *Handler) Rewrite(w http.ResponseWriter, r *http.Request) {
	claims, ok := shared.ClaimsFromContext(r.Context())
	if !ok {
		// RequireAuth/dual-auth middleware not wired upstream — router misconfig.
		// 500 loud so a misroute can't silently accept anonymous traffic.
		shared.WriteError(w, r, "internal", "auth middleware missing")
		return
	}

	raw, err := io.ReadAll(r.Body)
	if err != nil {
		shared.WriteError(w, r, "invalid-input", "Invalid request body")
		return
	}

	text, tone, target, source, msg := validateRewrite(raw)
	if msg != "" {
		shared.WriteError(w, r, "invalid-input", msg)
		return
	}

	out, err := h.svc.Rewrite(r.Context(), claims.UserID(), text, tone, target, source)
	if err != nil {
		var ute *UnknownToneError
		switch {
		case errors.Is(err, ErrQuotaExceeded):
			// AllExceptionsFilter drops the extra usage_today/daily_limit fields;
			// only the bare message survives. code rate-limited → 429.
			shared.WriteError(w, r, "rate-limited", "Daily limit reached")
		case errors.Is(err, ErrNoDefaultProvider):
			shared.WriteError(w, r, "invalid-input", ErrNoDefaultProvider.Error())
		case errors.Is(err, ErrProviderFailed):
			shared.WriteError(w, r, "provider-failed", providerUnavailableMsg)
		case errors.As(err, &ute):
			shared.WriteError(w, r, "invalid-input", ute.Error())
		default:
			// Opaque message — never leak err.Error() on the generic 500 path.
			shared.WriteError(w, r, "internal", "rewrite failed")
		}
		return
	}

	shared.WriteJSON(w, http.StatusCreated, out)
}

// clientIP mirrors the NestJS controller: first X-Forwarded-For hop (trimmed),
// else RemoteAddr, else "unknown". The value only forms the rate-limit key,
// which the shadow gate never inspects (per-fixture DB+limiter reset), so the
// ip:port vs bare-ip difference vs Node is value-invisible.
func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		if first := strings.TrimSpace(strings.Split(xff, ",")[0]); first != "" {
			return first
		}
	}
	if r.RemoteAddr != "" {
		return r.RemoteAddr
	}
	return "unknown"
}

// Trial handles POST /rewrite/trial → 201. Public/unauthenticated: no claims.
func (h *Handler) Trial(w http.ResponseWriter, r *http.Request) {
	raw, err := io.ReadAll(r.Body)
	if err != nil {
		shared.WriteError(w, r, "invalid-input", "Invalid request body")
		return
	}

	text, tone, target, source, msg := validateRewrite(raw)
	if msg != "" {
		shared.WriteError(w, r, "invalid-input", msg)
		return
	}

	out, err := h.svc.TrialRewrite(r.Context(), text, tone, clientIP(r), target, source)
	if err != nil {
		var ute *UnknownToneError
		switch {
		case errors.Is(err, ErrTrialLimit):
			shared.WriteError(w, r, "rate-limited", ErrTrialLimit.Error())
		case errors.Is(err, ErrNoDefaultProvider):
			shared.WriteError(w, r, "invalid-input", ErrNoDefaultProvider.Error())
		case errors.Is(err, ErrProviderFailed):
			shared.WriteError(w, r, "provider-failed", providerUnavailableMsg)
		case errors.As(err, &ute):
			shared.WriteError(w, r, "invalid-input", ute.Error())
		default:
			shared.WriteError(w, r, "internal", "rewrite failed")
		}
		return
	}

	shared.WriteJSON(w, http.StatusCreated, out)
}
