package auth

import (
	"context"
	"testing"

	"github.com/tannpv/draftright-rewrite/internal/user"
)

func TestRegister_HappyPath(t *testing.T) {
	users := newStubUsers()
	svc := newSvc(t, users)
	res, err := svc.Register(context.Background(), "  NEW@B.com ", "password8", "Al")
	if err != nil {
		t.Fatalf("register: %v", err)
	}
	if res.User.Email != "new@b.com" {
		t.Fatalf("email not normalized: %q", res.User.Email)
	}
	if res.User.EmailVerified {
		t.Fatal("email_verified must be false")
	}
	if res.AccessToken == "" || res.RefreshToken == "" {
		t.Fatal("missing tokens")
	}
	if !users.created {
		t.Fatal("user not created")
	}
}

func TestRegister_DuplicateEmail(t *testing.T) {
	users := newStubUsers()
	users.byEmail["dup@b.com"] = user.User{ID: "u1", Email: "dup@b.com"}
	svc := newSvc(t, users)
	_, err := svc.Register(context.Background(), "dup@b.com", "password8", "Al")
	assertConflict(t, err, "Email already registered")
}
