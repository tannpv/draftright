package parity

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestWriteTones asserts GET /rewrite/tones → 200 with the full catalog under
// the top-level "tones" key, byte-identical to Node's `return { tones: TONES }`.
func TestWriteTones(t *testing.T) {
	rec := httptest.NewRecorder()
	WriteTones(rec, httptest.NewRequest(http.MethodGet, "/rewrite/tones", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}

	// Top-level key must be exactly "tones".
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(rec.Body.Bytes(), &raw); err != nil {
		t.Fatalf("body not JSON object: %v", err)
	}
	if _, ok := raw["tones"]; !ok || len(raw) != 1 {
		t.Fatalf("top-level keys = %v, want exactly {tones}", keysOf(raw))
	}

	var env tonesEnvelope
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatalf("unmarshal envelope: %v", err)
	}
	if len(env.Tones) != 8 {
		t.Fatalf("tone count = %d, want 8", len(env.Tones))
	}
	first := env.Tones[0]
	if first != (ToneMeta{ID: "simple", Label: "Simple", Icon: "✎", Kind: "rewrite"}) {
		t.Fatalf("first tone = %+v", first)
	}
	last := env.Tones[7]
	if last != (ToneMeta{ID: "translate", Label: "Translate", Icon: "🌐", Kind: "translate"}) {
		t.Fatalf("last tone = %+v", last)
	}

	// Exact marshaled body — pins key order + the leading {"tones":[...
	want := `{"tones":[{"id":"simple","label":"Simple","icon":"✎","kind":"rewrite"},`
	if !strings.HasPrefix(strings.TrimSpace(rec.Body.String()), want) {
		t.Fatalf("body prefix mismatch:\n got %s\nwant %s…", rec.Body.String(), want)
	}
}

func keysOf(m map[string]json.RawMessage) []string {
	ks := make([]string, 0, len(m))
	for k := range m {
		ks = append(ks, k)
	}
	return ks
}
