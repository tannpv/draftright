package bugreports

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/tannpv/draftright-rewrite/internal/platform/auth"
	"github.com/tannpv/draftright-rewrite/internal/shared"
)

// maxUploadBytes is multer's limits.fileSize (5 MB) from the controller.
const maxUploadBytes = 5 * 1024 * 1024

// allowedMimes mirrors the controller's ALLOWED_MIMES allow-list exactly
// (bug-reports.controller.ts). NOTE: this is WIDER than the screenshots the
// service can actually STORE (extensionFor knows only png/jpeg/jpg) — webp,
// heic, heif and gif pass the controller filter but then fail in
// Storage.Save with the same "only PNG or JPEG screenshots are accepted"
// string. That two-stage behaviour is faithfully ported from Node.
var allowedMimes = map[string]bool{
	"image/png":  true,
	"image/jpeg": true,
	"image/jpg":  true,
	"image/webp": true,
	"image/heic": true,
	"image/heif": true,
	"image/gif":  true,
}

// errMimeNotAllowed is the controller fileFilter BadRequestException string —
// identical to the service's extensionFor string (both byte-for-byte for the
// shadow gate).
const errMimeNotAllowed = "only PNG or JPEG screenshots are accepted"

// createService is the handler's consumer-side port (Service satisfies it).
type createService interface {
	Create(ctx context.Context, in CreateInput, file *FilePart, userID string) (Created, error)
}

// Handler serves POST /bug-reports. Public; reads an OPTIONAL bearer for
// attribution.
type Handler struct {
	svc      createService
	verifier *auth.Verifier // may be nil in tests
}

// NewHandler wires the service + the access-token verifier.
func NewHandler(svc *Service, v *auth.Verifier) *Handler { return &Handler{svc: svc, verifier: v} }

// honeypotResp is the silent-drop body. Field order matches Node exactly:
// { id, status }. id serialises as literal JSON null.
type honeypotResp struct {
	ID     *string `json:"id"`
	Status string  `json:"status"`
}

// successResp is the normal create body. Field order matches Node exactly:
// { id, ref, message }.
type successResp struct {
	ID      string `json:"id"`
	Ref     string `json:"ref"`
	Message string `json:"message"`
}

