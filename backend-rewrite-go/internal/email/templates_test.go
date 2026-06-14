package email

import "testing"

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

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
