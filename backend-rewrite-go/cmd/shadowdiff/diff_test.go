package main

import "testing"

func TestDiffJSON_IgnoresRequestIDValue(t *testing.T) {
	node := []byte(`{"error":"x","code":"invalid-input","request_id":"aaaa"}`)
	goB := []byte(`{"error":"x","code":"invalid-input","request_id":"bbbb"}`)
	diffs := diffJSON(node, goB, []string{"request_id"})
	if len(diffs) != 0 {
		t.Fatalf("request_id value diff should be ignored, got %v", diffs)
	}
}

func TestDiffJSON_FlagsRealMismatch(t *testing.T) {
	node := []byte(`{"role":"user"}`)
	goB := []byte(`{"role":"admin"}`)
	diffs := diffJSON(node, goB, nil)
	if len(diffs) == 0 {
		t.Fatal("role mismatch must be reported")
	}
}

func TestDiffJSON_RequestIDMustBePresentInBoth(t *testing.T) {
	node := []byte(`{"request_id":"aaaa"}`)
	goB := []byte(`{}`)
	diffs := diffJSON(node, goB, []string{"request_id"})
	if len(diffs) == 0 {
		t.Fatal("missing request_id on one side must be reported even when value is ignored")
	}
}
