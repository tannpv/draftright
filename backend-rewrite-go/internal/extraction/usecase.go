package extraction

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"sort"
	"strconv"
	"strings"
)

// ErrNoDefaultProvider mirrors the Node BadRequestException thrown by
// AiProvidersService.findDefault() when no default AI provider is configured
// ("No default AI provider configured"). The provider/Completer layer surfaces
// this condition; the handler maps it to a 400 invalid-input (wired in Task 11).
var ErrNoDefaultProvider = errors.New("No default AI provider configured")

// DefaultProvider is the consumer-side port: resolve the configured default
// AI provider and run one blocking system+user completion, returning the text
// AND the provider's name (Node provider.name). Resolving per request (not a
// process-static provider) mirrors Node ExtractionService, which calls
// aiProviders.findDefault() inside extract(). The implementation surfaces
// ErrNoDefaultProvider when no default is configured.
type DefaultProvider interface {
	DefaultComplete(ctx context.Context, system, user string) (text, name string, err error)
}

// Service ports ExtractionService: LLM call → JSON parse → validate → dedupe.
type Service struct{ p DefaultProvider }

// NewService wires the default-provider port.
func NewService(p DefaultProvider) *Service { return &Service{p: p} }

// Extract ports ExtractionService.extract. On a provider/Complete error it
// returns the zero Response + the error (Task 4 maps provider errors). All
// parse/validation failures degrade to an empty entity array (never error),
// matching Node.
func (s *Service) Extract(ctx context.Context, text string, kinds []EntityKind) (Response, error) {
	system := buildSystemPrompt(kinds)
	raw, name, err := s.p.DefaultComplete(ctx, system, text)
	if err != nil {
		return Response{}, err
	}

	cleaned := stripCodeFences(raw)
	var arr []json.RawMessage
	if err := json.Unmarshal([]byte(cleaned), &arr); err != nil {
		// JS: JSON.parse throw OR non-array → empty response. Node logs
		// sha1-first8 of the raw output on parse failure.
		sum := sha1.Sum([]byte(raw))
		slog.Warn("extraction_llm_unparseable",
			"len", lenUTF16(raw), "sha1", hex.EncodeToString(sum[:])[:8])
		return Response{Entities: []Entity{}, Provider: name, TokensUsed: 0}, nil
	}

	out := make([]Entity, 0, len(arr))
	for _, item := range arr {
		if e, ok := validateEntity(item, text); ok {
			out = append(out, e)
		}
	}
	return Response{
		Entities:   dedupe(out),
		Provider:   name,
		TokensUsed: estimateTokens(text + raw),
	}, nil
}

// buildSystemPrompt ports ExtractionService.buildSystemPrompt byte-for-byte
// (the array .join("\n")). Default kinds when nil/empty = address,
// personName, dateTime, bankAccount; regex-handled kinds are filtered out.
func buildSystemPrompt(kinds []EntityKind) string {
	src := kinds
	if len(src) == 0 {
		src = defaultKinds
	}
	allowed := make([]string, 0, len(src))
	for _, k := range src {
		if !regexHandled[k] {
			allowed = append(allowed, string(k))
		}
	}
	forbidden := make([]string, 0, len(regexHandledOrder))
	for _, k := range regexHandledOrder {
		forbidden = append(forbidden, string(k))
	}
	lines := []string{
		"You extract structured entities from short messages.",
		"Return strict JSON array, no commentary. No code fences.",
		"Each item: {kind, value, display, confidence, meta?}.",
		fmt.Sprintf("Kinds you MAY emit: %s.", strings.Join(allowed, "|")),
		fmt.Sprintf("Kinds you MUST NOT emit (handled by client regex): %s.", strings.Join(forbidden, "|")),
		"value MUST be a literal substring of the input. confidence is 0..1.",
		`Example input: "Địa chỉ 123 Lê Lợi, Q1. Vietcombank 0123456789"`,
		`Example output: [{"kind":"address","value":"123 Lê Lợi, Q1","display":"123 Lê Lợi, Q1","confidence":0.9},{"kind":"bankAccount","value":"0123456789","display":"Vietcombank · 0123456789","confidence":0.95,"meta":{"bank":"Vietcombank"}}]`,
	}
	return strings.Join(lines, "\n")
}

