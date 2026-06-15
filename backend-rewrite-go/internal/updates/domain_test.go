package updates

import "testing"

func TestCompareVersions(t *testing.T) {
	cases := []struct {
		a, b string
		want int // sign only
	}{
		{"2.2.10", "2.2.9", 1},
		{"2.2.9", "2.2.10", -1},
		{"1.0", "1.0.0", 0},
		{"2.0", "1.9.9", 1},
		{"x.y", "0.0", 0}, // non-numeric → 0
		{"", "", 0},
	}
	for _, c := range cases {
		got := compareVersions(c.a, c.b)
		if sign(got) != c.want {
			t.Errorf("compareVersions(%q,%q) sign = %d, want %d", c.a, c.b, sign(got), c.want)
		}
	}
}

func sign(n int) int {
	if n > 0 {
		return 1
	}
	if n < 0 {
		return -1
	}
	return 0
}

func TestMaxVersion(t *testing.T) {
	if got := maxVersion([]string{"2.2.9", "2.3.0", "1.0.0"}); got != "2.3.0" {
		t.Errorf("maxVersion = %q, want 2.3.0", got)
	}
	if got := maxVersion(nil); got != "" {
		t.Errorf("maxVersion(nil) = %q, want empty", got)
	}
	// "0.0.0" vs seed "" compares to 0 (not >0) → seed wins, stays "".
	if got := maxVersion([]string{"0.0.0"}); got != "" {
		t.Errorf("maxVersion([0.0.0]) = %q, want empty (parity with Node reduce)", got)
	}
}

func TestBuildEnvelope_Anchor(t *testing.T) {
	all := map[string]*Release{
		"mac": {Version: "2.2.9", ReleaseNotes: "mac notes", Required: false},
		"ios": {Version: "2.4.1", ReleaseNotes: "ios notes", Required: true},
	}
	env := buildEnvelope(all, all["ios"])
	if env.Version != "2.4.1" || env.ReleaseNotes != "ios notes" || !env.Required {
		t.Fatalf("anchor envelope = %+v", env)
	}
}

func TestBuildEnvelope_NoAnchorUsesMaxVersionButMacNotes(t *testing.T) {
	all := map[string]*Release{
		"mac": {Version: "2.2.9", ReleaseNotes: "mac notes", Required: true},
		"ios": {Version: "2.4.1", ReleaseNotes: "ios notes", Required: false},
	}
	env := buildEnvelope(all, nil)
	// version = cross-platform max, but notes/required come from mac (legacy asymmetry).
	if env.Version != "2.4.1" {
		t.Errorf("version = %q, want 2.4.1 (max)", env.Version)
	}
	if env.ReleaseNotes != "mac notes" || !env.Required {
		t.Errorf("notes/required must come from mac, got notes=%q required=%v", env.ReleaseNotes, env.Required)
	}
}
