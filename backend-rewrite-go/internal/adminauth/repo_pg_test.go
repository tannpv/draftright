package adminauth

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	sqlc "github.com/tannpv/draftright-rewrite/internal/shared/pg/sqlc"
)

type fakeQ struct {
	byEmail map[string]sqlc.AdminUser
	byID    map[string]sqlc.AdminUser
	updated map[string]string // id -> new hash
}

func (f *fakeQ) FindAdminByEmailLower(_ context.Context, email string) (sqlc.AdminUser, error) {
	if u, ok := f.byEmail[email]; ok {
		return u, nil
	}
	return sqlc.AdminUser{}, pgx.ErrNoRows
}
func (f *fakeQ) FindAdminByID(_ context.Context, id pgtype.UUID) (sqlc.AdminUser, error) {
	if u, ok := f.byID[uuidStr(id)]; ok {
		return u, nil
	}
	return sqlc.AdminUser{}, pgx.ErrNoRows
}
func (f *fakeQ) UpdateAdminPasswordHash(_ context.Context, arg sqlc.UpdateAdminPasswordHashParams) error {
	if f.updated == nil {
		f.updated = map[string]string{}
	}
	f.updated[uuidStr(arg.ID)] = arg.PasswordHash
	return nil
}

func mkUUID(s string) pgtype.UUID {
	var u pgtype.UUID
	_ = u.Scan(s)
	return u
}

func sampleRow() sqlc.AdminUser {
	return sqlc.AdminUser{
		ID:           mkUUID("11111111-1111-1111-1111-111111111111"),
		Email:        "Admin@DraftRight.com",
		PasswordHash: "$2a$10$hash",
		Name:         "Root",
		IsActive:     true,
		Role:         "admin",
		CreatedAt:    pgtype.Timestamp{Time: time.Unix(1700000000, 0).UTC(), Valid: true},
		UpdatedAt:    pgtype.Timestamp{Time: time.Unix(1700000001, 0).UTC(), Valid: true},
	}
}

func TestRepo_FindByEmailLower(t *testing.T) {
	row := sampleRow()
	r := NewPgRepo(&fakeQ{byEmail: map[string]sqlc.AdminUser{"admin@draftright.com": row}})

	got, err := r.FindByEmailLower(context.Background(), "admin@draftright.com")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if got == nil || got.ID != "11111111-1111-1111-1111-111111111111" || got.Email != "Admin@DraftRight.com" ||
		got.Name != "Root" || !got.IsActive || got.Role != "admin" || got.PasswordHash != "$2a$10$hash" {
		t.Errorf("mapped = %+v", got)
	}
	if !got.CreatedAt.Equal(time.Unix(1700000000, 0).UTC()) {
		t.Errorf("CreatedAt = %v", got.CreatedAt)
	}
}

func TestRepo_FindByEmailLower_NotFound(t *testing.T) {
	r := NewPgRepo(&fakeQ{})
	got, err := r.FindByEmailLower(context.Background(), "nobody@x.com")
	if err != nil || got != nil {
		t.Errorf("got (%v,%v), want (nil,nil)", got, err)
	}
}

func TestRepo_FindByID_NotFound(t *testing.T) {
	r := NewPgRepo(&fakeQ{})
	got, err := r.FindByID(context.Background(), "11111111-1111-1111-1111-111111111111")
	if err != nil || got != nil {
		t.Errorf("got (%v,%v), want (nil,nil)", got, err)
	}
}

func TestRepo_UpdatePasswordHash(t *testing.T) {
	q := &fakeQ{}
	r := NewPgRepo(q)
	if err := r.UpdatePasswordHash(context.Background(), "11111111-1111-1111-1111-111111111111", "$2b$10$new"); err != nil {
		t.Fatalf("err: %v", err)
	}
	if q.updated["11111111-1111-1111-1111-111111111111"] != "$2b$10$new" {
		t.Errorf("updated = %v", q.updated)
	}
}
