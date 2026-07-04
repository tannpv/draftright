package transport

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/tannpv/draftright-rewrite/internal/rewrite/domain"
	"github.com/tannpv/draftright-rewrite/internal/rewrite/usecase"
	"github.com/tannpv/draftright-rewrite/internal/shared"
)

// MaxBodyBytes caps the request body. 64 KiB is comfortably above the
// 5000-char input limit (worst case ~20 KB UTF-8) and protects against
// runaway POST bodies. NestJS uses Express's default 100 KB; we match
// the spirit, not the byte count.
const MaxBodyBytes = 64 * 1024

// RewriteHandler is the chi-mounted handler for POST /v1/rewrite.
// Built once at startup with its dependency bundle, then satisfies the
// http.Handler interface. Constructor takes the use case deps as a
// struct so future ports (cache, audit) plug in without churning the
// handler's signature (Rule #1 — extendable).
type RewriteHandler struct {
	Deps usecase.RewriteDeps
	Log  *slog.Logger
	// Now is injectable for tests that want to assert on duration_ms;
	// production wires time.Now.
	Now func() time.Time
}

// rewriteBody is the request DTO. Field names mirror the NestJS
// RewriteDto (text / tone / target_language) so the same client
// payload reaches both backends. source_language is accepted but
// currently unused — added now so we don't need a wire-format bump
// when Task 8 ports the translate path.
type rewriteBody struct {
	Text           string `json:"text"`
	Tone           string `json:"tone"`
	TargetLanguage string `json:"target_language,omitempty"`
	SourceLanguage string `json:"source_language,omitempty"`
	InputKind      string `json:"input_kind,omitempty"`
}

// ServeHTTP is the entry point. Three phases:
//  1. Parse + validate request (synchronous, no I/O).
//  2. Resolve user from JWT claims, run the use case.
//  3. Stream tokens — SSE when the client asked for it, single JSON
//     response otherwise.
func (h *RewriteHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	claims, ok := shared.ClaimsFromContext(r.Context())
	if !ok {
		// RequireAuth middleware not wired upstream — bug in router.
		// Pick this up loud (500) so a misroute can't silently accept
		// anonymous traffic.
		h.Log.Error("rewrite: missing claims in context (router misconfig)")
		writeErrorJSON(w, http.StatusInternalServerError, "internal", "auth middleware missing")
		return
	}

	userID, err := domain.ParseUserID(claims.UserID())
	if err != nil {
		writeErrorJSON(w, http.StatusUnauthorized, "invalid-subject", "JWT sub is not a UUID")
		return
	}

	req, err := h.parseBody(r)
	if err != nil {
		status, code := statusForDomainErr(err)
		writeErrorJSON(w, status, code, err.Error())
		return
	}

	tokens, errs, err := usecase.Rewrite(r.Context(), h.Deps, userID, req)
	if err != nil {
		status, code := statusForDomainErr(err)
		writeErrorJSON(w, status, code, err.Error())
		return
	}

	if wantsSSE(r) {
		h.streamSSE(r.Context(), w, tokens, errs)
		return
	}
	h.streamJSON(r.Context(), w, tokens, errs)
}

// parseBody enforces the size cap, decodes JSON, and returns a
// validated domain.RewriteRequest. Returns domain sentinel errors so
// the caller maps via statusForDomainErr.
func (h *RewriteHandler) parseBody(r *http.Request) (domain.RewriteRequest, error) {
	if !strings.HasPrefix(strings.ToLower(r.Header.Get("Content-Type")), "application/json") {
		return domain.RewriteRequest{}, domain.ErrInvalidInput
	}
	r.Body = http.MaxBytesReader(nil, r.Body, MaxBodyBytes)
	var body rewriteBody
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&body); err != nil {
		if errors.Is(err, io.EOF) || errors.As(err, new(*json.SyntaxError)) {
			return domain.RewriteRequest{}, domain.ErrInvalidInput
		}
		// http.MaxBytesError, unknown-fields, type mismatches all
		// collapse here. Caller surfaces 400.
		return domain.RewriteRequest{}, domain.ErrInvalidInput
	}
	return domain.NewRewriteRequest(body.Text, body.Tone, body.TargetLanguage, body.InputKind)
}

