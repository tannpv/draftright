package adminauth

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func sptr(s string) *string { return &s }
func bptr(b bool) *bool     { return &b }

// adminUsersPatchSQL must walk fields in domain order and pin updated_at last,
// regardless of which subset is non-nil. name before is_active proves the
// ordering is deterministic (not map iteration) and password_hash is included
// when provided.
func TestAdminUsersPatchSQL_OnlyNonNilFields(t *testing.T) {
	set, args := adminUsersPatchSQL(AdminUserPatch{Name: sptr("Bob"), IsActive: bptr(false)})
	if set != "name = $1, is_active = $2, updated_at = now()" {
		t.Fatalf("set=%q", set)
	}
	if len(args) != 2 {
		t.Fatalf("args=%v", args)
	}
	if args[0] != "Bob" || args[1] != false {
		t.Fatalf("args=%v", args)
	}
}

// All fields set: confirms the full walk order — name, email, role, is_active,
// password_hash — with updated_at appended.
func TestAdminUsersPatchSQL_AllFieldsInOrder(t *testing.T) {
	set, args := adminUsersPatchSQL(AdminUserPatch{
		Name:         sptr("Bob"),
		Email:        sptr("bob@x.com"),
		Role:         sptr("admin"),
		IsActive:     bptr(true),
		PasswordHash: sptr("$2b$10$hash"),
	})
	want := "name = $1, email = $2, role = $3, is_active = $4, password_hash = $5, updated_at = now()"
	if set != want {
		t.Fatalf("set=%q\nwant=%q", set, want)
	}
	if len(args) != 5 {
		t.Fatalf("args=%v", args)
	}
}

// The paginated/update projection must never select password_hash — the
// secret never leaves the DB.
func TestAdminUsersSelectCols_NoSecret(t *testing.T) {
	if strings.Contains(adminUserSelectCols, "password_hash") {
		t.Fatalf("adminUserSelectCols leaks password_hash: %q", adminUserSelectCols)
	}
}

// adminUsersPaginatedSQL must project the 7 non-secret cols from admin_users,
// with the listquery WHERE/ORDER spliced in and LIMIT/OFFSET left as %d
// placeholders for positional filling.
func TestAdminUsersPaginatedSQL_UsesListqueryBuilt(t *testing.T) {
	where := "WHERE (name ILIKE $1)"
	order := "ORDER BY created_at DESC"
	got := adminUsersPaginatedSQL(where, order)
	want := "SELECT id, email, name, is_active, role, created_at, updated_at FROM admin_users " +
		"WHERE (name ILIKE $1) ORDER BY created_at DESC LIMIT $%d OFFSET $%d"
	if got != want {
		t.Fatalf("adminUsersPaginatedSQL mismatch:\n got=%q\nwant=%q", got, want)
	}
}

func TestAdminUsersCountSQL_SharesWhere(t *testing.T) {
	got := adminUsersCountSQL("WHERE (name ILIKE $1)")
	want := "SELECT COUNT(*) FROM admin_users WHERE (name ILIKE $1)"
	if got != want {
		t.Fatalf("adminUsersCountSQL mismatch:\n got=%q\nwant=%q", got, want)
	}
}

// AdminUserOut serialises exactly 7 keys, in order, with no password_hash and
// ms-precision timestamps.
func TestAdminUserOut_KeyOrderNoSecret(t *testing.T) {
	ts := time.Date(2026, 6, 18, 1, 2, 3, 456_000_000, time.UTC)
	out := AdminUserOut{
		ID: "11111111-1111-1111-1111-111111111111", Email: "a@x.com", Name: "Al",
		IsActive: true, Role: "admin", CreatedAt: ts, UpdatedAt: ts,
	}
	b, err := json.Marshal(out)
	if err != nil {
		t.Fatal(err)
	}
	got := string(b)
	want := `{"id":"11111111-1111-1111-1111-111111111111","email":"a@x.com",` +
		`"name":"Al","is_active":true,"role":"admin",` +
		`"created_at":"2026-06-18T01:02:03.456Z","updated_at":"2026-06-18T01:02:03.456Z"}`
	if got != want {
		t.Fatalf("AdminUserOut JSON mismatch:\n got=%s\nwant=%s", got, want)
	}
	if strings.Contains(got, "password_hash") {
		t.Fatalf("AdminUserOut leaks password_hash: %s", got)
	}
}
