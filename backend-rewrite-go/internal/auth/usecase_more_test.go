package auth

import (
	"context"
	"testing"
	"time"

	platauth "github.com/tannpv/draftright-rewrite/internal/platform/auth"
	"github.com/tannpv/draftright-rewrite/internal/shared"
	"github.com/tannpv/draftright-rewrite/internal/user"
)

func TestRefresh_ValidToken(t *testing.T) {
	u := user.User{ID: "u1", Email: "a@b.com", Role: "user", IsActive: true}
	svc := newSvc(t, &stubUsers{byID: map[string]user.User{"u1": u}})
	rt, _ := platauth.NewSigner("refresh-secret").Sign(platauth.Claims{Sub: "u1", Email: "a@b.com", Role: "user"}, time.Hour)
	toks, err := svc.Refresh(context.Background(), rt)
	if err != nil {
		t.Fatalf("refresh: %v", err)
	}
	if toks.AccessToken == "" || toks.RefreshToken == "" {
		t.Fatal("missing tokens")
	}
}

func TestRefresh_GarbageToken(t *testing.T) {
	svc := newSvc(t, &stubUsers{})
	_, err := svc.Refresh(context.Background(), "not-a-jwt")
	assertAuthErr(t, err, msgInvalidRefresh)
}

func TestRefresh_AccessTokenRejected(t *testing.T) {
	svc := newSvc(t, &stubUsers{byID: map[string]user.User{"u1": {ID: "u1", IsActive: true}}})
	at, _ := platauth.NewSigner("access-secret").Sign(platauth.Claims{Sub: "u1"}, time.Hour)
	_, err := svc.Refresh(context.Background(), at)
	assertAuthErr(t, err, msgInvalidRefresh)
}

func TestChangePassword_Success(t *testing.T) {
	hash, _ := shared.HashPassword("old")
	u := user.User{ID: "u1", PasswordHash: hash}
	svc := newSvc(t, &stubUsers{byID: map[string]user.User{"u1": u}})
	if err := svc.ChangePassword(context.Background(), "u1", "old", "new"); err != nil {
		t.Fatalf("change: %v", err)
	}
}

func TestChangePassword_WrongCurrent(t *testing.T) {
	hash, _ := shared.HashPassword("old")
	u := user.User{ID: "u1", PasswordHash: hash}
	svc := newSvc(t, &stubUsers{byID: map[string]user.User{"u1": u}})
	err := svc.ChangePassword(context.Background(), "u1", "WRONG", "new")
	assertAuthErr(t, err, msgCurrentPwWrong)
}
