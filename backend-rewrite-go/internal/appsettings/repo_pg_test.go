package appsettings

import "testing"

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
