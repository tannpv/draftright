package auth

import "testing"

func TestValidateRegister(t *testing.T) {
	cases := []struct{ email, pw, name, want string }{
		{"a@b.com", "password8", "Al", ""},
		{"notanemail", "password8", "Al", "Please enter a valid email address."},
		{"a@b.com", "short", "Al", "Password must be at least 8 characters."},
		{"a@b.com", "password8", "", "Name must be at least 1 characters."},
		{"bad", "short", "", "Please enter a valid email address. Password must be at least 8 characters. Name must be at least 1 characters."},
	}
	for _, c := range cases {
		got := validateRegister(c.email, c.pw, c.name)
		if got != c.want {
			t.Fatalf("validateRegister(%q,%q,%q) = %q, want %q", c.email, c.pw, c.name, got, c.want)
		}
	}
}
