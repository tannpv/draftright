package payment

// Internal test (package payment) so it can reach the unexported pure SQL
// builder. These pin the bespoke FindAll semantics + injection-safety WITHOUT a
// database: only the emitted SQL string, the bound value args, and the
// limit/offset are asserted. Parity authority: src/payment/payment.service.ts
// findAll (line 603).

import (
	"strings"
	"testing"
)

// TestPaymentFindAllSQL_DefaultPath: no status, no search, default sort →
// ORDER BY p.created_at DESC, no WHERE, LIMIT/OFFSET placeholders present, no
// value args. LEFT JOINs always present.
func TestPaymentFindAllSQL_DefaultPath(t *testing.T) {
	rows, count, args, limit, offset := paymentFindAllSQL(FindAllParams{Page: 1, Limit: 20})

	if strings.Contains(rows, "WHERE") {
		t.Fatalf("default path should have no WHERE, got: %s", rows)
	}
	if !strings.Contains(rows, "ORDER BY p.created_at DESC") {
		t.Fatalf("want ORDER BY p.created_at DESC, got: %s", rows)
	}
	if !strings.Contains(rows, "LEFT JOIN users u ON u.id = p.user_id") ||
		!strings.Contains(rows, "LEFT JOIN plans pl ON pl.id = p.plan_id") {
		t.Fatalf("want both LEFT JOINs, got: %s", rows)
	}
	if !strings.Contains(rows, "LIMIT $%d OFFSET $%d") {
		t.Fatalf("want LIMIT/OFFSET placeholders, got: %s", rows)
	}
	if !strings.Contains(count, "LEFT JOIN users u") || !strings.Contains(count, "LEFT JOIN plans pl") {
		t.Fatalf("count must keep the joins (search references u/pl), got: %s", count)
	}
	if len(args) != 0 {
		t.Fatalf("want 0 value args, got %d: %v", len(args), args)
	}
	if limit != 20 || offset != 0 {
		t.Fatalf("want limit=20 offset=0, got limit=%d offset=%d", limit, offset)
	}
}

// TestPaymentFindAllSQL_SortAllowList: unknown sort_by → p.created_at DESC;
// amount/ASC → p.amount ASC; user.email → u.email.
func TestPaymentFindAllSQL_SortAllowList(t *testing.T) {
	rows, _, _, _, _ := paymentFindAllSQL(FindAllParams{Page: 1, Limit: 20, SortBy: "bogus"})
	if !strings.Contains(rows, "ORDER BY p.created_at DESC") {
		t.Fatalf("unknown sort_by must default to p.created_at DESC, got: %s", rows)
	}

	rows, _, _, _, _ = paymentFindAllSQL(FindAllParams{Page: 1, Limit: 20, SortBy: "amount", SortOrder: "ASC"})
	if !strings.Contains(rows, "ORDER BY p.amount ASC") {
		t.Fatalf("want ORDER BY p.amount ASC, got: %s", rows)
	}

	rows, _, _, _, _ = paymentFindAllSQL(FindAllParams{Page: 1, Limit: 20, SortBy: "user.email"})
	if !strings.Contains(rows, "ORDER BY u.email DESC") {
		t.Fatalf("want ORDER BY u.email DESC, got: %s", rows)
	}
}

// TestPaymentFindAllSQL_StatusFilter: status set → WHERE p.status = $1, arg the
// raw status string (NOT a boolean), count shares the WHERE.
func TestPaymentFindAllSQL_StatusFilter(t *testing.T) {
	rows, count, args, _, _ := paymentFindAllSQL(FindAllParams{Page: 1, Limit: 20, Status: "completed"})
	if !strings.Contains(rows, "WHERE p.status = $1") {
		t.Fatalf("want WHERE p.status = $1, got: %s", rows)
	}
	if !strings.Contains(count, "WHERE p.status = $1") {
		t.Fatalf("count must share WHERE, got: %s", count)
	}
	if len(args) != 1 || args[0] != "completed" {
		t.Fatalf("want [completed] (string), got %v", args)
	}
}

// TestPaymentFindAllSQL_Search: non-empty (trimmed) search → the 5-col ILIKE
// all referencing the same $1, bound to %term% (trimmed).
func TestPaymentFindAllSQL_Search(t *testing.T) {
	rows, _, args, _, _ := paymentFindAllSQL(FindAllParams{Page: 1, Limit: 20, Search: "  bob  "})
	want := "(p.reference_code ILIKE $1 OR u.email ILIKE $1 OR u.name ILIKE $1 " +
		"OR pl.name ILIKE $1 OR p.method ILIKE $1)"
	if !strings.Contains(rows, want) {
		t.Fatalf("want 5-col ILIKE on $1, got: %s", rows)
	}
	if len(args) != 1 || args[0] != "%bob%" {
		t.Fatalf("want [%%bob%%] (trimmed), got %v", args)
	}
}

