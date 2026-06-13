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
