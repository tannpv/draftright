package extraction

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/tannpv/draftright-rewrite/internal/shared"
)

// extractor is the handler's consumer-side port (*Service satisfies it). A
// small interface keeps the HTTP edge testable with a fake LLM.
type extractor interface {
	Extract(ctx context.Context, text string, kinds []EntityKind) (Response, error)
}

// Handler serves POST /extract. Mounted under RequireAuth in Task 11, so it
// does NOT verify the JWT itself (mirrors @UseGuards(JwtAuthGuard)).
type Handler struct {
	svc extractor
}

// NewHandler wires the extraction service.
func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

// Extract handles POST /extract → 200 (NestJS @HttpCode(200), NOT 201).
func (h *Handler) Extract(w http.ResponseWriter, r *http.Request) {
	var body requestBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		shared.WriteError(w, r, "invalid-input", "Invalid request body")
		return
	}

	text, kinds, errMsg := validateExtract(body)
	if errMsg != "" {
		shared.WriteError(w, r, "invalid-input", errMsg)
		return
	}

	resp, err := h.svc.Extract(r.Context(), text, kinds)
	if err != nil {
		// findDefault() throws BadRequestException → 400 invalid-input. The
		// mapping from the provider layer to this sentinel is wired in Task 11.
		if errors.Is(err, ErrNoDefaultProvider) {
			shared.WriteError(w, r, "invalid-input", "No default AI provider configured")
			return
		}
		// Opaque message — never leak err.Error() on the generic 500 path.
		shared.WriteError(w, r, "internal", "extract failed")
		return
	}

	shared.WriteJSON(w, http.StatusOK, resp)
}
