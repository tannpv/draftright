package extraction

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

type fakeCompleter struct{ out string }

func (f fakeCompleter) Complete(context.Context, string, string) (string, int64, error) {
	return f.out, 1, nil
}
func (f fakeCompleter) Name() string { return "openai" }

func extract(t *testing.T, out, text string) Response {
	t.Helper()
	svc := NewService(fakeCompleter{out: out})
	resp, err := svc.Extract(context.Background(), text, nil)
	if err != nil {
		t.Fatalf("Extract returned error: %v", err)
	}
	return resp
}

// (a) hallucinated value (not a substring of text) → dropped.
func TestExtract_DropsHallucination(t *testing.T) {
	text := "Call John Smith tomorrow"
	out := `[{"kind":"personName","value":"Jane Doe","display":"Jane Doe","confidence":0.9}]`
	resp := extract(t, out, text)
	if len(resp.Entities) != 0 {
		t.Fatalf("expected hallucinated entity dropped, got %d", len(resp.Entities))
	}
}

// (b) a regexHandled kind (phone) → dropped even when value is a real substring.
func TestExtract_DropsRegexHandledKind(t *testing.T) {
	text := "ring me at 0123456789 ok"
	out := `[{"kind":"phone","value":"0123456789","display":"0123456789","confidence":0.99}]`
	resp := extract(t, out, text)
	if len(resp.Entities) != 0 {
		t.Fatalf("expected regex-handled phone dropped, got %d", len(resp.Entities))
	}
}

// (c) dedupe: same kind + lowercased value → keep MAX confidence; sorted by start asc.
func TestExtract_DedupeKeepsMaxConfidenceAndSorts(t *testing.T) {
	// text contains "Hanoi" (at a later offset) and "John" earlier.
	// Two personName "John" entries dedupe to the higher confidence (0.8).
	text := "John met john near Hanoi"
	out := `[
		{"kind":"address","value":"Hanoi","display":"Hanoi","confidence":0.7},
		{"kind":"personName","value":"John","display":"John","confidence":0.3},
		{"kind":"personName","value":"john","display":"john","confidence":0.8}
	]`
	resp := extract(t, out, text)
	if len(resp.Entities) != 2 {
		t.Fatalf("expected 2 entities after dedupe, got %d: %+v", len(resp.Entities), resp.Entities)
	}
	// dedupe keeps the HIGHER-confidence entity object (the "john" at start 9,
	// conf 0.8), mirroring Node's map.set on higher confidence. "Hanoi" is at
	// start 18. Sorted by start asc: john(9) before Hanoi(18).
	if resp.Entities[0].Kind != KindPersonName {
		t.Fatalf("expected personName first (start asc), got %s", resp.Entities[0].Kind)
	}
	if resp.Entities[0].Confidence != 0.8 {
		t.Fatalf("expected dedupe to keep max confidence 0.8, got %v", resp.Entities[0].Confidence)
	}
	if resp.Entities[0].Value != "john" {
		t.Fatalf("expected dedupe to keep the higher-confidence 'john' object, got %q", resp.Entities[0].Value)
	}
	if resp.Entities[1].Kind != KindAddress {
		t.Fatalf("expected address second, got %s", resp.Entities[1].Kind)
	}
	if !(resp.Entities[1].Start > resp.Entities[0].Start) {
		t.Fatalf("entities not sorted by start asc: %+v", resp.Entities)
	}
}

// (d) unparseable provider output → empty Response; entities marshals as [].
func TestExtract_UnparseableReturnsEmptyAndMarshalsArray(t *testing.T) {
	resp := extract(t, "this is not json at all", "some text")
	if len(resp.Entities) != 0 {
		t.Fatalf("expected 0 entities, got %d", len(resp.Entities))
	}
	if resp.Provider != "openai" {
		t.Fatalf("expected provider openai, got %q", resp.Provider)
	}
	if resp.TokensUsed != 0 {
		t.Fatalf("expected tokensUsed 0 on parse failure, got %d", resp.TokensUsed)
	}
	b, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if got := string(b); got != `{"entities":[],"provider":"openai","tokensUsed":0}` {
		t.Fatalf("empty entities must marshal as []: got %s", got)
	}
}

// non-array top-level JSON → same empty Response.
func TestExtract_NonArrayReturnsEmpty(t *testing.T) {
	resp := extract(t, `{"kind":"address"}`, "some text")
	if len(resp.Entities) != 0 {
		t.Fatalf("expected 0 entities for non-array, got %d", len(resp.Entities))
	}
	if resp.TokensUsed != 0 {
		t.Fatalf("expected tokensUsed 0 for non-array, got %d", resp.TokensUsed)
	}
}

// (e) tokensUsed = ceil((lenUTF16(text)+lenUTF16(raw))/4).
func TestExtract_TokensUsedCeilDiv4(t *testing.T) {
	text := "John is here"
	out := `[{"kind":"personName","value":"John","display":"John","confidence":0.9}]`
	resp := extract(t, out, text)
	want := (lenUTF16(text) + lenUTF16(out) + 3) / 4
	if resp.TokensUsed != want {
		t.Fatalf("tokensUsed = %d, want %d", resp.TokensUsed, want)
	}
}