// Create handles POST /bug-reports → 201.
//
// NestJS execution order = the parity order we replicate:
//  1. FileInterceptor (multer) parses multipart FIRST — its fileFilter
//     rejects a disallowed mime (→ 400) and limits.fileSize rejects an
//     oversize file (→ multer LIMIT_FILE_SIZE → PayloadTooLargeException 413).
//  2. ValidationPipe validates the DTO (@MinLength/@MaxLength) → 400.
//  3. Controller body: honeypot check, then the service.
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	// Stage 1 (multer): parse multipart. ParseMultipartForm buffers file
	// parts; we cap with MaxBytesReader so an oversize upload trips the
	// 413 path rather than buffering 5 MB+ into memory.
	r.Body = http.MaxBytesReader(w, r.Body, maxUploadBytes+(1<<20)) // +1 MB headroom for fields/boundaries
	if err := r.ParseMultipartForm(maxUploadBytes); err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			h.writeOversize(w, r)
			return
		}
		// Malformed multipart → Node busboy → BadRequestException 400.
		shared.WriteError(w, r, "invalid-input", "Multipart: Unexpected end of form")
		return
	}

	// Extract the optional screenshot file part.
	var file *FilePart
	if r.MultipartForm != nil && len(r.MultipartForm.File["screenshot"]) > 0 {
		fh := r.MultipartForm.File["screenshot"][0]
		// multer limits.fileSize: reject before reading the whole part.
		if fh.Size > maxUploadBytes {
			h.writeOversize(w, r)
			return
		}
		mime := fh.Header.Get("Content-Type")
		// Stage 1 fileFilter: mime allow-list → 400 before validation.
		if !allowedMimes[mime] {
			shared.WriteError(w, r, "invalid-input", errMimeNotAllowed)
			return
		}
		f, err := fh.Open()
		if err != nil {
			shared.WriteError(w, r, "internal", "bug-reports failed")
			return
		}
		buf, err := io.ReadAll(f)
		_ = f.Close()
		if err != nil {
			// A partial/failed read of the (size-bounded) part is a
			// malformed upload → 400, same path as a bad multipart body.
			shared.WriteError(w, r, "invalid-input", "Multipart: Unexpected end of form")
			return
		}
		file = &FilePart{Buffer: buf, OriginalName: fh.Filename, Mimetype: mime}
	}

	flds := readFields(r)

	// Stage 2 (ValidationPipe): DTO constraints, BEFORE the honeypot.
	if msg := validateBugReport(flds); msg != "" {
		shared.WriteError(w, r, "invalid-input", msg)
		return
	}

	// Stage 3a (controller): honeypot — non-empty website → silent 201 drop,
	// no row written. Distinct body { id:null, status:'received' }.
	if strings.TrimSpace(flds.website) != "" {
		shared.WriteJSON(w, http.StatusCreated, honeypotResp{ID: nil, Status: "received"})
		return
	}

	// Stage 3b: optional bearer + service.
	userID := auth.OptionalUserID(h.verifier, r)
	created, err := h.svc.Create(r.Context(), CreateInput{
		Description: flds.description,
		Source:      flds.source,
		AppVersion:  flds.appVersion,
		OsInfo:      flds.osInfo,
		UserEmail:   flds.userEmail,
		Context:     flds.context,
		HasContext:  flds.contextPresent,
	}, file, userID)
	if err != nil {
		switch {
		case errors.Is(err, ErrDescriptionRequired), errors.Is(err, ErrSourceRequired):
			shared.WriteError(w, r, "invalid-input", err.Error())
		case errors.Is(err, errUnsupportedMime):
			// Storage.Save rejected a mime the controller allowed (webp/heic/…).
			shared.WriteError(w, r, "invalid-input", errMimeNotAllowed)
		default:
			// Log the real cause — a Storage.Save EACCES (e.g. an env whose
			// BugReportsDir volume isn't mounted/writable) otherwise vanishes
			// behind the generic 500, making it invisible in server logs (#67).
			slog.Default().ErrorContext(r.Context(), "bug-report create failed",
				"err", err, "has_screenshot", file != nil)
			shared.WriteError(w, r, "internal", "bug-reports failed")
		}
		return
	}

	ref := "BUG-" + strconv.FormatInt(created.DisplayNo, 10)
	shared.WriteJSON(w, http.StatusCreated, successResp{
		ID:      created.ID,
		Ref:     ref,
		Message: "Bug report received. Thanks! Reference: " + ref,
	})
}

// readFields pulls the multipart text fields into the validate struct,
// recording presence for the required fields.
func readFields(r *http.Request) fields {
	get := func(k string) (string, bool) {
		if r.MultipartForm == nil {
			return "", false
		}
		v, ok := r.MultipartForm.Value[k]
		if !ok || len(v) == 0 {
			return "", false
		}
		return v[0], true
	}
	desc, descPresent := get("description")
	src, srcPresent := get("source")
	ctx, ctxPresent := get("context")
	appV, _ := get("app_version")
	osI, _ := get("os_info")
	email, _ := get("user_email")
	site, _ := get("website")
	return fields{
		description:        desc,
		descriptionPresent: descPresent,
		source:             src,
		sourcePresent:      srcPresent,
		appVersion:         appV,
		osInfo:             osI,
		userEmail:          email,
		context:            ctx,
		contextPresent:     ctxPresent,
		website:            site,
	}
}

// oversizeBody is the 413 envelope, in the AllExceptionsFilter key order
// { error, code, request_id }. multer LIMIT_FILE_SIZE → PayloadTooLargeException
// (413); AllExceptionsFilter.inferCode(413) has no mapping so code = "http-413",
// and the message is multer's "File too large".
type oversizeBody struct {
	Error     string `json:"error"`
	Code      string `json:"code"`
	RequestID string `json:"request_id"`
}

// writeOversize emits the exact 413 envelope Node produces for an oversize
// upload. WriteError can't be used: it derives status from the code, and
// "http-413" would map to 500 — here the status is fixed at 413.
func (h *Handler) writeOversize(w http.ResponseWriter, r *http.Request) {
	shared.WriteJSON(w, http.StatusRequestEntityTooLarge, oversizeBody{
		Error:     "File too large",
		Code:      "http-413",
		RequestID: shared.RequestIDFromContext(r.Context()),
	})
}
