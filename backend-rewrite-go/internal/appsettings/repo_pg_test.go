package appsettings

import (
	"strings"
	"testing"
)

func ptr(s string) *string { return &s }
func ptrInt(i int) *int    { return &i }

func TestPatchSQL_OnlyNonNilFields(t *testing.T) {
	set, args := patchSQL(Patch{Environment: ptr("prod"), TrialLimit: ptrInt(5)})
	if set != "environment = $1, trial_limit = $2, updated_at = now()" {
		t.Fatalf("set=%q", set)
	}
	if len(args) != 2 {
		t.Fatalf("args=%v", args)
	}
}

// Apple App Store product ids are plain, non-secret columns (mirrors the
// PayPal plan-id pattern): present in the SET clause when set, absent when
// nil.
func TestPatchSQL_AppleProductIDs(t *testing.T) {
	set, args := patchSQL(Patch{AppleProductMonthly: ptr("com.draftright.monthly")})
	if !strings.Contains(set, "apple_product_monthly = $1") {
		t.Fatalf("set=%q, want apple_product_monthly", set)
	}
	if strings.Contains(set, "apple_product_yearly") {
		t.Fatalf("set=%q, apple_product_yearly must be absent when nil", set)
	}
	if len(args) != 1 || args[0] != "com.draftright.monthly" {
		t.Fatalf("args=%v", args)
	}
}
