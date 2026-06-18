package aiprovider

import "testing"

func TestPaginatedSQL_UsesListqueryBuilt(t *testing.T) {
	where := "WHERE (name ILIKE $1)"
	order := "ORDER BY created_at DESC"
	got := paginatedSQL(where, order)
	want := "SELECT id, name, type, endpoint_url, api_key, model, temperature::text AS temperature, " +
		"is_default, is_active, created_at, updated_at FROM ai_providers " +
		"WHERE (name ILIKE $1) ORDER BY created_at DESC LIMIT $%d OFFSET $%d"
	if got != want {
		t.Fatalf("paginatedSQL mismatch:\n got=%q\nwant=%q", got, want)
	}
}
