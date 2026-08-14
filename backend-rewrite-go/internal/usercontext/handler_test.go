package usercontext

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/tannpv/draftright-rewrite/internal/platform/auth"
	"github.com/tannpv/draftright-rewrite/internal/shared"
	"github.com/tannpv/draftright-rewrite/internal/shared/pg/sqlc"
)

const uid = "11111111-1111-1111-1111-111111111111"

// crudFake implements crudQuerier in memory.
type crudFake struct {
	row     sqlc.GetUserContextRow
	hasRow  bool
	upErr   error
	getErr  error
	deleted bool
	lastUp  sqlc.UpsertUserContextParams
}

func (f *crudFake) GetUserContext(_ context.Context, _ pgtype.UUID) (sqlc.GetUserContextRow, error) {
	if f.getErr != nil {
		return sqlc.GetUserContextRow{}, f.getErr
	}
	if !f.hasRow {
		return sqlc.GetUserContextRow{}, pgx.ErrNoRows
	}
	return f.row, nil
}

func (f *crudFake) UpsertUserContext(_ context.Context, arg sqlc.UpsertUserContextParams) (sqlc.UpsertUserContextRow, error) {
	f.lastUp = arg
	if f.upErr != nil {
		return sqlc.UpsertUserContextRow{}, f.upErr
	}
	return sqlc.UpsertUserContextRow{
		Enabled: arg.Enabled, JobTitle: arg.JobTitle, Industry: arg.Industry,
		Audience: arg.Audience, StyleNotes: arg.StyleNotes,
	}, nil
}

func (f *crudFake) DeleteUserContext(_ context.Context, _ pgtype.UUID) error {
	f.deleted = true
	return nil
}

func authed(method, body, sub string) *http.Request {
	r := httptest.NewRequest(method, "/me/context", strings.NewReader(body))
	return r.WithContext(shared.ContextWithClaims(r.Context(), &auth.Claims{Sub: sub}))
}

func newHandler(f *crudFake) *Handler {
	return NewHandler(f, slog.New(slog.NewTextHandler(io.Discard, nil)))
}

func TestGet_NoRowReturnsDefaults(t *testing.T) {
	w := httptest.NewRecorder()
	newHandler(&crudFake{}).Get(w, authed(http.MethodGet, "", uid))
	if w.Code != http.StatusOK {
		t.Fatalf("code=%d", w.Code)
	}
	var v view
	_ = json.Unmarshal(w.Body.Bytes(), &v)
	if v.Enabled || v.JobTitle != "" {
		t.Fatalf("want empty defaults, got %+v", v)
	}
}

func TestPut_SavesAndEncryptsNotes(t *testing.T) {
	// With a key present, style_notes must be encrypted at rest (#50).
	t.Setenv("SECRETS_ENCRYPTION_KEY", "BwcHBwcHBwcHBwcHBwcHBwcHBwcHBwcHBwcHBwcHBwc=")
	f := &crudFake{}
	w := httptest.NewRecorder()
	newHandler(f).Put(w, authed(http.MethodPut, `{"enabled":true,"job_title":"Lawyer","style_notes":"formal"}`, uid))
	if w.Code != http.StatusOK {
		t.Fatalf("code=%d body=%s", w.Code, w.Body)
	}
	// style_notes stored encrypted (enc:v1:) — never plaintext
	if f.lastUp.StyleNotes == "formal" || !strings.HasPrefix(f.lastUp.StyleNotes, "enc:v1:") {
		t.Fatalf("style_notes not encrypted at rest: %q", f.lastUp.StyleNotes)
	}
	// response echoes plaintext to the caller
	var v view
	_ = json.Unmarshal(w.Body.Bytes(), &v)
	if v.StyleNotes != "formal" || !v.Enabled || v.JobTitle != "Lawyer" {
		t.Fatalf("bad view: %+v", v)
	}
}

func TestPut_PartialKeepsExistingFields(t *testing.T) {
	f := &crudFake{hasRow: true, row: sqlc.GetUserContextRow{Enabled: true, JobTitle: "Lawyer"}}
	w := httptest.NewRecorder()
	// patch industry only — job_title must survive
	newHandler(f).Put(w, authed(http.MethodPut, `{"industry":"finance"}`, uid))
	if w.Code != http.StatusOK {
		t.Fatalf("code=%d", w.Code)
	}
	if f.lastUp.JobTitle != "Lawyer" || f.lastUp.Industry != "finance" {
		t.Fatalf("partial merge lost a field: %+v", f.lastUp)
	}
}

func TestPut_RejectsTooLongField(t *testing.T) {
	w := httptest.NewRecorder()
	long := strings.Repeat("x", fieldMaxLen+1)
	newHandler(&crudFake{}).Put(w, authed(http.MethodPut, `{"job_title":"`+long+`"}`, uid))
	if w.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d", w.Code)
	}
}

func TestDelete_RemovesRow(t *testing.T) {
	f := &crudFake{}
	w := httptest.NewRecorder()
	newHandler(f).Delete(w, authed(http.MethodDelete, "", uid))
	if w.Code != http.StatusNoContent || !f.deleted {
		t.Fatalf("code=%d deleted=%v", w.Code, f.deleted)
	}
}

func TestGet_NoClaimsUnauthorized(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/me/context", nil) // no claims
	newHandler(&crudFake{}).Get(w, r)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("want 401, got %d", w.Code)
	}
}
