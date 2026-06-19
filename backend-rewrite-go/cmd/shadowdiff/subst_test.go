package main

import "testing"

func TestSubstitute_HeaderAndBody(t *testing.T) {
	f := fixture{
		Headers: map[string]string{"Authorization": "Bearer {{user_token}}"},
		Body:    []byte(`{"token":"{{ext_token}}"}`),
	}
	vars := map[string]string{"user_token": "AAA", "ext_token": "BBB"}
	out := substitute(f, vars)
	if out.Headers["Authorization"] != "Bearer AAA" {
		t.Fatalf("header = %q", out.Headers["Authorization"])
	}
	if string(out.Body) != `{"token":"BBB"}` {
		t.Fatalf("body = %q", out.Body)
	}
}

func TestSubstitute_UnknownPlaceholderLeftIntact(t *testing.T) {
	f := fixture{Headers: map[string]string{"X": "{{nope}}"}}
	out := substitute(f, map[string]string{"user_token": "AAA"})
	if out.Headers["X"] != "{{nope}}" {
		t.Fatalf("unknown placeholder should be left as-is, got %q", out.Headers["X"])
	}
}

func TestSubstitute_DoesNotMutateInput(t *testing.T) {
	f := fixture{Headers: map[string]string{"Authorization": "Bearer {{user_token}}"}}
	_ = substitute(f, map[string]string{"user_token": "AAA"})
	if f.Headers["Authorization"] != "Bearer {{user_token}}" {
		t.Fatal("input fixture must not be mutated")
	}
}
