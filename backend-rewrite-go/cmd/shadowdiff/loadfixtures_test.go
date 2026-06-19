package main

import (
	"os"
	"path/filepath"
	"testing"
)

// TestLoadFixturesRecurses guards the gate against silently loading zero
// fixtures: run-gate.sh points loadFixtures at the top-level fixtures/ dir
// while every fixture lives in a module sub-dir. A non-recursive Glob would
// match nothing there and the gate would pass vacuously.
func TestLoadFixturesRecurses(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "top.json"), `{"name":"top","method":"GET","path":"/a"}`)
	mustWrite(t, filepath.Join(root, "mod_a", "one.json"), `{"name":"a_one","method":"GET","path":"/b"}`)
	mustWrite(t, filepath.Join(root, "mod_b", "two.json"), `{"name":"b_two","method":"GET","path":"/c"}`)
	mustWrite(t, filepath.Join(root, "mod_b", "notes.txt"), `ignore me`)

	got, err := loadFixtures(root)
	if err != nil {
		t.Fatalf("loadFixtures: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("want 3 fixtures (top + 2 sub-dir), got %d", len(got))
	}
	names := map[string]bool{}
	for _, f := range got {
		names[f.Name] = true
	}
	for _, want := range []string{"top", "a_one", "b_two"} {
		if !names[want] {
			t.Errorf("missing fixture %q; got %v", want, names)
		}
	}
}

// TestLoadFixturesNamesFromFile confirms a fixture with no explicit name
// falls back to the file base name.
func TestLoadFixturesNamesFromFile(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "sub", "unnamed.json"), `{"method":"GET","path":"/x"}`)
	got, err := loadFixtures(root)
	if err != nil {
		t.Fatalf("loadFixtures: %v", err)
	}
	if len(got) != 1 || got[0].Name != "unnamed.json" {
		t.Fatalf("want name fallback to file base, got %+v", got)
	}
}

func mustWrite(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}
