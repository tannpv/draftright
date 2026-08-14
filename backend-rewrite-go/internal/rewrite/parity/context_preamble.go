package parity

import (
	"strings"
	"unicode"
)

// Per-user personalization context (#173). This is the Go authority for the
// injected preamble; it MUST stay byte-for-byte identical to Node's
// buildContextPreamble (backend/src/user-context/user-context.constants.ts) or
// the two backends personalize the same user differently. context_preamble_test.go
// pins that with golden vectors copied from the Node authority — the #22 lesson
// (a copy-pasted upsert drifted and shipped a prod hole) applied up front.

// ContextPreambleMaxChars caps the injected block so a long profile can never
// blow the prompt's token budget. Mirrors Node CONTEXT_PREAMBLE_MAX_CHARS.
const ContextPreambleMaxChars = 900

// UserContextProfile is the decrypted, persisted profile the builder needs —
// the shape a repo adapter loads from user_contexts.
type UserContextProfile struct {
	Enabled    bool
	JobTitle   string
	Industry   string
	Audience   string
	StyleNotes string
}

// truncateContext reserves one rune for the ellipsis so the result is never
// longer than max — mirrors Node's truncate (slice(0, max-1).trimEnd()+'…').
func truncateContext(value string, max int) string {
	r := []rune(value)
	if len(r) <= max {
		return value
	}
	head := strings.TrimRightFunc(string(r[:max-1]), unicode.IsSpace)
	return head + "…"
}

// BuildContextPreamble mirrors Node buildContextPreamble: the system-context
// block prepended to the tone prompt, or "" when there is nothing to inject
// (disabled, or every field empty) — an empty result means the rewrite is
// byte-identical to today at zero cost. The model is told not to mention the
// context so it never leaks into output.
func BuildContextPreamble(ctx UserContextProfile) string {
	if !ctx.Enabled {
		return ""
	}

	var who []string
	if ctx.JobTitle != "" {
		who = append(who, ctx.JobTitle)
	}
	if ctx.Industry != "" {
		who = append(who, "in "+ctx.Industry)
	}
	if ctx.Audience != "" {
		who = append(who, "writing for "+ctx.Audience)
	}

	style := strings.TrimSpace(ctx.StyleNotes)
	if len(who) == 0 && style == "" {
		return ""
	}

	var parts []string
	if len(who) > 0 {
		parts = append(parts, "About the person you are writing for: "+strings.Join(who, ", ")+".")
	}
	if style != "" {
		parts = append(parts, "Their writing style: "+style+".")
	}
	parts = append(parts, "Apply this to the rewrite, but do not mention it or address the person.")

	block := strings.Join(parts, " ")
	return truncateContext(block, ContextPreambleMaxChars) + "\n\n"
}