// stripCodeFences ports ExtractionService.stripCodeFences. trim; if it
// starts with ``` strip the first line, then cut at the last ```; else
// return the trimmed string. Lengths/offsets here are byte-safe because the
// fence markers + newline are ASCII.
func stripCodeFences(s string) string {
	trimmed := strings.TrimSpace(s)
	if strings.HasPrefix(trimmed, "```") {
		firstNl := strings.Index(trimmed, "\n")
		var body string
		if firstNl >= 0 {
			body = trimmed[firstNl+1:]
		}
		endIdx := strings.LastIndex(body, "```")
		if endIdx >= 0 {
			return strings.TrimSpace(body[:endIdx])
		}
		return strings.TrimSpace(body)
	}
	return trimmed
}

// rawEntity mirrors the loose shape Node reads off each parsed item. Each
// field stays json.RawMessage so we can replicate JS typeof checks exactly
// (string vs number vs object) before coercion.
type rawEntity struct {
	Kind       json.RawMessage `json:"kind"`
	Value      json.RawMessage `json:"value"`
	Display    json.RawMessage `json:"display"`
	Confidence json.RawMessage `json:"confidence"`
	Meta       json.RawMessage `json:"meta"`
}

// validateEntity ports ExtractionService.validateEntity. Returns ok=false to
// drop the item (mirrors Node returning null).
func validateEntity(item json.RawMessage, text string) (Entity, bool) {
	// typeof raw !== 'object' || raw === null → drop. A JSON object decodes
	// into rawEntity; anything else (array, string, number, null) is rejected.
	if !isJSONObject(item) {
		return Entity{}, false
	}
	var r rawEntity
	if err := json.Unmarshal(item, &r); err != nil {
		return Entity{}, false
	}

	kindStr, ok := asString(r.Kind)
	if !ok {
		return Entity{}, false
	}
	kind := EntityKind(kindStr)
	if !kindValid(kind) {
		return Entity{}, false
	}
	if regexHandled[kind] {
		return Entity{}, false // defense in depth
	}

	value, ok := asString(r.Value)
	if !ok || value == "" {
		return Entity{}, false
	}
	start := indexUTF16(text, value)
	if start < 0 {
		slog.Warn("extraction_hallucination", "kind", string(kind), "value_len", lenUTF16(value))
		return Entity{}, false
	}

	// display = trimmed-nonempty string display else value.
	display := value
	if d, ok := asString(r.Display); ok && strings.TrimSpace(d) != "" {
		display = d
	}

	// confidence = clamp(number else 0.5, 0, 1).
	confidence := 0.5
	if n, ok := asNumber(r.Confidence); ok {
		confidence = n
	}
	confidence = math.Max(0, math.Min(1, confidence))

	meta := asMeta(r.Meta)

	return Entity{
		Kind:       kind,
		Value:      value,
		Display:    display,
		Start:      start,
		End:        start + lenUTF16(value),
		Confidence: confidence,
		Meta:       meta,
	}, true
}

// dedupe ports ExtractionService.dedupe: key = kind + ":" + lower(value),
// keep higher confidence; then sort by start ascending (stable to mirror
// JS Array.prototype.sort stability in V8 for equal keys).
func dedupe(items []Entity) []Entity {
	byKey := make(map[string]int) // key → index in out
	out := make([]Entity, 0, len(items))
	for _, e := range items {
		key := string(e.Kind) + ":" + strings.ToLower(e.Value)
		if idx, seen := byKey[key]; seen {
			if e.Confidence > out[idx].Confidence {
				out[idx] = e
			}
			continue
		}
		byKey[key] = len(out)
		out = append(out, e)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Start < out[j].Start })
	return out
}

