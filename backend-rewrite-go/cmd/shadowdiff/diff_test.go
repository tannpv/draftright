package main

import "testing"

func TestDiffJSON_IgnoresRequestIDValue(t *testing.T) {
	node := []byte(`{"error":"x","code":"invalid-input","request_id":"aaaa"}`)
	goB := []byte(`{"error":"x","code":"invalid-input","request_id":"bbbb"}`)
	diffs := diffJSON(node, goB, []string{"request_id"})
	if len(diffs) != 0 {
		t.Fatalf("request_id value diff should be ignored, got %v", diffs)
	}
}

func TestDiffJSON_FlagsRealMismatch(t *testing.T) {
	node := []byte(`{"role":"user"}`)
	goB := []byte(`{"role":"admin"}`)
	diffs := diffJSON(node, goB, nil)
	if len(diffs) == 0 {
		t.Fatal("role mismatch must be reported")
	}
}

func TestDiffJSON_RequestIDMustBePresentInBoth(t *testing.T) {
	node := []byte(`{"request_id":"aaaa"}`)
	goB := []byte(`{}`)
	diffs := diffJSON(node, goB, []string{"request_id"})
	if len(diffs) == 0 {
		t.Fatal("missing request_id on one side must be reported even when value is ignored")
	}
}

// --- top-level arrays (GET /plans, GET /auth/extension-tokens) ---

func TestDiffJSON_EqualArrays(t *testing.T) {
	node := []byte(`[{"id":1,"name":"Free"},{"id":2,"name":"Pro"}]`)
	goB := []byte(`[{"id":1,"name":"Free"},{"id":2,"name":"Pro"}]`)
	if diffs := diffJSON(node, goB, nil); len(diffs) != 0 {
		t.Fatalf("identical arrays must be parity, got %v", diffs)
	}
}

func TestDiffJSON_ArrayLengthMismatch(t *testing.T) {
	node := []byte(`[{"name":"Free"}]`)
	goB := []byte(`[{"name":"Free"},{"name":"Pro"}]`)
	if diffs := diffJSON(node, goB, nil); len(diffs) == 0 {
		t.Fatal("array length mismatch must be reported")
	}
}

func TestDiffJSON_ArrayElementFieldMismatch(t *testing.T) {
	node := []byte(`[{"name":"Free","price_cents":0}]`)
	goB := []byte(`[{"name":"Free","price_cents":900}]`)
	if diffs := diffJSON(node, goB, nil); len(diffs) == 0 {
		t.Fatal("element field mismatch must be reported")
	}
}

func TestDiffJSON_IgnoresKeyInsideArrayElements(t *testing.T) {
	// id + last_used_at differ per row but must be present+non-empty on both.
	node := []byte(`[{"id":"a1","device_name":"phone","last_used_at":"2026-06-14T00:00:00.000Z"}]`)
	goB := []byte(`[{"id":"b9","device_name":"phone","last_used_at":"2026-06-14T09:00:00.000Z"}]`)
	if diffs := diffJSON(node, goB, []string{"id", "last_used_at"}); len(diffs) != 0 {
		t.Fatalf("ignored keys inside array elements must not diff, got %v", diffs)
	}
}

func TestDiffJSON_RootTypeMismatch(t *testing.T) {
	node := []byte(`[{"name":"Free"}]`)
	goB := []byte(`{"name":"Free"}`)
	if diffs := diffJSON(node, goB, nil); len(diffs) == 0 {
		t.Fatal("array-vs-object at root must be reported")
	}
}

// --- empty / 204 No Content bodies (DELETE /auth/extension-tokens/{id}) ---

func TestDiffJSON_BothEmptyIsParity(t *testing.T) {
	if diffs := diffJSON([]byte(""), []byte("  \n"), nil); len(diffs) != 0 {
		t.Fatalf("two empty bodies must be parity, got %v", diffs)
	}
}

func TestDiffJSON_EmptyVsNonEmptyIsDiff(t *testing.T) {
	if diffs := diffJSON([]byte(""), []byte(`{"deleted":true}`), nil); len(diffs) == 0 {
		t.Fatal("empty-vs-nonempty body must be reported")
	}
}

// --- nested object key ignore (subscription.nudge, verify-receipt.subscription) ---

func TestDiffJSON_IgnoresNestedKey(t *testing.T) {
	node := []byte(`{"plan":{"name":"Pro"},"nudge":{"tier":"pro","expiresAt":"2026-07-14T00:00:00.000Z"}}`)
	goB := []byte(`{"plan":{"name":"Pro"},"nudge":{"tier":"pro","expiresAt":"2026-07-14T09:00:00.000Z"}}`)
	if diffs := diffJSON(node, goB, []string{"expiresAt"}); len(diffs) != 0 {
		t.Fatalf("ignored nested key must not diff, got %v", diffs)
	}
}

func TestDiffJSON_FlagsNestedRealMismatch(t *testing.T) {
	node := []byte(`{"nudge":{"banner":"pro_expiring"}}`)
	goB := []byte(`{"nudge":{"banner":"just_expired"}}`)
	if diffs := diffJSON(node, goB, nil); len(diffs) == 0 {
		t.Fatal("nested banner mismatch must be reported")
	}
}
