package main

import "testing"

func TestParseAccessToken_OK(t *testing.T) {
	tok, err := parseAccessToken([]byte(`{"access_token":"abc.def.ghi","refresh_token":"x"}`))
	if err != nil {
		t.Fatal(err)
	}
	if tok != "abc.def.ghi" {
		t.Fatalf("tok = %q", tok)
	}
}

func TestParseAccessToken_Missing(t *testing.T) {
	if _, err := parseAccessToken([]byte(`{"refresh_token":"x"}`)); err == nil {
		t.Fatal("missing access_token must error")
	}
}

func TestParseExtToken_OK(t *testing.T) {
	tok, err := parseExtToken([]byte(`{"token":"dr_ext_abc","id":"1"}`))
	if err != nil {
		t.Fatal(err)
	}
	if tok != "dr_ext_abc" {
		t.Fatalf("tok = %q", tok)
	}
}
