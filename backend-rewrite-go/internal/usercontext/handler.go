package usercontext

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/tannpv/draftright-rewrite/internal/platform/secretcipher"
	"github.com/tannpv/draftright-rewrite/internal/shared"
	"github.com/tannpv/draftright-rewrite/internal/shared/pg/sqlc"
)

// Storage-side length limits — mirror the Node DTO
// (backend/src/user-context/user-context.constants.ts). Counted in runes
// (close to Node's JS string length for BMP text); the Node DTO is the
// co-authority since both backends validate.
const (
	fieldMaxLen = 120
	notesMaxLen = 1000
)

// crudQuerier is the consumer-side port — the three sqlc queries this handler
// needs, so tests can fake it without a database.
type crudQuerier interface {
	GetUserContext(ctx context.Context, userID pgtype.UUID) (sqlc.GetUserContextRow, error)
	UpsertUserContext(ctx context.Context, arg sqlc.UpsertUserContextParams) (sqlc.UpsertUserContextRow, error)
	DeleteUserContext(ctx context.Context, userID pgtype.UUID) error
}

// Handler serves GET/PUT/DELETE /me/context. Mounted inside the router's
// RequireAuth group, so every request already carries verified JWT claims;
// the userID is taken from the token, never the body — a caller can only ever
// touch their own row.
type Handler struct {
	q   crudQuerier
	log *slog.Logger
}

func NewHandler(q crudQuerier, log *slog.Logger) *Handler {
	return &Handler{q: q, log: log}
}

// view is the client-facing shape — snake_case to match the Node response the
// web client already consumes. Never exposes server-only columns.
type view struct {
	Enabled    bool   `json:"enabled"`
	JobTitle   string `json:"job_title"`
	Industry   string `json:"industry"`
	Audience   string `json:"audience"`
	StyleNotes string `json:"style_notes"`
}

// updateBody uses pointers so an absent field is distinguishable from "" —
// only provided fields change (partial PUT, mirroring Node's upsert).
type updateBody struct {
	Enabled    *bool   `json:"enabled"`
	JobTitle   *string `json:"job_title"`
	Industry   *string `json:"industry"`
	Audience   *string `json:"audience"`
	StyleNotes *string `json:"style_notes"`
}

func (h *Handler) userID(r *http.Request) (pgtype.UUID, bool) {
	claims, ok := shared.ClaimsFromContext(r.Context())
	if !ok {
		return pgtype.UUID{}, false
	}
	uid, err := uuid.Parse(claims.UserID())
	if err != nil {
		return pgtype.UUID{}, false
	}
	return pgtype.UUID{Bytes: uid, Valid: true}, true
}

// Get returns the caller's profile (decrypted), or empty defaults if none.
func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	uid, ok := h.userID(r)
	if !ok {
		shared.WriteJSON(w, http.StatusUnauthorized, map[string]string{"error": "Unauthorized"})
		return
	}
	row, err := h.q.GetUserContext(r.Context(), uid)
	if errors.Is(err, pgx.ErrNoRows) {
		shared.WriteJSON(w, http.StatusOK, view{})
		return
	}
	if err != nil {
		h.log.Warn("get user context", "err", err)
		shared.WriteJSON(w, http.StatusInternalServerError, map[string]string{"error": "Internal error"})
		return
	}
	notes, err := secretcipher.Decrypt(row.StyleNotes)
	if err != nil {
		h.log.Warn("decrypt style_notes", "err", err)
		notes = ""
	}
	shared.WriteJSON(w, http.StatusOK, view{
		Enabled: row.Enabled, JobTitle: row.JobTitle, Industry: row.Industry,
		Audience: row.Audience, StyleNotes: notes,
	})
}

// Put create-or-updates the caller's context; only provided fields change.
func (h *Handler) Put(w http.ResponseWriter, r *http.Request) {
	uid, ok := h.userID(r)
	if !ok {
		shared.WriteJSON(w, http.StatusUnauthorized, map[string]string{"error": "Unauthorized"})
		return
	}
	var body updateBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		shared.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": "Invalid body"})
		return
	}

	// Start from the existing row (decrypted) so unset fields survive.
	cur := view{}
	if row, err := h.q.GetUserContext(r.Context(), uid); err == nil {
		notes, _ := secretcipher.Decrypt(row.StyleNotes)
		cur = view{Enabled: row.Enabled, JobTitle: row.JobTitle, Industry: row.Industry, Audience: row.Audience, StyleNotes: notes}
	} else if !errors.Is(err, pgx.ErrNoRows) {
		h.log.Warn("load user context", "err", err)
		shared.WriteJSON(w, http.StatusInternalServerError, map[string]string{"error": "Internal error"})
		return
	}

	if body.Enabled != nil {
		cur.Enabled = *body.Enabled
	}
	if body.JobTitle != nil {
		cur.JobTitle = *body.JobTitle
	}
	if body.Industry != nil {
		cur.Industry = *body.Industry
	}
	if body.Audience != nil {
		cur.Audience = *body.Audience
	}
	if body.StyleNotes != nil {
		cur.StyleNotes = *body.StyleNotes
	}

	if msg := validate(cur); msg != "" {
		shared.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": msg})
		return
	}

	// Encrypt the free-text notes for storage (#50); structured fields stay plaintext.
	encNotes, err := secretcipher.Encrypt(cur.StyleNotes)
	if err != nil {
		h.log.Warn("encrypt style_notes", "err", err)
		shared.WriteJSON(w, http.StatusInternalServerError, map[string]string{"error": "Internal error"})
		return
	}
	saved, err := h.q.UpsertUserContext(r.Context(), sqlc.UpsertUserContextParams{
		UserID: uid, Enabled: cur.Enabled, JobTitle: cur.JobTitle,
		Industry: cur.Industry, Audience: cur.Audience, StyleNotes: encNotes,
	})
	if err != nil {
		h.log.Warn("upsert user context", "err", err)
		shared.WriteJSON(w, http.StatusInternalServerError, map[string]string{"error": "Internal error"})
		return
	}
	// Echo the plaintext the caller set (saved.StyleNotes is the ciphertext).
	shared.WriteJSON(w, http.StatusOK, view{
		Enabled: saved.Enabled, JobTitle: saved.JobTitle, Industry: saved.Industry,
		Audience: saved.Audience, StyleNotes: cur.StyleNotes,
	})
}

// Delete removes the caller's context row (GDPR erasure).
func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	uid, ok := h.userID(r)
	if !ok {
		shared.WriteJSON(w, http.StatusUnauthorized, map[string]string{"error": "Unauthorized"})
		return
	}
	if err := h.q.DeleteUserContext(r.Context(), uid); err != nil {
		h.log.Warn("delete user context", "err", err)
		shared.WriteJSON(w, http.StatusInternalServerError, map[string]string{"error": "Internal error"})
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func validate(v view) string {
	if utf8.RuneCountInString(v.JobTitle) > fieldMaxLen ||
		utf8.RuneCountInString(v.Industry) > fieldMaxLen ||
		utf8.RuneCountInString(v.Audience) > fieldMaxLen {
		return "profile fields must be at most 120 characters"
	}
	if utf8.RuneCountInString(v.StyleNotes) > notesMaxLen {
		return "style_notes must be at most 1000 characters"
	}
	return ""
}
