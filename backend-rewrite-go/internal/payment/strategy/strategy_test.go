package strategy

import "testing"

func TestResolveCredential(t *testing.T) {
	cases := []struct{ db, env, want string }{
		{"from-db", "from-env", "from-db"}, // DB priority
		{"", "from-env", "from-env"},       // empty DB → env
		{"", "", ""},                       // both empty
	}
	for _, c := range cases {
		if got := ResolveCredential(c.db, c.env); got != c.want {
			t.Fatalf("ResolveCredential(%q,%q)=%q want %q", c.db, c.env, got, c.want)
		}
	}
}

func TestTimingSafeStrEqual(t *testing.T) {
	if !TimingSafeStrEqual("secret", "secret") {
		t.Fatal("equal strings must compare true")
	}
	if TimingSafeStrEqual("secret", "secre") {
		t.Fatal("length mismatch must be false")
	}
	if TimingSafeStrEqual("secret", "secreT") {
		t.Fatal("differing strings must be false")
	}
}
