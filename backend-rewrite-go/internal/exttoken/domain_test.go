package exttoken_test

import (
	"regexp"
	"strings"
	"testing"

	"github.com/tannpv/draftright-rewrite/internal/exttoken"
)

func TestConstants(t *testing.T) {
	if exttoken.TokenPrefix != "dr_ext_" {
		t.Fatalf("TokenPrefix = %q, want dr_ext_", exttoken.TokenPrefix)
	}
	if exttoken.ScopeRewrite != "rewrite" {
		t.Fatalf("ScopeRewrite = %q, want rewrite", exttoken.ScopeRewrite)
	}
}

func TestSentinelErrorStrings(t *testing.T) {
	// Byte-compared against Node's RewriteAuthGuard UnauthorizedException msgs.
	cases := []struct {
		err  error
		want string
	}{
		{exttoken.ErrMissingToken, "Missing bearer token"},
		{exttoken.ErrInvalidToken, "Invalid extension token"},
		{exttoken.ErrMissingScope, "Token missing rewrite scope"},
	}
	for _, c := range cases {
		if c.err.Error() != c.want {
			t.Errorf("error = %q, want %q", c.err.Error(), c.want)
		}
	}
}

var hexRe = regexp.MustCompile(`^[0-9a-f]{64}$`)

func TestGenerateToken(t *testing.T) {
	plain, hash, err := exttoken.GenerateToken()
	if err != nil {
		t.Fatalf("GenerateToken err = %v", err)
	}
	if !strings.HasPrefix(plain, "dr_ext_") {
		t.Errorf("plain %q missing dr_ext_ prefix", plain)
	}
	// 32 bytes base64url-nopad = 43 chars + len("dr_ext_")=7 → 50.
	if len(plain) != 50 {
		t.Errorf("plain len = %d, want 50 (token=%q)", len(plain), plain)
	}
	if !hexRe.MatchString(hash) {
		t.Errorf("hash %q is not 64-hex", hash)
	}
	// hash must equal hashToken(plain).
	if got := exttoken.HashToken(plain); got != hash {
		t.Errorf("hash %q != HashToken(plain) %q", hash, got)
	}
}

func TestGenerateTokenUnique(t *testing.T) {
	a, ah, err := exttoken.GenerateToken()
	if err != nil {
		t.Fatal(err)
	}
	b, bh, err := exttoken.GenerateToken()
	if err != nil {
		t.Fatal(err)
	}
	if a == b {
		t.Errorf("two GenerateToken calls produced identical plaintext %q", a)
	}
	if ah == bh {
		t.Errorf("two GenerateToken calls produced identical hash %q", ah)
	}
}

func TestHashTokenDeterministicAndKnownVector(t *testing.T) {
	const in = "dr_ext_known"
	// sha256("dr_ext_known") computed independently.
	const want = "c2e0b2e1d1cae183abc147112c8e8264a0a55308a93b3e2cbbb9b15c28904b5a"
	got := exttoken.HashToken(in)
	if got != want {
		t.Errorf("HashToken(%q) = %q, want %q", in, got, want)
	}
	if again := exttoken.HashToken(in); again != got {
		t.Errorf("HashToken not deterministic: %q vs %q", got, again)
	}
}
