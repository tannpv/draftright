package main

import (
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
)

// diffJSON deep-compares two JSON bodies. Keys in ignoreValueOf are
// compared for presence + non-emptiness only (their values may
// legitimately differ per request, e.g. request_id). Returns a sorted
// list of human-readable diff lines; empty means parity.
func diffJSON(a, b []byte, ignoreValueOf []string) []string {
	var ma, mb map[string]any
	if err := json.Unmarshal(a, &ma); err != nil {
		return []string{fmt.Sprintf("node body not JSON: %v", err)}
	}
	if err := json.Unmarshal(b, &mb); err != nil {
		return []string{fmt.Sprintf("go body not JSON: %v", err)}
	}
	ignore := map[string]bool{}
	for _, k := range ignoreValueOf {
		ignore[k] = true
	}
	var diffs []string
	for _, k := range unionKeys(ma, mb) {
		av, aok := ma[k]
		bv, bok := mb[k]
		if ignore[k] {
			if !aok || !bok || isEmpty(av) || isEmpty(bv) {
				diffs = append(diffs, fmt.Sprintf("%s: must be present+non-empty on both (node=%v go=%v)", k, av, bv))
			}
			continue
		}
		if !reflect.DeepEqual(av, bv) {
			diffs = append(diffs, fmt.Sprintf("%s: node=%v go=%v", k, av, bv))
		}
	}
	sort.Strings(diffs)
	return diffs
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

func isEmpty(v any) bool {
	s, ok := v.(string)
	return v == nil || (ok && s == "")
}
