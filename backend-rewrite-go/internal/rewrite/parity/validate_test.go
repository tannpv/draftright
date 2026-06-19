package parity

import "testing"

func TestValidateRewrite(t *testing.T) {
	const toneMsg = "tone must be one of the following values: simple, natural, polished, concise, technical, claude, grammar_check, translate"

	cases := []struct {
		name string
		body string
		want string // expected joined error message; "" = valid
	}{
		{
			name: "empty body — text then tone, in declaration order",
			body: `{}`,
			want: "text must be a string. " + toneMsg,
		},
		{
			name: "bad tone only",
			body: `{"text":"hi","tone":"nope"}`,
			want: toneMsg,
		},
		{
			name: "non-string text",
			body: `{"text":5,"tone":"polished"}`,
			want: "text must be a string",
		},
		{
			name: "unknown key",
			body: `{"text":"hi","tone":"polished","bogus":1}`,
			want: "property bogus should not exist",
		},
		{
			name: "non-string target_language",
			body: `{"text":"hi","tone":"polished","target_language":5}`,
			want: "target_language must be a string",
		},
		{
			name: "non-string source_language",
			body: `{"text":"hi","tone":"polished","source_language":5}`,
			want: "source_language must be a string",
		},
		{
			name: "valid minimal",
			body: `{"text":"hi","tone":"polished"}`,
			want: "",
		},
		{
			name: "valid with optionals",
			body: `{"text":"hi","tone":"translate","target_language":"Vietnamese","source_language":"English"}`,
			want: "",
		},
		{
			name: "grammar_check tone valid",
			body: `{"text":"hi","tone":"grammar_check"}`,
			want: "",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, _, _, _, msg := validateRewrite([]byte(tc.body))
			if msg != tc.want {
				t.Fatalf("msg = %q, want %q", msg, tc.want)
			}
		})
	}
}

func TestValidateRewrite_ParsedFields(t *testing.T) {
	text, tone, target, source, msg := validateRewrite([]byte(
		`{"text":"hello","tone":"translate","target_language":"Vietnamese","source_language":"English"}`))
	if msg != "" {
		t.Fatalf("unexpected error: %q", msg)
	}
	if text != "hello" || tone != "translate" || target != "Vietnamese" || source != "English" {
		t.Fatalf("parsed = %q/%q/%q/%q", text, tone, target, source)
	}
}
