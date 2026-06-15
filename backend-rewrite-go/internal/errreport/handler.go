package errreport

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/tannpv/draftright-rewrite/internal/platform/auth"
	"github.com/tannpv/draftright-rewrite/internal/shared"
)

const maxBodyBytes = 100 * 1024 // Express default json limit → 413 past this

// ingestService is the handler's consumer-side port (Service satisfies it).
type ingestService interface {
	Ingest(ctx context.Context, in CreateErrorReport, userID string) (*Existing, error)
}

// Handler serves POST /errors. Public; reads an OPTIONAL bearer for
// attribution.
type Handler struct {
	svc      ingestService
	verifier *auth.Verifier // may be nil in tests
}

// NewHandler wires the service + the access-token verifier.
func NewHandler(svc *Service, v *auth.Verifier) *Handler { return &Handler{svc: svc, verifier: v} }

// requestBody mirrors the Node DTO field names (snake_case JSON).
type requestBody struct {
	Platform   string          `json:"platform"`
	AppVersion string          `json:"app_version"`
	Severity   string          `json:"severity"`
	ErrorType  string          `json:"error_type"`
	Message    string          `json:"message"`
	StackTrace string          `json:"stack_trace"`
	Context    json.RawMessage `json:"context"`
	DeviceID   string          `json:"device_id"`
	Website    string          `json:"website"` // honeypot
}

// honeypotResp is the silent-drop body. Field order matches Node exactly:
// { ok, id, fingerprint, count, first_seen_at }.
type honeypotResp struct {
	OK          bool    `json:"ok"`
	ID          *string `json:"id"`
	Fingerprint *string `json:"fingerprint"`
	Count       int     `json:"count"`
	FirstSeenAt *string `json:"first_seen_at"`
}

// successResp is the normal ingest body. Field order matches Node exactly:
// { ok, id, ref, fingerprint, count, first_seen_at }.
type successResp struct {
	OK          bool   `json:"ok"`
	ID          string `json:"id"`
	Ref         string `json:"ref"`
	Fingerprint string `json:"fingerprint"`
	Count       int32  `json:"count"`
	FirstSeenAt string `json:"first_seen_at"`
}

// Ingest handles POST /errors → 201.
func (h *Handler) Ingest(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	var body requestBody
	dec := json.NewDecoder(r.Body)
	if err := dec.Decode(&body); err != nil {
		// MaxBytesReader trips a *http.MaxBytesError once the limit is
		// exceeded → mirror Express's 413 (AllExceptionsFilter inferCode).
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			shared.WriteError(w, r, "http-413", "request entity too large")
			return
		}
		shared.WriteError(w, r, "invalid-input", "Invalid request body")
		return
	}

	// Honeypot: any non-empty `website` → silent 201 drop, distinct body
	// (no ref). Happens BEFORE the bearer read and BEFORE the service.
	if strings.TrimSpace(body.Website) != "" {
		shared.WriteJSON(w, http.StatusCreated, honeypotResp{OK: true})
		return
	}

	userID := h.optionalUserID(r)

	var ctxBytes []byte
	if len(body.Context) > 0 && string(body.Context) != "null" {
		ctxBytes = []byte(body.Context)
	}
	res, err := h.svc.Ingest(r.Context(), CreateErrorReport{
		Platform: body.Platform, AppVersion: body.AppVersion, Severity: body.Severity,
		ErrorType: body.ErrorType, Message: body.Message, StackTrace: body.StackTrace,
		Context: ctxBytes, DeviceID: body.DeviceID, Website: body.Website,
	}, userID)
	if err != nil {
		if errors.Is(err, ErrInvalidPlatform) {
			shared.WriteError(w, r, "invalid-input", err.Error())
			return
		}
		// Opaque message — never leak err.Error() on the generic 500 path.
		shared.WriteError(w, r, "internal", "errors failed")
		return
	}

	shared.WriteJSON(w, http.StatusCreated, successResp{
		OK:          true,
		ID:          res.ID,
		Ref:         "ERR-" + strconv.FormatInt(res.DisplayNo, 10),
		Fingerprint: res.Fingerprint,
		Count:       res.Count,
		FirstSeenAt: shared.ISOMillis(res.FirstSeenAt),
	})
}

// optionalUserID extracts `sub` from a best-effort bearer; "" on any failure.
func (h *Handler) optionalUserID(r *http.Request) string {
	if h.verifier == nil {
		return ""
	}
	authz := r.Header.Get("Authorization")
	const p = "Bearer "
	// Node: authHeader.startsWith('Bearer ') — case-sensitive prefix.
	if !strings.HasPrefix(authz, p) {
		return ""
	}
	claims, err := h.verifier.Verify(authz[len(p):])
	if err != nil {
		return ""
	}
	return claims.UserID()
}