// streamSSE consumes the use-case channels and forwards each token
// as a `data: {"delta": …}` line. Terminal usage event + `[DONE]`
// marker on clean close; `event: error` on mid-stream failure.
func (h *RewriteHandler) streamSSE(ctx context.Context, w http.ResponseWriter, tokens <-chan string, errs <-chan error) {
	sse, err := newSSEWriter(w)
	if err != nil {
		// Server-side: the handler chain doesn't support flushing. We
		// can still return JSON.
		h.Log.Warn("rewrite: SSE unsupported, falling back to JSON", "err", err.Error())
		h.streamJSON(ctx, w, tokens, errs)
		return
	}
	sse.Open()

	var (
		now       = h.now()
		inputLen  int32
		outputLen int32
	)
	// We don't have direct access to inputLen here — the use case
	// already validated. Leave 0; future revision can include it via
	// a use-case return value if mobile insists.
	_ = inputLen

	for {
		select {
		case <-ctx.Done():
			// Client disconnected. Drain remaining channels so the
			// use case goroutine doesn't block on send.
			drain(tokens, errs)
			return
		case tok, open := <-tokens:
			if !open {
				// Stream finished. Race-safe: drain any late error
				// before declaring success (mirrors use case logic).
				select {
				case e, ok := <-errs:
					if ok && e != nil {
						h.emitSSEError(sse, e)
						return
					}
				default:
				}
				elapsed := int32(time.Since(now).Milliseconds())
				_ = sse.writeUsage(0, outputLen, elapsed)
				_ = sse.writeDone()
				return
			}
			outputLen += int32(len(tok))
			if err := sse.writeDelta(tok); err != nil {
				h.Log.Warn("rewrite: SSE write failed; dropping client", "err", err.Error())
				drain(tokens, errs)
				return
			}
		case e, open := <-errs:
			if !open {
				errs = nil
				continue
			}
			if e != nil {
				h.emitSSEError(sse, e)
				drain(tokens, nil)
				return
			}
		}
	}
}

// streamJSON accumulates every token then emits a single JSON object
// — back-compat path for clients that don't speak SSE. Same wire
// shape NestJS already serves so the cutover is invisible to them.
func (h *RewriteHandler) streamJSON(ctx context.Context, w http.ResponseWriter, tokens <-chan string, errs <-chan error) {
	var b strings.Builder
	for {
		select {
		case <-ctx.Done():
			drain(tokens, errs)
			writeErrorJSON(w, http.StatusGatewayTimeout, "client-disconnect", "client closed connection")
			return
		case tok, open := <-tokens:
			if !open {
				// Drain late error.
				select {
				case e, ok := <-errs:
					if ok && e != nil {
						status, code := statusForDomainErr(e)
						writeErrorJSON(w, status, code, e.Error())
						return
					}
				default:
				}
				shared.WriteJSON(w, http.StatusOK, map[string]string{
					"text":    b.String(),
					"service": "rewrite-go",
				})
				return
			}
			b.WriteString(tok)
		case e, open := <-errs:
			if !open {
				errs = nil
				continue
			}
			if e != nil {
				status, code := statusForDomainErr(e)
				writeErrorJSON(w, status, code, e.Error())
				drain(tokens, nil)
				return
			}
		}
	}
}

func (h *RewriteHandler) emitSSEError(sse *sseWriter, err error) {
	_, code := statusForDomainErr(err)
	_ = sse.writeError(code, err.Error())
	_ = sse.writeDone()
}

// drain consumes any remaining values on the channels so the upstream
// goroutine doesn't block sending into a now-abandoned receiver.
// Returns when both channels close.
func drain(tokens <-chan string, errs <-chan error) {
	if tokens == nil && errs == nil {
		return
	}
	for tokens != nil || errs != nil {
		select {
		case _, ok := <-tokens:
			if !ok {
				tokens = nil
			}
		case _, ok := <-errs:
			if !ok {
				errs = nil
			}
		}
	}
}

// wantsSSE returns true when the client's Accept header asks for
// text/event-stream. We match prefix so quality-values
// ("text/event-stream;q=1.0") still count.
func wantsSSE(r *http.Request) bool {
	a := r.Header.Get("Accept")
	if a == "" {
		return false
	}
	for _, part := range strings.Split(a, ",") {
		if strings.HasPrefix(strings.TrimSpace(part), "text/event-stream") {
			return true
		}
	}
	return false
}

func (h *RewriteHandler) now() time.Time {
	if h.Now != nil {
		return h.Now()
	}
	return time.Now()
}

// writeErrorJSON emits a JSON error body using the shared WriteJSON helper
// (Rule #1 — one place owns the wire write).
func writeErrorJSON(w http.ResponseWriter, status int, code, msg string) {
	shared.WriteJSON(w, status, httpError{Error: msg, Code: code})
}
