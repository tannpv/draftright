package usercontext

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/tannpv/draftright-rewrite/internal/shared/pg/sqlc"
)

type fakeQuerier struct {
	row sqlc.GetUserContextRow
	err error
}

func (f *fakeQuerier) GetUserContext(_ context.Context, _ pgtype.UUID) (sqlc.GetUserContextRow, error) {
	return f.row, f.err
}

const validUUID = "11111111-1111-1111-1111-111111111111"

func TestPreamble_BuildsFromEnabledProfile(t *testing.T) {
	p := NewProvider(&fakeQuerier{row: sqlc.GetUserContextRow{
		Enabled: true, JobTitle: "Lawyer",
	}})
	got := p.Preamble(context.Background(), validUUID)
	if !strings.Contains(got, "Lawyer") || !strings.Contains(got, "do not mention it") {
		t.Fatalf("expected personalized preamble, got %q", got)
	}
}

func TestPreamble_DisabledReturnsEmpty(t *testing.T) {
	p := NewProvider(&fakeQuerier{row: sqlc.GetUserContextRow{Enabled: false, JobTitle: "Lawyer"}})
	if got := p.Preamble(context.Background(), validUUID); got != "" {
		t.Fatalf("disabled must yield empty, got %q", got)
	}
}

func TestPreamble_NoRowReturnsEmpty(t *testing.T) {
	p := NewProvider(&fakeQuerier{err: pgx.ErrNoRows})
	if got := p.Preamble(context.Background(), validUUID); got != "" {
		t.Fatalf("no row must yield empty, got %q", got)
	}
}

func TestPreamble_DBErrorDegradesToEmpty(t *testing.T) {
	p := NewProvider(&fakeQuerier{err: errors.New("db down")})
	if got := p.Preamble(context.Background(), validUUID); got != "" {
		t.Fatalf("db error must yield empty (never break rewrite), got %q", got)
	}
}

func TestPreamble_InvalidUserIDReturnsEmpty(t *testing.T) {
	p := NewProvider(&fakeQuerier{row: sqlc.GetUserContextRow{Enabled: true, JobTitle: "X"}})
	if got := p.Preamble(context.Background(), "not-a-uuid"); got != "" {
		t.Fatalf("invalid userID must yield empty, got %q", got)
	}
}
