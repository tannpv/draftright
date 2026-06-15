package updates

import (
	"context"
	"net/http"

	"github.com/tannpv/draftright-rewrite/internal/shared"
)

// listService is the handler's consumer-side port (Service satisfies it).
type listService interface {
	listEffective(ctx context.Context, override string) (map[string]*Release, error)
}

// Handler serves GET /updates/latest. Public, HTTP 200.
type Handler struct{ svc listService }

// NewHandler wires the service.
func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

// platformEntry is one value in the `platforms` map. Field order + names
// match the Node controller exactly.
type platformEntry struct {
	Version   string `json:"version"`
	URL       string `json:"url"`
	SHA256    string `json:"sha256"`
	Notes     string `json:"notes"`
	Required  bool   `json:"required"`
	Channel   string `json:"channel"`
	UpdatedAt string `json:"updated_at"`
}

// Latest builds the full update manifest. Never 404s.
func (h *Handler) Latest(w http.ResponseWriter, r *http.Request) {
	channel := r.URL.Query().Get("channel")
	platform := r.URL.Query().Get("platform")

	all, err := h.svc.listEffective(r.Context(), channel)
	if err != nil {
		shared.WriteError(w, r, "internal", err.Error())
		return
	}
	var anchor *Release
	if IsPlatform(platform) {
		anchor = all[platform]
	}
	env := buildEnvelope(all, anchor)

	urlOf := func(p string) string {
		if r := all[p]; r != nil {
			return r.DownloadURL
		}
		return ""
	}
	shaOf := func(p string) string {
		if r := all[p]; r != nil {
			return r.SHA256
		}
		return ""
	}
	plats := make(map[string]platformEntry)
	for _, p := range Platforms {
		v := all[p]
		if v == nil {
			continue
		}
		plats[p] = platformEntry{
			Version: v.Version, URL: v.DownloadURL, SHA256: v.SHA256,
			Notes: v.ReleaseNotes, Required: v.Required, Channel: v.Channel,
			UpdatedAt: shared.ISOMillis(v.UpdatedAt),
		}
	}

	// Ordered top-level object. Use a struct so field order is fixed.
	resp := struct {
		Version      string                   `json:"version"`
		MacURL       string                   `json:"mac_url"`
		WindowsURL   string                   `json:"windows_url"`
		LinuxURL     string                   `json:"linux_url"`
		AndroidURL   string                   `json:"android_url"`
		IosURL       string                   `json:"ios_url"`
		MacSHA       string                   `json:"mac_sha256"`
		WindowsSHA   string                   `json:"windows_sha256"`
		LinuxSHA     string                   `json:"linux_sha256"`
		AndroidSHA   string                   `json:"android_sha256"`
		IosSHA       string                   `json:"ios_sha256"`
		ReleaseNotes string                   `json:"release_notes"`
		Required     bool                     `json:"required"`
		Platforms    map[string]platformEntry `json:"platforms"`
	}{
		Version: env.Version,
		MacURL:  urlOf("mac"), WindowsURL: urlOf("windows"), LinuxURL: urlOf("linux"), AndroidURL: urlOf("android"), IosURL: urlOf("ios"),
		MacSHA: shaOf("mac"), WindowsSHA: shaOf("windows"), LinuxSHA: shaOf("linux"), AndroidSHA: shaOf("android"), IosSHA: shaOf("ios"),
		ReleaseNotes: env.ReleaseNotes, Required: env.Required, Platforms: plats,
	}
	shared.WriteJSON(w, http.StatusOK, resp)
}