// non-BMP / Vietnamese offset parity for indexUTF16 vs JS String.indexOf.
func TestIndexUTF16_NonBMPAndVietnamese(t *testing.T) {
	// "😀" is U+1F600, one rune = TWO UTF-16 code units (a surrogate pair).
	// JS: "a😀bc".indexOf("bc") === 3 (a=0, surrogate hi=1, lo=2, b=3).
	if got := indexUTF16("a😀bc", "bc"); got != 3 {
		t.Fatalf("indexUTF16 emoji: got %d, want 3", got)
	}
	// JS: "Lê Lợi".indexOf("Lợi") — BMP chars, one code unit each.
	if got := indexUTF16("123 Lê Lợi, Q1", "Lê Lợi"); got != 4 {
		t.Fatalf("indexUTF16 vietnamese: got %d, want 4", got)
	}
	if got := indexUTF16("abc", "zzz"); got != -1 {
		t.Fatalf("indexUTF16 miss: got %d, want -1", got)
	}
}

// validateEntity meta object → map[string]string; display fallback.
func TestExtract_MetaAndDisplayFallback(t *testing.T) {
	text := "Vietcombank 0123456789 here"
	out := `[{"kind":"bankAccount","value":"0123456789","display":"  ","confidence":0.95,"meta":{"bank":"Vietcombank"}}]`
	resp := extract(t, out, text)
	if len(resp.Entities) != 1 {
		t.Fatalf("expected 1 entity, got %d", len(resp.Entities))
	}
	e := resp.Entities[0]
	// display is blank-after-trim → falls back to value.
	if e.Display != "0123456789" {
		t.Fatalf("expected display fallback to value, got %q", e.Display)
	}
	if e.Meta == nil || (*e.Meta)["bank"] != "Vietcombank" {
		t.Fatalf("expected meta bank=Vietcombank, got %v", e.Meta)
	}
	if e.End != e.Start+lenUTF16(e.Value) {
		t.Fatalf("end mismatch: start=%d end=%d", e.Start, e.End)
	}
}

// Fix 1 — confidence parity with Node's `typeof raw.confidence === 'number'`.
// A QUOTED-numeric string ("0.9") is NOT a JS number → falls back to 0.5.
// A genuine JSON number (0.9) is accepted.
func TestExtract_ConfidenceStringRejectedNumberAccepted(t *testing.T) {
	text := "Hanoi is nice"
	// string "0.9" → rejected → default 0.5.
	outStr := `[{"kind":"address","value":"Hanoi","display":"Hanoi","confidence":"0.9"}]`
	resp := extract(t, outStr, text)
	if len(resp.Entities) != 1 {
		t.Fatalf("expected 1 entity, got %d", len(resp.Entities))
	}
	if resp.Entities[0].Confidence != 0.5 {
		t.Fatalf("quoted-string confidence must fall back to 0.5, got %v", resp.Entities[0].Confidence)
	}
	// number 0.9 → accepted.
	outNum := `[{"kind":"address","value":"Hanoi","display":"Hanoi","confidence":0.9}]`
	resp = extract(t, outNum, text)
	if len(resp.Entities) != 1 {
		t.Fatalf("expected 1 entity, got %d", len(resp.Entities))
	}
	if resp.Entities[0].Confidence != 0.9 {
		t.Fatalf("numeric confidence must be 0.9, got %v", resp.Entities[0].Confidence)
	}
}

// Fix 2 — meta presence parity (marshaled bytes).
//   - "meta":{}            → response JSON contains "meta":{}
//   - "meta":{"bank":"x"}  → "meta":{"bank":"x"}
//   - no meta              → marshaled JSON has NO "meta" key
func TestExtract_MetaPresenceMarshaledBytes(t *testing.T) {
	text := "Hanoi is nice"

	// explicit empty object → emit "meta":{}.
	resp := extract(t, `[{"kind":"address","value":"Hanoi","display":"Hanoi","confidence":0.9,"meta":{}}]`, text)
	if len(resp.Entities) != 1 {
		t.Fatalf("expected 1 entity, got %d", len(resp.Entities))
	}
	b, err := json.Marshal(resp.Entities[0])
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if !strings.Contains(string(b), `"meta":{}`) {
		t.Fatalf("explicit empty meta must marshal as \"meta\":{}, got %s", b)
	}

	// populated object → emit the key/value.
	resp = extract(t, `[{"kind":"address","value":"Hanoi","display":"Hanoi","confidence":0.9,"meta":{"bank":"x"}}]`, text)
	b, err = json.Marshal(resp.Entities[0])
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if !strings.Contains(string(b), `"meta":{"bank":"x"}`) {
		t.Fatalf("populated meta must marshal with kv, got %s", b)
	}

	// no meta → key omitted entirely.
	resp = extract(t, `[{"kind":"address","value":"Hanoi","display":"Hanoi","confidence":0.9}]`, text)
	b, err = json.Marshal(resp.Entities[0])
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if strings.Contains(string(b), "meta") {
		t.Fatalf("absent meta must omit the key, got %s", b)
	}
}
