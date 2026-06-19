package user_test

import (
	"context"
	"testing"

	"github.com/tannpv/draftright-rewrite/internal/user"
)

type fakeRepo struct {
	byEmail map[string]user.User
	updated map[string]string
	deleted []string

	lastPatch    user.UserPatch
	created      []user.NewUser
	state        map[string]user.AuthState
	bySocial     map[string]user.User
	updateCalled bool
}

func (f *fakeRepo) ByEmail(_ context.Context, e string) (user.User, error) {
	u, ok := f.byEmail[e]
	if !ok {
		return user.User{}, user.ErrNotFound
	}
	return u, nil
}
func (f *fakeRepo) ByID(_ context.Context, id string) (user.User, error) {
	for _, u := range f.byEmail {
		if u.ID == id {
			return u, nil
		}
	}
	return user.User{}, user.ErrNotFound
}
func (f *fakeRepo) UpdatePasswordHash(_ context.Context, id, h string) error {
	if f.updated == nil {
		f.updated = map[string]string{}
	}
	f.updated[id] = h
	return nil
}
func (f *fakeRepo) DeleteAccount(_ context.Context, id string) error {
	f.deleted = append(f.deleted, id)
	return nil
}
func (f *fakeRepo) Create(_ context.Context, in user.NewUser) (user.User, error) {
	f.created = append(f.created, in)
	return user.User{ID: "new", Email: in.Email, Name: in.Name}, nil
}
func (f *fakeRepo) Update(_ context.Context, _ string, p user.UserPatch) error {
	f.updateCalled = true
	f.lastPatch = p
	return nil
}
func (f *fakeRepo) FindBySocialId(_ context.Context, provider, socialID string) (user.User, error) {
	u, ok := f.bySocial[provider+"|"+socialID]
	if !ok {
		return user.User{}, user.ErrNotFound
	}
	return u, nil
}
func (f *fakeRepo) AuthState(_ context.Context, email string) (user.AuthState, error) {
	st, ok := f.state[email]
	if !ok {
		return user.AuthState{}, user.ErrNotFound
	}
	return st, nil
}

func TestService_ByEmail_HitAndMiss(t *testing.T) {
	r := &fakeRepo{byEmail: map[string]user.User{"a@b.com": {ID: "u1", Email: "a@b.com"}}}
	s := user.NewService(r)
	if u, err := s.ByEmail(context.Background(), "a@b.com"); err != nil || u.ID != "u1" {
		t.Fatalf("hit failed: %v %v", u, err)
	}
	if _, err := s.ByEmail(context.Background(), "missing@b.com"); err != user.ErrNotFound {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
}

func TestService_ByID_HitAndMiss(t *testing.T) {
	r := &fakeRepo{byEmail: map[string]user.User{"a@b.com": {ID: "u1", Email: "a@b.com"}}}
	s := user.NewService(r)
	if u, err := s.ByID(context.Background(), "u1"); err != nil || u.Email != "a@b.com" {
		t.Fatalf("hit failed: %v %v", u, err)
	}
	if _, err := s.ByID(context.Background(), "nope"); err != user.ErrNotFound {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
}

func TestService_Create_Passthrough(t *testing.T) {
	r := &fakeRepo{byEmail: map[string]user.User{}}
	s := user.NewService(r)
	u, err := s.Create(context.Background(), user.NewUser{Email: "n@b.com", Name: "N", PasswordHash: "h"})
	if err != nil || u.Email != "n@b.com" {
		t.Fatalf("create: %v %v", u, err)
	}
}

func TestService_Update_Passthrough(t *testing.T) {
	r := &fakeRepo{byEmail: map[string]user.User{}}
	s := user.NewService(r)
	on := true
	if err := s.Update(context.Background(), "u1", user.UserPatch{EmailVerified: &on}); err != nil {
		t.Fatal(err)
	}
	if r.lastPatch.EmailVerified == nil || !*r.lastPatch.EmailVerified {
		t.Fatal("patch not forwarded")
	}
}

func TestService_FindBySocialId(t *testing.T) {
	r := &fakeRepo{bySocial: map[string]user.User{"google|sid1": {ID: "u1", Email: "g@b.com"}}}
	s := user.NewService(r)
	u, err := s.FindBySocialId(context.Background(), "google", "sid1")
	if err != nil || u.ID != "u1" {
		t.Fatalf("got %v %v", u, err)
	}
	if _, err := s.FindBySocialId(context.Background(), "google", "nope"); err != user.ErrNotFound {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
}

func TestService_UpdatePasswordHash(t *testing.T) {
	r := &fakeRepo{byEmail: map[string]user.User{}}
	s := user.NewService(r)
	if err := s.UpdatePasswordHash(context.Background(), "u1", "newhash"); err != nil {
		t.Fatal(err)
	}
	if r.updated["u1"] != "newhash" {
		t.Fatalf("hash not updated: %v", r.updated)
	}
}
