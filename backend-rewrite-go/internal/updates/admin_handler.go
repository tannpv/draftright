// admin_handler.go — HTTP edge for the admin release/policy routes (R1–R4).
// Mounts inside the admin group (jwtMW → RequireAdmin) wired in the router
// task; the handler itself does no auth.
//
//	GET    /admin/releases                       → 200 nested {mac,windows,linux,android,ios}
//	POST   /admin/releases                       → 201 upserted AppRelease
//	DELETE /admin/releases/:platform/:channel    → 200 { ok: true }; absent → 404
//	POST   /admin/release-policies               → 201 AppReleasePolicy
//
// Node parity notes (src/admin/admin.controller.ts):
//   - POST /releases defaults `channel` to 'direct' (body.channel ?? 'direct')
//     BEFORE calling upsertChannel, mirroring the release-publish.sh script.
//   - upsertChannel / upsert validation throws BadRequestException → 400
//     invalid-input with the verbatim message.
//   - deleteChannel throws NotFoundException('No release row for {p}/{c}') when
//     0 rows matched → 404 not-found with that exact message; success returns
//     { ok: true } (the controller discards deleteChannel's void).
//   - POST routes are @Post → Nest default 201; GET → 200; DELETE → 200.
package updates

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/tannpv/draftright-rewrite/internal/shared"
)

// adminHandlerService is the handler's consumer-side port; *AdminService
// satisfies it. Kept on the consumer side (CLAUDE.md guardrail) so tests
// inject a fake without a DB.
type adminHandlerService interface {
	ListAll(ctx context.Context) (ReleasesView, error)
	UpsertChannel(ctx context.Context, in UpsertChannelInput) (AppRelease, error)
	DeleteChannel(ctx context.Context, platform, channel string) error
	UpsertPolicy(ctx context.Context, in UpsertPolicyInput) (AppReleasePolicy, error)
}

// AdminHandler serves the admin release/policy routes (R1–R4).
type AdminHandler struct {
	svc adminHandlerService
}

// NewAdminHandler wires the admin release/policy service.
func NewAdminHandler(svc *AdminService) *AdminHandler {
	return &AdminHandler{svc: svc}
}

// upsertReleaseBody is the POST /admin/releases inline body. Optional fields
// are pointers so "absent" differs from a zero value (Node's per-field
// `if (dto.x !== undefined)` merge). channel is read raw here; the handler
// defaults it to "direct" before the use case, matching the controller.
type upsertReleaseBody struct {
	Platform     string  `json:"platform"`
	Channel      *string `json:"channel"`
	Version      string  `json:"version"`
	DownloadURL  string  `json:"download_url"`
	Sha256       *string `json:"sha256"`
	ReleaseNotes *string `json:"release_notes"`
	Required     *bool   `json:"required"`
	Enabled      *bool   `json:"enabled"`
}

// upsertPolicyBody is the POST /admin/release-policies inline body.
type upsertPolicyBody struct {
	Platform    string  `json:"platform"`
	Preferred   *string `json:"preferred"`
	StoreStatus *string `json:"store_status"`
	Notes       *string `json:"notes"`
}

// okResponse is the { ok: true } DELETE body.
type okResponse struct {
	OK bool `json:"ok"`
}

// ListReleases handles GET /admin/releases → 200 nested view keyed in
// PLATFORMS order (mac, windows, linux, android, ios), each card
// { policy, channels:{ direct, store } }.
func (h *AdminHandler) ListReleases(w http.ResponseWriter, r *http.Request) {
	view, err := h.svc.ListAll(r.Context())
	if err != nil {
		shared.WriteError(w, r, "internal", err.Error())
		return
	}
	shared.WriteJSON(w, http.StatusOK, view)
}

// UpsertRelease handles POST /admin/releases → 201 upserted AppRelease.
// channel defaults to "direct" before the use case (Node controller). A
// validation failure → 400 invalid-input with the verbatim message.
func (h *AdminHandler) UpsertRelease(w http.ResponseWriter, r *http.Request) {
	var body upsertReleaseBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil && err != io.EOF {
		shared.WriteError(w, r, "invalid-input", "Invalid request body")
		return
	}
	// Node: `channel: body.channel ?? 'direct'`.
	channel := "direct"
	if body.Channel != nil {
		channel = *body.Channel
	}
	row, err := h.svc.UpsertChannel(r.Context(), UpsertChannelInput{
		Platform:     body.Platform,
		Channel:      channel,
		Version:      body.Version,
		DownloadURL:  body.DownloadURL,
		Sha256:       body.Sha256,
		ReleaseNotes: body.ReleaseNotes,
		Required:     body.Required,
		Enabled:      body.Enabled,
	})
	if err != nil {
		h.writeServiceError(w, r, err)
		return
	}
	shared.WriteJSON(w, http.StatusCreated, row)
}

// DeleteRelease handles DELETE /admin/releases/:platform/:channel → 200
// { ok: true }. 0 rows matched → 404 not-found "No release row for {p}/{c}".
func (h *AdminHandler) DeleteRelease(w http.ResponseWriter, r *http.Request) {
	platform := chi.URLParam(r, "platform")
	channel := chi.URLParam(r, "channel")
	if err := h.svc.DeleteChannel(r.Context(), platform, channel); err != nil {
		h.writeServiceError(w, r, err)
		return
	}
	shared.WriteJSON(w, http.StatusOK, okResponse{OK: true})
}

// UpsertPolicy handles POST /admin/release-policies → 201 AppReleasePolicy.
// A validation failure → 400 invalid-input with the verbatim message.
func (h *AdminHandler) UpsertPolicy(w http.ResponseWriter, r *http.Request) {
	var body upsertPolicyBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil && err != io.EOF {
		shared.WriteError(w, r, "invalid-input", "Invalid request body")
		return
	}
	row, err := h.svc.UpsertPolicy(r.Context(), UpsertPolicyInput{
		Platform:    body.Platform,
		Preferred:   body.Preferred,
		StoreStatus: body.StoreStatus,
		Notes:       body.Notes,
	})
	if err != nil {
		h.writeServiceError(w, r, err)
		return
	}
	shared.WriteJSON(w, http.StatusCreated, row)
}

// writeServiceError maps use-case errors to the canonical envelope.
// ErrReleaseNotFound is the deleteChannel NotFoundException → 404 not-found
// with the verbatim "No release row for {p}/{c}" message. Every other
// use-case error here is a validation BadRequestException → 400 invalid-input
// with its verbatim message (the use cases validate before touching the repo).
func (h *AdminHandler) writeServiceError(w http.ResponseWriter, r *http.Request, err error) {
	if errors.Is(err, ErrReleaseNotFound) {
		shared.WriteError(w, r, "not-found", err.Error())
		return
	}
	shared.WriteError(w, r, "invalid-input", err.Error())
}
