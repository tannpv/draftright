package adminauth

import (
	"context"
	"testing"

	platauth "github.com/tannpv/draftright-rewrite/internal/platform/auth"
	"github.com/tannpv/draftright-rewrite/internal/shared"
)

const (
	accSecret = "access-secret-at-least-16-chars"
	refSecret = "refresh-secret-at-least-16-char"
)

type fakeRepo struct {
	byEmail map[string]*AdminUser
	byID    map[string]*AdminUser
	newHash string
}

func (f *fakeRepo) FindByEmailLower(_ context.Context, email string) (*AdminUser, error) {
	return f.byEmail[email], nil
}
func (f *fakeRepo) FindByID(_ context.Context, id string) (*AdminUser, error) { return f.byID[id], nil }
func (f *fakeRepo) UpdatePasswordHash(_ context.Context, _, hash string) error {
	f.newHash = hash
	return nil
}

func mustHash(t *testing.T, pw string) string {
	t.Helper()
	h, err := shared.HashPassword(pw)
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	return h
}

func newSvc(repo Repo, isProd bool) *Service {
	return NewService(repo, accSecret, refSecret, isProd)
}

func TestLogin_Success(t *testing.T) {
	admin := &AdminUser{ID: "a1", Email: "Admin@DraftRight.com", PasswordHash: mustHash(t, "pw"), Name: "Root", IsActive: true, Role: "admin"}
	svc := newSvc(&fakeRepo{byEmail: map[string]*AdminUser{"admin@draftright.com": admin}}, false)

	res, err := svc.Login(context.Background(), "  Admin@DraftRight.com  ", "pw")
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	if res.User.ID != "a1" || res.User.Email != "Admin@DraftRight.com" || res.User.Name != "Root" || res.User.Role != "admin" {
		t.Errorf("user = %+v", res.User)
	}
	claims, err := platauth.NewVerifier(accSecret).Verify(res.AccessToken)
	if err != nil {
		t.Fatalf("verify access: %v", err)
	}
	if claims.Sub != "a1" || claims.Role != "admin" || !claims.IsAdminFlag {
		t.Errorf("access claims = %+v", claims)
	}
	if _, err := platauth.NewVerifier(refSecret).Verify(res.RefreshToken); err != nil {
		t.Fatalf("verify refresh: %v", err)
	}
	if _, err := platauth.NewVerifier(accSecret).Verify(res.RefreshToken); err == nil {
		t.Error("refresh token verified with access secret — secrets not separated")
	}
}

func TestLogin_Errors(t *testing.T) {
	good := &AdminUser{ID: "a1", Email: "a@b.c", PasswordHash: mustHash(t, "pw"), IsActive: true, Role: "admin"}
	disabled := &AdminUser{ID: "a2", Email: "d@b.c", PasswordHash: mustHash(t, "pw"), IsActive: false, Role: "admin"}
	svc := newSvc(&fakeRepo{byEmail: map[string]*AdminUser{"a@b.c": good, "d@b.c": disabled}}, false)

	cases := []struct {
		name, email, pw string
		want            *Error
	}{
		{"no user", "ghost@b.c", "pw", ErrInvalidCredentials},
		{"bad password", "a@b.c", "wrong", ErrInvalidCredentials},
		{"disabled", "d@b.c", "pw", ErrAccountDisabled},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := svc.Login(context.Background(), tc.email, tc.pw)
			if err != tc.want {
				t.Errorf("err = %v, want %v", err, tc.want)
			}
		})
	}
}