// TestPaymentFindAllSQL_WhitespaceSearchIgnored: a whitespace-only search adds
// no predicate (Node search?.trim() guard).
func TestPaymentFindAllSQL_WhitespaceSearchIgnored(t *testing.T) {
	rows, _, args, _, _ := paymentFindAllSQL(FindAllParams{Page: 1, Limit: 20, Search: "   "})
	if strings.Contains(rows, "ILIKE") {
		t.Fatalf("whitespace-only search must add no ILIKE, got: %s", rows)
	}
	if len(args) != 0 {
		t.Fatalf("want 0 args, got %v", args)
	}
}

// TestPaymentFindAllSQL_StatusAndSearch: both → p.status = $1 AND ILIKE on $2,
// args [status, %term%].
func TestPaymentFindAllSQL_StatusAndSearch(t *testing.T) {
	rows, _, args, _, _ := paymentFindAllSQL(FindAllParams{
		Page: 1, Limit: 20, Status: "pending", Search: "x", SortBy: "method", SortOrder: "ASC",
	})
	if !strings.Contains(rows, "WHERE p.status = $1 AND (p.reference_code ILIKE $2") {
		t.Fatalf("unexpected WHERE: %s", rows)
	}
	if !strings.Contains(rows, "ORDER BY p.method ASC") {
		t.Fatalf("want ORDER BY p.method ASC, got: %s", rows)
	}
	if len(args) != 2 || args[0] != "pending" || args[1] != "%x%" {
		t.Fatalf("want [pending, %%x%%], got %v", args)
	}
}

// TestPaymentFindAllSQL_NoLimitClamp: Limit:500 passes through unclamped;
// offset = (page-1)*limit.
func TestPaymentFindAllSQL_NoLimitClamp(t *testing.T) {
	_, _, _, limit, offset := paymentFindAllSQL(FindAllParams{Page: 3, Limit: 500})
	if limit != 500 {
		t.Fatalf("limit must pass through unclamped, got %d", limit)
	}
	if offset != 1000 { // (3-1)*500
		t.Fatalf("want offset 1000, got %d", offset)
	}
}

// TestPaymentFindAllSQL_PageDefault: page < 1 defaults to 1 → offset 0.
func TestPaymentFindAllSQL_PageDefault(t *testing.T) {
	_, _, _, _, offset := paymentFindAllSQL(FindAllParams{Page: 0, Limit: 20})
	if offset != 0 {
		t.Fatalf("page<1 must default to 1 → offset 0, got %d", offset)
	}
}

// TestResolvePaymentSortCol_AllowList: each allow-listed key maps to its
// qualified column; anything else falls back to p.created_at.
func TestResolvePaymentSortCol_AllowList(t *testing.T) {
	want := map[string]string{
		"reference_code": "p.reference_code",
		"amount":         "p.amount",
		"method":         "p.method",
		"status":         "p.status",
		"created_at":     "p.created_at",
		"user.email":     "u.email",
	}
	for in, exp := range want {
		if got := resolvePaymentSortCol(in); got != exp {
			t.Fatalf("resolvePaymentSortCol(%q) = %q, want %q", in, got, exp)
		}
	}
	for _, bad := range []string{"", "unknown", "password_hash", "id", "user_id"} {
		if got := resolvePaymentSortCol(bad); got != "p.created_at" {
			t.Fatalf("resolvePaymentSortCol(%q) = %q, want p.created_at", bad, got)
		}
	}
}

// TestResolvePaymentSortOrder: ASC iff exactly "ASC", else DESC.
func TestResolvePaymentSortOrder(t *testing.T) {
	if resolvePaymentSortOrder("ASC") != "ASC" {
		t.Fatal("ASC must stay ASC")
	}
	for _, in := range []string{"asc", "DESC", "desc", "", "DROP"} {
		if got := resolvePaymentSortOrder(in); got != "DESC" {
			t.Fatalf("resolvePaymentSortOrder(%q) = %q, want DESC", in, got)
		}
	}
}

// TestPaymentFindAllSQL_InjectionSafe: arbitrary sort_by / sort_order must
// NEVER reach the emitted SQL — only allow-listed column literals + a hardcoded
// ASC/DESC direction appear. The status value still binds as a $N arg (so the
// quote in it can't break out).
func TestPaymentFindAllSQL_InjectionSafe(t *testing.T) {
	rows, count, args, _, _ := paymentFindAllSQL(FindAllParams{
		Page: 1, Limit: 20,
		Status:    "completed'; DROP TABLE payments; --",
		Search:    "x'; --",
		SortBy:    "p.created_at; DROP TABLE payments; --",
		SortOrder: "ASC; DELETE FROM payments",
	})
	for _, sql := range []string{rows, count} {
		if strings.Contains(sql, "DROP") || strings.Contains(sql, "DELETE") {
			t.Fatalf("injection leaked into SQL: %s", sql)
		}
		if strings.Contains(sql, "--") {
			t.Fatalf("comment injection leaked into SQL: %s", sql)
		}
	}
	if !strings.Contains(rows, "ORDER BY p.created_at DESC") {
		t.Fatalf("want safe ORDER BY p.created_at DESC, got: %s", rows)
	}
	// The malicious status binds as a value arg, not inlined.
	if len(args) != 2 || args[0] != "completed'; DROP TABLE payments; --" {
		t.Fatalf("status must bind as a value arg, got %v", args)
	}
}
