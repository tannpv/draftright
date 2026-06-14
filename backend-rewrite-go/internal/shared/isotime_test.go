package shared

import (
	"testing"
	"time"
)

func TestISOMillis(t *testing.T) {
	tm := time.Date(2026, 6, 14, 9, 30, 0, 0, time.UTC)
	if got := ISOMillis(tm); got != "2026-06-14T09:30:00.000Z" {
		t.Fatalf("got %q", got)
	}
	loc := time.FixedZone("x", 3600)
	tm2 := time.Date(2026, 6, 14, 10, 30, 0, 0, loc)
	if got := ISOMillis(tm2); got != "2026-06-14T09:30:00.000Z" {
		t.Fatalf("tz convert got %q", got)
	}
	tm3 := time.Date(2026, 6, 14, 9, 30, 0, 123_456_789, time.UTC)
	if got := ISOMillis(tm3); got != "2026-06-14T09:30:00.123Z" {
		t.Fatalf("ms got %q", got)
	}
}
