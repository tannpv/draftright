package aiprovider

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/tannpv/draftright-rewrite/internal/shared"
	"github.com/tannpv/draftright-rewrite/internal/shared/listquery"
)

// aiSearchCols / aiSortAllow are this consumer's listquery config for the
// paginated route (admin.controller.ts → findAllPaginated). searchCols are
// the columns ILIKE-matched against `search`; sortAllow is the injection-safe
// allow-list mapping public sort keys → SQL columns.
var aiSearchCols = []string{"name", "type", "model"}
var aiSortAllow = map[string]string{
	"name": "name", "type": "type", "model": "model",
	"is_default": "is_default", "is_active": "is_active", "created_at": "created_at",
}

// aiProviderService is the handler's consumer-side port; *Service satisfies it.
// Kept on the consumer side (CLAUDE.md guardrail) so tests inject the real
// Service with faked repo/factory.
type aiProviderService interface {
	List(ctx context.Context) ([]AiProvider, error)
	ListPaginated(ctx context.Context, b listquery.Built) ([]AiProvider, int, error)
	Create(ctx context.Context, in NewProvider) (AiProvider, error)
	Update(ctx context.Context, id string, p ProviderPatch) (AiProvider, error)
	SoftDelete(ctx context.Context, id string) error
	Test(ctx context.Context, id string) TestResult
}

// Handler serves the admin ai-providers routes. All routes mount inside the
// admin group (jwtMW → RequireAdmin) wired in the router task; the handler
// itself does no auth.
//
//	GET    /admin/ai-providers            list (created_at ASC array)
//	GET    /admin/ai-providers/paginated  listquery → { rows, total }
//	POST   /admin/ai-providers            create → 201
//	PATCH  /admin/ai-providers/:id        partial update → 200
//	DELETE /admin/ai-providers/:id        soft delete → 200 { success:true }
//	POST   /admin/ai-providers/:id/test   live provider call → 200
type Handler struct {
	svc aiProviderService
}

// NewHandler wires the service.
func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

// paginatedResponse is the { rows, total } body of the paginated route. Field
// order matches Node's findAllPaginated return exactly.
type paginatedResponse struct {
	Rows  []AiProvider `json:"rows"`
	Total int          `json:"total"`
}

// createBody decodes the POST body
// { name, type, endpoint_url, api_key?, model, temperature?, is_default?,
// is_active? }. Pointers distinguish an absent key (nil) from an explicit
// value — temperature absent defaults to "0.3" (the DB column default Node
// relies on); is_default absent defaults to false (its column default), and
// is_active absent defaults to TRUE — the ai_provider entity column default is
// `true`, and Node's create() lets that default apply, so an omitted is_active
// persists true (NOT the Go bool zero false). See aiprovider parity issue #42.
//
// Temperature arrives as a JSON number from the admin UI; json.Number keeps it
// lossless so we format it back to the same textual decimal Node stores.
type createBody struct {
	Name        *string      `json:"name"`
	Type        *string      `json:"type"`
	EndpointURL *string      `json:"endpoint_url"`
	APIKey      *string      `json:"api_key"`
	Model       *string      `json:"model"`
	Temperature *json.Number `json:"temperature"`
	IsDefault   *bool        `json:"is_default"`
	IsActive    *bool        `json:"is_active"`
}

// patchBody decodes the PATCH body — every field optional (nil = unchanged).
type patchBody struct {
	Name        *string      `json:"name"`
	Type        *string      `json:"type"`
	EndpointURL *string      `json:"endpoint_url"`
	APIKey      *string      `json:"api_key"`
	Model       *string      `json:"model"`
	Temperature *json.Number `json:"temperature"`
	IsDefault   *bool        `json:"is_default"`
	IsActive    *bool        `json:"is_active"`
}

// List handles GET /admin/ai-providers → 200 [ … ]. Empty serializes as [].
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	providers, err := h.svc.List(r.Context())
	if err != nil {
		shared.WriteError(w, r, "internal", "ai providers failed")
		return
	}
	if providers == nil {
		providers = []AiProvider{}
	}
	shared.WriteJSON(w, http.StatusOK, providers)
}

