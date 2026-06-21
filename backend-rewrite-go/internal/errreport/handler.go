package errreport

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/tannpv/draftright-rewrite/internal/platform/auth"
	"github.com/tannpv/draftright-rewrite/internal/shared"
)

const maxBodyBytes = 100 * 1024 // Express default json limit; past this → 500 internal (see Ingest)

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
	// #59: reject a top-level `null` body (Node's strict body-parser 400s it
	// before any handling). RejectNullBody buffers through the MaxBytesReader,
	// so an oversize body still surfaces *http.MaxBytesError via readErr and
	// keeps the 500 "request entity too large" parity below (oversize is
	// checked BEFORE null, exactly as Node's body-parser rejects size first).
	buf, isNull, err := shared.RejectNullBody(r)
	if err != nil {
		// MaxBytesReader trips a *http.MaxBytesError once the limit is
		// exceeded. In Node there's no body-limit override and no custom
		// oversize error class: Express's raw-body-parser throws a plain
		// PayloadTooLargeError (NOT a Nest HttpException), so
		// AllExceptionsFilter takes the non-HttpException branch →
		// { status: 500, code: 'internal', message: 'request entity too large' }.
		// PayloadTooLargeError is not a Nest HttpException → AllExceptionsFilter → 500 internal.
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			shared.WriteError(w, r, "internal", "request entity too large")
			return
		}
		shared.WriteError(w, r, "invalid-input", "Invalid request body")
		return
	}
	if isNull {
		shared.WriteNullBodyError(w)
		return
	}
	dec := json.NewDecoder(bytes.NewReader(buf))
	if err := dec.Decode(&body); err != nil {
		shared.WriteError(w, r, "invalid-input", "Invalid request body")
		return
	}

	// ValidationPipe parity: @MaxLength checks run in the middleware→pipe
	// stage, BEFORE the controller body — so this precedes the honeypot.
	if msg := validateErrorReport(body); msg != "" {
		shared.WriteError(w, r, "invalid-input", msg)
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
// Thin wrapper over the shared auth.OptionalUserID (Rule #1: bug_reports +
// feedback reuse the same logic).
func (h *Handler) optionalUserID(r *http.Request) string {
	return auth.OptionalUserID(h.verifier, r)
}
