package errreport

import (
	"strings"
	"testing"
)

func TestScrub(t *testing.T) {
	in := "auth Bearer abc.def-123 failed password=hunter2 for joe@x.com"
	got := Scrub(in)
	if strings.Contains(got, "abc.def") || strings.Contains(got, "hunter2") || strings.Contains(got, "joe@x.com") {
		t.Fatalf("scrub leaked: %q", got)
	}
	if !strings.Contains(got, "Bearer [REDACTED]") || !strings.Contains(got, "[email]") {
		t.Fatalf("scrub markers missing: %q", got)
	}
}

func TestFingerprint_StableOnFirstThreeFrames(t *testing.T) {
	a := Fingerprint("TypeError", "  at f1\n at f2\n at f3\n at f4")
	b := Fingerprint("TypeError", "  at f1\n at f2\n at f3\n at DIFFERENT")
	if a != b {
		t.Fatalf("fingerprint must ignore frames past 3: %s vs %s", a, b)
	}
	if len(a) != 64 {
		t.Fatalf("fingerprint len = %d, want 64 hex", len(a))
	}
	c := Fingerprint("RangeError", "  at f1\n at f2\n at f3")
	if a == c {
		t.Fatal("different error_type must change fingerprint")
	}
}

func TestCoerceSeverityAndPlatform(t *testing.T) {
	if CoerceSeverity("nope") != "error" {
		t.Fatal("invalid severity must coerce to error")
	}
	if CoerceSeverity("fatal") != "fatal" {
		t.Fatal("valid severity preserved")
	}
	if PlatformValid("symbian") || !PlatformValid("ios") {
		t.Fatal("platform allowlist wrong")
	}
}

func TestSliceUTF16(t *testing.T) {
	if got := sliceUTF16("hello", 3); got != "hel" {
		t.Fatalf("sliceUTF16 = %q, want hel", got)
	}
	if got := sliceUTF16("hi", 10); got != "hi" {
		t.Fatalf("sliceUTF16 under-cap = %q, want hi", got)
	}
}