// Paginated handles GET /admin/ai-providers/paginated → 200 { rows, total }.
func (h *Handler) Paginated(w http.ResponseWriter, r *http.Request) {
	q := listquery.Parse(r.URL.Query())
	b := listquery.Build(q, aiSearchCols, aiSortAllow, "created_at", "is_active")
	rows, total, err := h.svc.ListPaginated(r.Context(), b)
	if err != nil {
		shared.WriteError(w, r, "internal", "ai providers failed")
		return
	}
	if rows == nil {
		rows = []AiProvider{}
	}
	shared.WriteJSON(w, http.StatusOK, paginatedResponse{Rows: rows, Total: total})
}

// Create handles POST /admin/ai-providers → 201. Node's @Post() has no
// @HttpCode override, so the success status is 201.
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	var body createBody
	dec := json.NewDecoder(r.Body)
	// Node's create body is NOT a class-validator DTO (it's an inline
	// `@Body() body: {...}` typed param — no ValidationPipe whitelist), so
	// unknown fields are silently ignored. Keep the decode lenient (no
	// DisallowUnknownFields) for parity; only a malformed body 400s.
	dec.UseNumber()
	if err := dec.Decode(&body); err != nil {
		shared.WriteError(w, r, "invalid-input", "Invalid request body")
		return
	}

	temperature := "0.3"
	if body.Temperature != nil {
		temperature = body.Temperature.String()
	}

	in := NewProvider{
		Name:        derefStr(body.Name),
		Type:        derefStr(body.Type),
		EndpointURL: derefStr(body.EndpointURL),
		APIKey:      derefStr(body.APIKey),
		Model:       derefStr(body.Model),
		Temperature: temperature,
		IsDefault:   derefBool(body.IsDefault),
		IsActive:    derefBoolOr(body.IsActive, true),
	}

	p, err := h.svc.Create(r.Context(), in)
	if err != nil {
		shared.WriteError(w, r, "internal", "ai providers failed")
		return
	}
	shared.WriteJSON(w, http.StatusCreated, p)
}

// Update handles PATCH /admin/ai-providers/:id → 200. Missing keys = nil =
// column untouched (TypeORM partial update parity).
func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var body patchBody
	dec := json.NewDecoder(r.Body)
	dec.UseNumber()
	if err := dec.Decode(&body); err != nil {
		shared.WriteError(w, r, "invalid-input", "Invalid request body")
		return
	}

	patch := ProviderPatch{
		Name:        body.Name,
		Type:        body.Type,
		EndpointURL: body.EndpointURL,
		APIKey:      body.APIKey,
		Model:       body.Model,
		IsDefault:   body.IsDefault,
		IsActive:    body.IsActive,
	}
	if body.Temperature != nil {
		t := body.Temperature.String()
		patch.Temperature = &t
	}

	p, err := h.svc.Update(r.Context(), id, patch)
	if err != nil {
		shared.WriteError(w, r, "internal", "ai providers failed")
		return
	}
	shared.WriteJSON(w, http.StatusOK, p)
}

// Delete handles DELETE /admin/ai-providers/:id → 200 { success:true }.
func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.svc.SoftDelete(r.Context(), id); err != nil {
		shared.WriteError(w, r, "internal", "ai providers failed")
		return
	}
	shared.WriteJSON(w, http.StatusOK, struct {
		Success bool `json:"success"`
	}{true})
}

// Test handles POST /admin/ai-providers/:id/test → 201. Node's
// @Post('ai-providers/:id/test') has no @HttpCode override, so the success
// status is 201 (NOT 200). See aiprovider parity issue #43. Body shape is
// { success, response, response_time_ms } on success; { success:false, error }
// on failure / not-found.
func (h *Handler) Test(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	res := h.svc.Test(r.Context(), id)
	shared.WriteJSON(w, http.StatusCreated, res)
}

func derefStr(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

func derefBool(p *bool) bool {
	if p == nil {
		return false
	}
	return *p
}

// derefBoolOr returns *p, or def when p is nil — for fields whose absent-key
// default is not the bool zero value (e.g. is_active defaults to true).
func derefBoolOr(p *bool, def bool) bool {
	if p == nil {
		return def
	}
	return *p
}
