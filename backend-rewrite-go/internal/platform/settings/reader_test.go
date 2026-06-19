package settings_test

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/tannpv/draftright-rewrite/internal/platform/settings"
	sqlc "github.com/tannpv/draftright-rewrite/internal/shared/pg/sqlc"
)

type fakeQ struct {
	row sqlc.GetAuthTokenSettingsRow
	err error
}

func (f fakeQ) GetAuthTokenSettings(ctx context.Context) (sqlc.GetAuthTokenSettingsRow, error) {
	return f.row, f.err
}

func TestTTLs_FromRow(t *testing.T) {
	r := settings.NewReader(fakeQ{row: sqlc.GetAuthTokenSettingsRow{TokenExpiryMinutes: 30, RefreshTokenExpiryDays: 7}})
	a, rf, err := r.TokenTTLs(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if a != 30*time.Minute || rf != 7*24*time.Hour {
		t.Fatalf("got %v / %v", a, rf)
	}
}

func TestTTLs_DefaultsWhenNoRow(t *testing.T) {
	r := settings.NewReader(fakeQ{err: pgx.ErrNoRows})
	a, rf, err := r.TokenTTLs(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if a != 15*time.Minute || rf != 90*24*time.Hour {
		t.Fatalf("defaults wrong: %v / %v", a, rf)
	}
}