// estimateTokens ports ExtractionService.estimateTokens: Math.ceil(len/4)
// over UTF-16 code units.
func estimateTokens(s string) int {
	return (lenUTF16(s) + 3) / 4
}

// isJSONObject reports whether raw is a JSON object ({...}), matching JS
// `typeof raw === 'object' && raw !== null` AND Node's subsequent property
// reads (arrays would pass typeof but field access yields undefined → drop;
// we reject arrays up front, which is equivalent since none of kind/value
// would be a valid string on an array).
func isJSONObject(raw json.RawMessage) bool {
	t := strings.TrimSpace(string(raw))
	return len(t) > 0 && t[0] == '{'
}

// asString returns the string value of raw when it is a JSON string (JS
// typeof === 'string'), else ok=false.
func asString(raw json.RawMessage) (string, bool) {
	if len(raw) == 0 {
		return "", false
	}
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		return "", false
	}
	return s, true
}

// asNumber returns the float value of raw when it is a JSON number (JS
// typeof === 'number'), else ok=false. A QUOTED-numeric string ("0.9") is
// rejected: Go's json.Number would happily decode it, but Node's
// `typeof raw.confidence === 'number'` is FALSE for a string, so the caller
// must fall back to the 0.5 default. Require the first non-space byte to be a
// sign or digit; a leading '"' (or anything else) is "not a number".
func asNumber(raw json.RawMessage) (float64, bool) {
	t := strings.TrimSpace(string(raw))
	if t == "" {
		return 0, false
	}
	if c := t[0]; c != '-' && c != '+' && (c < '0' || c > '9') {
		return 0, false
	}
	var n json.Number
	dec := json.NewDecoder(strings.NewReader(string(raw)))
	dec.UseNumber()
	if err := dec.Decode(&n); err != nil {
		return 0, false
	}
	f, err := n.Float64()
	if err != nil {
		return 0, false
	}
	return f, true
}

// asMeta ports the meta branch: when raw is a JSON object, return a non-nil
// pointer to map[String(k)]String(v) (possibly an empty map, so an explicit
// "meta":{} marshals back as "meta":{}); else nil (absent/non-object → key
// omitted). Mirrors Node `raw.meta && typeof raw.meta === 'object' ?
// Object.fromEntries(Object.entries(raw.meta).map(([k,v]) =>
// [String(k), String(v)])) : undefined`.
func asMeta(raw json.RawMessage) *map[string]string {
	if len(raw) == 0 || !isJSONObject(raw) {
		return nil
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil
	}
	out := make(map[string]string, len(m))
	for k, v := range m {
		out[k] = jsString(v)
	}
	return &out
}

// jsString coerces a parsed JSON value to its JS String(...) form for meta
// values: string → as-is, number → minimal decimal, bool → true/false,
// null → "null", object/array → JSON text (close enough; Node's String()
// on objects yields "[object Object]", but the LLM is instructed to emit
// flat string/number meta — see DEVIATION note).
func jsString(raw json.RawMessage) string {
	t := strings.TrimSpace(string(raw))
	if t == "" {
		return ""
	}
	switch t[0] {
	case '"':
		var s string
		if err := json.Unmarshal(raw, &s); err == nil {
			return s
		}
		return t
	case 't', 'f':
		return t // true / false
	case 'n':
		return "null"
	case '{', '[':
		return t
	default: // number
		var n json.Number
		dec := json.NewDecoder(strings.NewReader(t))
		dec.UseNumber()
		if err := dec.Decode(&n); err == nil {
			return jsNumberString(n.String())
		}
		return t
	}
}

// jsNumberString normalises a JSON number's text toward JS String(number):
// integral floats lose the decimal (5.0 → "5"); other forms pass through.
func jsNumberString(s string) string {
	if f, err := strconv.ParseFloat(s, 64); err == nil && f == math.Trunc(f) && !strings.ContainsAny(s, "eE") {
		return strconv.FormatInt(int64(f), 10)
	}
	return s
}
