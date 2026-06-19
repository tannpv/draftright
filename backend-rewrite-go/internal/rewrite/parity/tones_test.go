package parity

import (
	"encoding/json"
	"reflect"
	"testing"
)

// TestTones_MarshalBytes locks the exact marshaled JSON — key order
// (id,label,icon,kind), entry order, and the literal UTF-8 emoji/icons.
// Mirrors Node's GET /rewrite/tones which serves { tones: TONES }.
func TestTones_MarshalBytes(t *testing.T) {
	b, err := json.Marshal(map[string]any{"tones": Tones})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	want := `{"tones":[` +
		`{"id":"simple","label":"Simple","icon":"✎","kind":"rewrite"},` +
		`{"id":"natural","label":"Natural","icon":"💬","kind":"rewrite"},` +
		`{"id":"polished","label":"Polished","icon":"✨","kind":"rewrite"},` +
		`{"id":"concise","label":"Concise","icon":"⊖","kind":"rewrite"},` +
		`{"id":"technical","label":"Technical","icon":"🔧","kind":"rewrite"},` +
		`{"id":"claude","label":"Claude","icon":"✦","kind":"rewrite"},` +
		`{"id":"grammar_check","label":"Grammar Check","icon":"✓","kind":"grammar"},` +
		`{"id":"translate","label":"Translate","icon":"🌐","kind":"translate"}` +
		`]}`
	if got := string(b); got != want {
		t.Errorf("marshaled tones mismatch\n got: %s\nwant: %s", got, want)
	}
}

func TestToneIDs(t *testing.T) {
	want := []string{"simple", "natural", "polished", "concise", "technical", "claude", "grammar_check", "translate"}
	if !reflect.DeepEqual(ToneIDs, want) {
		t.Errorf("ToneIDs mismatch\n got: %v\nwant: %v", ToneIDs, want)
	}
}
