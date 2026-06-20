package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
)

// diffJSON deep-compares two JSON response bodies and returns a sorted list
// of human-readable diff lines; empty means parity.
//
// Bodies may be a JSON object (most endpoints), a JSON array (e.g. GET /plans,
// GET /auth/extension-tokens), or empty (a 204 No Content, e.g. token revoke).
// Two empty bodies are parity; an empty-vs-nonempty body is a diff.
//
// Keys named in ignoreValueOf are compared for presence + non-emptiness only,
// wherever they appear in the tree (their values legitimately differ per
// request — request_id, a freshly-minted token, a row id, last_used_at, …).
//
// PARSED-COMPARE ALLOWANCES (load-bearing — see issues #38 and #47): both
// bodies are json.Unmarshal'd into `any` before comparing, so this differ is
// deliberately blind to three byte-level formatting differences that are NOT
// behavior changes:
//   - JSON key ORDER (maps are unordered) — #47.
//   - insignificant WHITESPACE — #38.
//   - NUMBER formatting (1.0 vs 1, 1e2 vs 100 both decode to float64) — #38.
//
// These are why a stored jsonb `context` (error_reports/bug_reports) compares
// equal even though Postgres canonicalizes it and Go round-trips raw bytes.
// Do NOT "fix" #38 by re-marshalling Go's context through a map encoder — that
// would SORT keys and break Postgres insertion-order. If a raw-bytes/string
// body-compare mode is ever added (the #47 harness ask, to catch key-order
// regressions), it MUST first canonicalize jsonb context fields with an
// ORDER-PRESERVING encoder (or allowlist them), or #38 resurfaces as a false
// positive.
func diffJSON(a, b []byte, ignoreValueOf []string) []string {
	ignore := map[string]bool{}
	for _, k := range ignoreValueOf {
		ignore[k] = true
	}

	ea, eb := isBlank(a), isBlank(b)
	if ea && eb {
		return nil // both empty (204 No Content) → parity
	}
	if ea != eb {
		return []string{fmt.Sprintf("body presence: node=%q go=%q", string(a), string(b))}
	}

	var va, vb any
	if err := json.Unmarshal(a, &va); err != nil {
		return []string{fmt.Sprintf("node body not JSON: %v", err)}
	}
	if err := json.Unmarshal(b, &vb); err != nil {
		return []string{fmt.Sprintf("go body not JSON: %v", err)}
	}

	diffs := cmpVal("", va, vb, ignore)
	sort.Strings(diffs)
	return diffs
}

// cmpVal recursively compares two decoded JSON values. label is the dotted/
// indexed path to the current node ("" at the root).
func cmpVal(label string, a, b any, ignore map[string]bool) []string {
	switch av := a.(type) {
	case map[string]any:
		bv, ok := b.(map[string]any)
		if !ok {
			return []string{fmt.Sprintf("%s: type: node=object go=%T", at(label), b)}
		}
		return cmpObject(label, av, bv, ignore)
	case []any:
		bv, ok := b.([]any)
		if !ok {
			return []string{fmt.Sprintf("%s: type: node=array go=%T", at(label), b)}
		}
		return cmpArray(label, av, bv, ignore)
	default:
		if !reflect.DeepEqual(a, b) {
			return []string{fmt.Sprintf("%s: node=%v go=%v", at(label), a, b)}
		}
		return nil
	}
}

func cmpObject(label string, a, b map[string]any, ignore map[string]bool) []string {
	var diffs []string
	for _, k := range unionKeys(a, b) {
		av, aok := a[k]
		bv, bok := b[k]
		child := join(label, k)
		if ignore[k] {
			// Value may differ per request; require presence + non-emptiness
			// on both sides only.
			if !aok || !bok || isEmpty(av) || isEmpty(bv) {
				diffs = append(diffs, fmt.Sprintf("%s: must be present+non-empty on both (node=%v go=%v)", child, av, bv))
			}
			continue
		}
		diffs = append(diffs, cmpVal(child, av, bv, ignore)...)
	}
	return diffs
}

func cmpArray(label string, a, b []any, ignore map[string]bool) []string {
	if len(a) != len(b) {
		return []string{fmt.Sprintf("%s: length: node=%d go=%d", at(label), len(a), len(b))}
	}
	var diffs []string
	for i := range a {
		diffs = append(diffs, cmpVal(fmt.Sprintf("%s[%d]", label, i), a[i], b[i], ignore)...)
	}
	return diffs
}

// at renders an empty path as "<root>" so root-level type/length diffs read
// sensibly; otherwise it returns the label unchanged.
func at(label string) string {
	if label == "" {
		return "<root>"
	}
	return label
}

func join(label, key string) string {
	if label == "" {
		return key
	}
	return label + "." + key
}

func unionKeys(a, b map[string]any) []string {
	set := map[string]bool{}
	for k := range a {
		set[k] = true
	}
	for k := range b {
		set[k] = true
	}
	out := make([]string, 0, len(set))
	for k := range set {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func isBlank(b []byte) bool {
	return len(bytes.TrimSpace(b)) == 0
}

func isEmpty(v any) bool {
	s, ok := v.(string)
	return v == nil || (ok && s == "")
}
