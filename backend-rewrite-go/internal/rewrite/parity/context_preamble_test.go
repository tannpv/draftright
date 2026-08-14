package parity

import (
	"strings"
	"testing"
)

// Golden vectors copied byte-for-byte from the Node authority
// (backend/src/user-context/user-context.constants.ts +
// user-context.constants.spec.ts). If Node's builder changes, these MUST change
// in lock-step — that is the whole point (RULE #1: two copies in different
// languages, guarded by a test that asserts they agree).

const noMention = "Apply this to the rewrite, but do not mention it or address the person."

func TestBuildContextPreamble_DisabledOrEmpty(t *testing.T) {
	// disabled with a full profile → empty
	if got := BuildContextPreamble(UserContextProfile{Enabled: false, JobTitle: "Lawyer", StyleNotes: "formal"}); got != "" {
		t.Fatalf("disabled: want empty, got %q", got)
	}
	// enabled but every field empty → empty (zero-cost no-op)
	if got := BuildContextPreamble(UserContextProfile{Enabled: true}); got != "" {
		t.Fatalf("empty: want empty, got %q", got)
	}
}

func TestBuildContextPreamble_WhoClause(t *testing.T) {
	got := BuildContextPreamble(UserContextProfile{
		Enabled: true, JobTitle: "Lawyer", Industry: "finance", Audience: "clients",
	})
	want := "About the person you are writing for: Lawyer, in finance, writing for clients. " + noMention + "\n\n"
	if got != want {
		t.Fatalf("who-clause mismatch:\n want %q\n  got %q", want, got)
	}
}

func TestBuildContextPreamble_StyleOnly(t *testing.T) {
	got := BuildContextPreamble(UserContextProfile{Enabled: true, StyleNotes: "British spelling, no emojis"})
	want := "Their writing style: British spelling, no emojis. " + noMention + "\n\n"
	if got != want {
		t.Fatalf("style-only mismatch:\n want %q\n  got %q", want, got)
	}
}

func TestBuildContextPreamble_WhoAndStyle(t *testing.T) {
	got := BuildContextPreamble(UserContextProfile{Enabled: true, JobTitle: "Engineer", StyleNotes: "concise"})
	want := "About the person you are writing for: Engineer. Their writing style: concise. " + noMention + "\n\n"
	if got != want {
		t.Fatalf("who+style mismatch:\n want %q\n  got %q", want, got)
	}
}

func TestBuildContextPreamble_AlwaysHasNoMentionAndSeparator(t *testing.T) {
	got := BuildContextPreamble(UserContextProfile{Enabled: true, JobTitle: "Nurse"})
	if !strings.Contains(got, "do not mention it") {
		t.Fatalf("missing no-mention instruction: %q", got)
	}
	if !strings.HasSuffix(got, "\n\n") {
		t.Fatalf("missing trailing separator: %q", got)
	}
}

func TestBuildContextPreamble_TruncatesToBudget(t *testing.T) {
	got := BuildContextPreamble(UserContextProfile{Enabled: true, StyleNotes: strings.Repeat("x", 5000)})
	// block content (minus the trailing "\n\n") never exceeds the cap
	if len([]rune(got)) > ContextPreambleMaxChars+2 {
		t.Fatalf("not truncated: len=%d", len([]rune(got)))
	}
	if !strings.HasSuffix(got, "\n\n") {
		t.Fatalf("truncated result lost its separator: %q", got)
	}
}
