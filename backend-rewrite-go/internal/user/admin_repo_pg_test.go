package user

// Internal test (package user) so it can reach the unexported pure SQL
// builders. These pin the bespoke-list semantics + injection-safety WITHOUT a
// database: only the emitted SQL string + the bound value args are asserted.

import (
	"strings"
	"testing"
)

// TestUserListSQL_DefaultPath: no status, no search, default sort → ORDER BY
// created_at DESC, no WHERE, LIMIT/OFFSET placeholders present, no value args.
func TestUserListSQL_DefaultPath(t *testing.T) {
	sortCol := resolveSortCol("")
	sortOrder := resolveSortOrder("")
	rows, count, args := userListSQL("", "", sortCol, sortOrder)

	if strings.Contains(rows, "WHERE") {
		t.Fatalf("default path should have no WHERE, got: %s", rows)
	}
	if !strings.Contains(rows, "ORDER BY created_at DESC") {
		t.Fatalf("want ORDER BY created_at DESC, got: %s", rows)
	}
	if !strings.Contains(rows, "LIMIT $%d OFFSET $%d") {
		t.Fatalf("want LIMIT/OFFSET placeholders, got: %s", rows)
	}
	if count != "SELECT COUNT(*) FROM users " {
		t.Fatalf("unexpected count SQL: %q", count)
	}
	if len(args) != 0 {
		t.Fatalf("want 0 value args, got %d: %v", len(args), args)
	}
}

// TestUserListSQL_StatusActive: status=active → is_active = $1 predicate bound
// to true.
func TestUserListSQL_StatusActive(t *testing.T) {
	rows, count, args := userListSQL("active", "", "created_at", "DESC")

	if !strings.Contains(rows, "WHERE is_active = $1") {
		t.Fatalf("want WHERE is_active = $1, got: %s", rows)
	}
	if !strings.Contains(count, "WHERE is_active = $1") {
		t.Fatalf("count must share WHERE, got: %s", count)
	}
	if len(args) != 1 || args[0] != true {
		t.Fatalf("want [true], got %v", args)
	}
}

// TestUserListSQL_StatusInactive: status=inactive → is_active bound to false.
func TestUserListSQL_StatusInactive(t *testing.T) {
	_, _, args := userListSQL("inactive", "", "created_at", "DESC")
	if len(args) != 1 || args[0] != false {
		t.Fatalf("want [false], got %v", args)
	}
}

// TestUserListSQL_StatusAll: status=all (and empty) → NO is_active predicate.
func TestUserListSQL_StatusAll(t *testing.T) {
	for _, s := range []string{"all", ""} {
		rows, _, args := userListSQL(s, "", "created_at", "DESC")
		// is_active appears in the SELECT projection; assert it is not a WHERE
		// predicate ("is_active = ") rather than absent entirely.
		if strings.Contains(rows, "WHERE") || strings.Contains(rows, "is_active = ") {
			t.Fatalf("status=%q must not filter is_active, got: %s", s, rows)
		}
		if len(args) != 0 {
			t.Fatalf("status=%q want 0 args, got %v", s, args)
		}
	}
}

// TestUserListSQL_Search: non-empty search → ILIKE pair both referencing the
// same $1, bound to %term%.
func TestUserListSQL_Search(t *testing.T) {
	rows, _, args := userListSQL("", "bob", "created_at", "DESC")
	if !strings.Contains(rows, "(email ILIKE $1 OR name ILIKE $1)") {
		t.Fatalf("want ILIKE pair on $1, got: %s", rows)
	}
	if len(args) != 1 || args[0] != "%bob%" {
		t.Fatalf("want [%%bob%%], got %v", args)
	}
}

// TestUserListSQL_StatusAndSearch: both predicates → is_active = $1 AND ILIKE
// on $2, args ordered [true, %x%].
func TestUserListSQL_StatusAndSearch(t *testing.T) {
	rows, _, args := userListSQL("active", "x", "email", "ASC")
	if !strings.Contains(rows, "WHERE is_active = $1 AND (email ILIKE $2 OR name ILIKE $2)") {
		t.Fatalf("unexpected WHERE: %s", rows)
	}
	if !strings.Contains(rows, "ORDER BY email ASC") {
		t.Fatalf("want ORDER BY email ASC, got: %s", rows)
	}
	if len(args) != 2 || args[0] != true || args[1] != "%x%" {
		t.Fatalf("want [true, %%x%%], got %v", args)
	}
}

// TestResolveSortCol_AllowList: each allow-listed key maps to its bare column;
// anything else falls back to created_at.
func TestResolveSortCol_AllowList(t *testing.T) {
	want := map[string]string{
		"email":      "email",
		"name":       "name",
		"role":       "role",
		"is_active":  "is_active",
		"created_at": "created_at",
	}
	for in, exp := range want {
		if got := resolveSortCol(in); got != exp {
			t.Fatalf("resolveSortCol(%q) = %q, want %q", in, got, exp)
		}
	}
	for _, bad := range []string{"", "unknown", "password_hash", "id"} {
		if got := resolveSortCol(bad); got != "created_at" {
			t.Fatalf("resolveSortCol(%q) = %q, want created_at", bad, got)
		}
	}
}

// TestResolveSortOrder: ASC iff exactly "ASC", else DESC.
func TestResolveSortOrder(t *testing.T) {
	if resolveSortOrder("ASC") != "ASC" {
		t.Fatal("ASC must stay ASC")
	}
	for _, in := range []string{"asc", "DESC", "desc", "", "DROP"} {
		if got := resolveSortOrder(in); got != "DESC" {
			t.Fatalf("resolveSortOrder(%q) = %q, want DESC", in, got)
		}
	}
}

// TestUserListSQL_InjectionSafe: arbitrary sort_by / sort_order must NEVER
// reach the emitted SQL — only allow-listed column literals + a hardcoded
// ASC/DESC direction appear.
func TestUserListSQL_InjectionSafe(t *testing.T) {
	evilBy := "created_at; DROP TABLE users; --"
	evilOrder := "ASC; DELETE FROM users"

	sortCol := resolveSortCol(evilBy)
	sortOrder := resolveSortOrder(evilOrder)
	rows, count, _ := userListSQL("active'; --", "x'; --", sortCol, sortOrder)

	for _, sql := range []string{rows, count} {
		if strings.Contains(sql, "DROP") || strings.Contains(sql, "DELETE") {
			t.Fatalf("injection leaked into SQL: %s", sql)
		}
		if strings.Contains(sql, "--") {
			t.Fatalf("comment injection leaked into SQL: %s", sql)
		}
	}
	// The evil status string is not one of active/inactive → no predicate.
	// (is_active is still in the SELECT projection; check the predicate form.)
	if strings.Contains(rows, "is_active = ") {
		t.Fatalf("non-allow-list status must not filter, got: %s", rows)
	}
	// Resolved column is the safe default; direction is the safe DESC.
	if !strings.Contains(rows, "ORDER BY created_at DESC") {
		t.Fatalf("want safe ORDER BY created_at DESC, got: %s", rows)
	}
}
