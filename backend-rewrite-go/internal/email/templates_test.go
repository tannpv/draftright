package email

import (
	"testing"
	"time"
)

func TestRender_VerificationSubjectAndSubst(t *testing.T) {
	subj, html := renderTemplate("verification", map[string]string{"name": "Al", "code": "123456"})
	if subj != "Welcome to DraftRight — confirm your email" {
		t.Fatalf("subject: %q", subj)
	}
	if !contains(html, "123456") || !contains(html, "Al") {
		t.Fatalf("substitution missing in html")
	}
}

func TestRender_PasswordResetSubject(t *testing.T) {
	subj, _ := renderTemplate("password-reset", map[string]string{"name": "Al", "code": "000111"})
	if subj != "Reset your DraftRight password" {
		t.Fatalf("subject: %q", subj)
	}
}

func TestRender_UnknownKeyEmpty(t *testing.T) {
	subj, html := renderTemplate("nope", nil)
	if subj != "" || html != "" {
		t.Fatalf("unknown key should be empty, got %q %q", subj, html)
	}
}

func TestRender_SubscriptionActivatedSubstitutesAll(t *testing.T) {
	subj, html := renderTemplate("subscription-activated", map[string]string{
		"name": "Al", "plan": "Pro", "amount": "$9.99", "expires": "Mon Jun 15 2026",
	})
	if subj != "Your DraftRight Pro subscription is active" {
		t.Fatalf("subject: %q", subj)
	}
	if contains(html, "{{") || contains(html, "}}") {
		t.Fatalf("leftover token in html: %q", html)
	}
	for _, want := range []string{"Al", "Pro", "$9.99", "Mon Jun 15 2026"} {
		if !contains(html, want) {
			t.Fatalf("html missing %q", want)
		}
	}
}

func TestRender_HTMLEscapesBodyNotSubject(t *testing.T) {
	subj, html := renderTemplate("subscription-expired", map[string]string{
		"name": "<b>Al</b>", "plan": "Pro",
	})
	// Subject rendered with escape=false → raw.
	if !contains(subj, "Pro") {
		t.Fatalf("subject: %q", subj)
	}
	// HTML body rendered with escape=true → escaped, never raw.
	if !contains(html, "&lt;b&gt;Al&lt;/b&gt;") {
		t.Fatalf("html should escape value, got %q", html)
	}
	if contains(html, "<b>Al</b>") {
		t.Fatalf("html must not contain raw value, got %q", html)
	}
}

func TestRender_SubjectEscapeFalseRaw(t *testing.T) {
	// subscription-activated subject contains {{plan}} — escape=false keeps raw.
	subj, _ := renderTemplate("subscription-activated", map[string]string{
		"name": "Al", "plan": "<b>", "amount": "$1.00", "expires": "x",
	})
	if !contains(subj, "<b>") {
		t.Fatalf("subject must keep raw value (escape=false), got %q", subj)
	}
}

func TestFormatAmount(t *testing.T) {
	cases := []struct {
		currency string
		amount   int
		want     string
	}{
		{"USD", 999, "$9.99"},
		{"VND", 50000, "50,000 VND"},
		{"VND", 1234567, "1,234,567 VND"},
		{"USD", 0, "$0.00"},
		{"VND", 0, "0 VND"},
		{"VND", 999, "999 VND"},
	}
	for _, c := range cases {
		if got := formatAmount(c.currency, c.amount); got != c.want {
			t.Fatalf("formatAmount(%q,%d) = %q want %q", c.currency, c.amount, got, c.want)
		}
	}
}

func TestRender_UnknownTokenEmpty(t *testing.T) {
	// A template with an unknown {{token}} → empty, not literal braces.
	out := substitute("hi {{nope}} end", map[string]string{}, false)
	if out != "hi  end" {
		t.Fatalf("unknown token should be empty, got %q", out)
	}
	outEsc := substitute("hi {{nope}} end", map[string]string{}, true)
	if outEsc != "hi  end" {
		t.Fatalf("unknown token (escaped) should be empty, got %q", outEsc)
	}
}

func TestDateString(t *testing.T) {
	// JS Date(2026,5,5).toDateString() === "Fri Jun 05 2026" (zero-padded day).
	d1 := time.Date(2026, time.June, 5, 12, 0, 0, 0, time.UTC)
	if got := dateString(d1); got != "Fri Jun 05 2026" {
		t.Fatalf("dateString 1-digit day = %q want %q", got, "Fri Jun 05 2026")
	}
	d2 := time.Date(2026, time.June, 15, 12, 0, 0, 0, time.UTC)
	if got := dateString(d2); got != "Mon Jun 15 2026" {
		t.Fatalf("dateString 2-digit day = %q want %q", got, "Mon Jun 15 2026")
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
