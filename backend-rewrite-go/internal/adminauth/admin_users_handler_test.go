package adminauth

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/tannpv/draftright-rewrite/internal/shared/listquery"
)

// fakeAdminUsersRepo satisfies the admin-users usecase's adminUsersRepo port.
// It captures the NewAdminUser passed to Insert (so create tests can assert the
// hashed password + role default) and records which list branch ran for the
// dual-mode tests. emailExists drives the duplicate-email path.
type fakeAdminUsersRepo struct {
	all           []AdminUserOut
	paginated     []AdminUserOut
	total         int
	emailExists   bool
	inserted      NewAdminUser   // captured Insert arg
	patched       AdminUserPatch // captured Update arg
	builtSeen     listquery.Built
	calledListAll bool
	calledPaginat bool
}

func (f *fakeAdminUsersRepo) ListAll(ctx context.Context) ([]AdminUserOut, error) {
	f.calledListAll = true
	return f.all, nil
}

func (f *fakeAdminUsersRepo) ListPaginated(ctx context.Context, b listquery.Built) ([]AdminUserOut, int, error) {
	f.calledPaginat = true
	f.builtSeen = b
	return f.paginated, f.total, nil
}

func (f *fakeAdminUsersRepo) EmailExists(ctx context.Context, email string) (bool, error) {
	return f.emailExists, nil
}

func (f *fakeAdminUsersRepo) Insert(ctx context.Context, in NewAdminUser) (AdminUserOut, error) {
	f.inserted = in
	return AdminUserOut{ID: "new-id", Email: in.Email, Name: in.Name, Role: in.Role, IsActive: true}, nil
}

func (f *fakeAdminUsersRepo) Update(ctx context.Context, id string, p AdminUserPatch) (AdminUserOut, error) {
	f.patched = p
	return AdminUserOut{ID: id}, nil
}

func (f *fakeAdminUsersRepo) SoftDelete(ctx context.Context, id string) error { return nil }

// assertAdminUserKeyOrder fails unless the JSON keys appear in raw in the given
// order.
func assertAdminUserKeyOrder(t *testing.T, raw string, keys ...string) {
	t.Helper()
	prev := -1
	for _, k := range keys {
		idx := strings.Index(raw, `"`+k+`"`)
		if idx < 0 {
			t.Fatalf("key %q missing from body: %s", k, raw)
		}
		if idx <= prev {
			t.Fatalf("key %q out of order in body: %s", k, raw)
		}
		prev = idx
	}
}

func newAdminUsersHandler(repo *fakeAdminUsersRepo) *AdminUsersHandler {
	return NewAdminUsersHandler(NewAdminUsersService(repo))
}

// TestCreateAdminUser_201_DupEmail400: POST creates → 201 with password_hash
// stripped + role defaulted to "admin"; a duplicate email → 400 invalid-input
// with the Node message "Email already exists".
func TestCreateAdminUser_201_DupEmail400(t *testing.T) {
	// Success: 201, password_hash never in body, role default "admin".
	repo := &fakeAdminUsersRepo{}
	h := newAdminUsersHandler(repo)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/admin/admin-users",
		strings.NewReader(`{"email":"a@b.com","password":"secret123","name":"Al"}`))
	h.Create(rec, req)

	if rec.Code != 201 {
		t.Fatalf("status = %d, want 201; body=%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if strings.Contains(body, "password_hash") {
		t.Fatalf("body leaked password_hash: %s", body)
	}
	if repo.inserted.Role != "admin" {
		t.Fatalf("role = %q, want \"admin\" (default)", repo.inserted.Role)
	}
	if repo.inserted.PasswordHash == "" || repo.inserted.PasswordHash == "secret123" {
		t.Fatalf("password not hashed: %q", repo.inserted.PasswordHash)
	}
	if repo.inserted.Email != "a@b.com" || repo.inserted.Name != "Al" {
		t.Fatalf("insert args wrong: %+v", repo.inserted)
	}

	// Duplicate email: 400 invalid-input, message "Email already exists".
	dup := &fakeAdminUsersRepo{emailExists: true}
	dh := newAdminUsersHandler(dup)
	drec := httptest.NewRecorder()
	dreq := httptest.NewRequest(http.MethodPost, "/admin/admin-users",
		strings.NewReader(`{"email":"a@b.com","password":"secret123","name":"Al"}`))
	dh.Create(drec, dreq)

	if drec.Code != 400 {
		t.Fatalf("dup status = %d, want 400; body=%s", drec.Code, drec.Body.String())
	}
	var env struct {
		Error string `json:"error"`
		Code  string `json:"code"`
	}
	if err := json.Unmarshal(drec.Body.Bytes(), &env); err != nil {
		t.Fatalf("dup body not JSON: %v (%s)", err, drec.Body.String())
	}
	if env.Code != "invalid-input" {
		t.Fatalf("dup code = %q, want invalid-input", env.Code)
	}
	if env.Error != "Email already exists" {
		t.Fatalf("dup message = %q, want \"Email already exists\"", env.Error)
	}
}

// TestCreateAdminUser_RolePassthrough: an explicit role is passed verbatim, not
// overridden by the default.
func TestCreateAdminUser_RolePassthrough(t *testing.T) {
	repo := &fakeAdminUsersRepo{}
	h := newAdminUsersHandler(repo)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/admin/admin-users",
		strings.NewReader(`{"email":"a@b.com","password":"x","name":"Al","role":"superadmin"}`))
	h.Create(rec, req)

	if rec.Code != 201 {
		t.Fatalf("status = %d, want 201", rec.Code)
	}
	if repo.inserted.Role != "superadmin" {
		t.Fatalf("role = %q, want superadmin", repo.inserted.Role)
	}
}

// TestListAdminUsers_DualMode: no params → bare JSON array (ListAll, "[]" when
// empty); any of page/search/status/sort_by → { rows, total } via ListPaginated.
func TestListAdminUsers_DualMode(t *testing.T) {
	// No params → bare array branch.
	repo := &fakeAdminUsersRepo{all: []AdminUserOut{}}
	h := newAdminUsersHandler(repo)
	rec := httptest.NewRecorder()
	h.List(rec, httptest.NewRequest(http.MethodGet, "/admin/admin-users", nil))

	if rec.Code != 200 {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	if !repo.calledListAll || repo.calledPaginat {
		t.Fatalf("expected ListAll branch; listAll=%v paginated=%v", repo.calledListAll, repo.calledPaginat)
	}
	if got := strings.TrimSpace(rec.Body.String()); got != "[]" {
		t.Fatalf("body = %q, want %q", got, "[]")
	}

	// page param → paginated branch, { rows, total } key order.
	prepo := &fakeAdminUsersRepo{paginated: []AdminUserOut{}, total: 3}
	ph := newAdminUsersHandler(prepo)
	prec := httptest.NewRecorder()
	ph.List(prec, httptest.NewRequest(http.MethodGet, "/admin/admin-users?page=2", nil))

	if prec.Code != 200 {
		t.Fatalf("paginated status = %d, want 200; body=%s", prec.Code, prec.Body.String())
	}
	if !prepo.calledPaginat || prepo.calledListAll {
		t.Fatalf("expected ListPaginated branch; listAll=%v paginated=%v", prepo.calledListAll, prepo.calledPaginat)
	}
	raw := prec.Body.String()
	assertAdminUserKeyOrder(t, raw, "rows", "total")
	var pbody struct {
		Rows  []map[string]any `json:"rows"`
		Total int              `json:"total"`
	}
	if err := json.Unmarshal([]byte(raw), &pbody); err != nil {
		t.Fatalf("paginated body not JSON: %v (%s)", err, raw)
	}
	if pbody.Total != 3 {
		t.Fatalf("total = %d, want 3", pbody.Total)
	}
}

// TestListAdminUsers_Config: search/sort/status params produce a Built that
// reaches ListPaginated, with the admin-users sort allow-list honoured (role →
// role) and the status column being is_active.
func TestListAdminUsers_Config(t *testing.T) {
	repo := &fakeAdminUsersRepo{paginated: []AdminUserOut{}, total: 0}
	h := newAdminUsersHandler(repo)
	rec := httptest.NewRecorder()
	h.List(rec, httptest.NewRequest(http.MethodGet,
		"/admin/admin-users?search=al&sort_by=role&sort_order=asc&status=active", nil))

	if !repo.calledPaginat {
		t.Fatalf("expected ListPaginated branch")
	}
	if !strings.Contains(repo.builtSeen.Order, "role") {
		t.Fatalf("order %q does not contain 'role'", repo.builtSeen.Order)
	}
	if !strings.Contains(repo.builtSeen.Where, "is_active") {
		t.Fatalf("where %q does not filter on is_active (status column)", repo.builtSeen.Where)
	}

	// Sanity: the allow-list maps the expected public keys. created_at is the
	// default sort fallback.
	if adminUserSortAllow["role"] != "role" || adminUserSortAllow["created_at"] != "created_at" {
		t.Fatalf("sort allow-list wrong: %v", adminUserSortAllow)
	}
}

// TestUpdateAdminUser_PartialPatch: only provided keys reach the patch; an
// absent key stays nil; password (truthy) becomes a hashed PasswordHash, and
// password_hash is stripped from the 200 body.
func TestUpdateAdminUser_PartialPatch(t *testing.T) {
	repo := &fakeAdminUsersRepo{}
	h := newAdminUsersHandler(repo)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPatch, "/admin/admin-users/u1",
		strings.NewReader(`{"name":"New","password":"freshpass"}`))
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "u1")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	h.Update(rec, req)

	if rec.Code != 200 {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	if repo.patched.Name == nil || *repo.patched.Name != "New" {
		t.Fatalf("name not patched: %+v", repo.patched.Name)
	}
	if repo.patched.Email != nil || repo.patched.Role != nil || repo.patched.IsActive != nil {
		t.Fatalf("absent keys should be nil: %+v", repo.patched)
	}
	if repo.patched.PasswordHash == nil || *repo.patched.PasswordHash == "freshpass" || *repo.patched.PasswordHash == "" {
		t.Fatalf("password not hashed: %+v", repo.patched.PasswordHash)
	}
	if strings.Contains(rec.Body.String(), "password_hash") {
		t.Fatalf("body leaked password_hash: %s", rec.Body.String())
	}
}

// TestUpdateAdminUser_EmptyPasswordIgnored: an empty password must NOT set
// PasswordHash (Node `if (body.password)` truthy-guard).
func TestUpdateAdminUser_EmptyPasswordIgnored(t *testing.T) {
	repo := &fakeAdminUsersRepo{}
	h := newAdminUsersHandler(repo)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPatch, "/admin/admin-users/u1",
		strings.NewReader(`{"role":"admin","password":""}`))
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "u1")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	h.Update(rec, req)

	if rec.Code != 200 {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if repo.patched.PasswordHash != nil {
		t.Fatalf("empty password must not set PasswordHash: %+v", repo.patched.PasswordHash)
	}
	if repo.patched.Role == nil || *repo.patched.Role != "admin" {
		t.Fatalf("role not patched: %+v", repo.patched.Role)
	}
}

// TestUpdateAdminUser_NotFound500: a missing id (repo ErrAdminNotFound) maps to
// 500 internal (Node findOneOrFail → AllExceptionsFilter), NOT a 404.
func TestUpdateAdminUser_NotFound500(t *testing.T) {
	h := NewAdminUsersHandler(NewAdminUsersService(&notFoundRepo{}))
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPatch, "/admin/admin-users/missing",
		strings.NewReader(`{"name":"x"}`))
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "missing")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	h.Update(rec, req)

	if rec.Code != 500 {
		t.Fatalf("status = %d, want 500; body=%s", rec.Code, rec.Body.String())
	}
}

// notFoundRepo returns ErrAdminNotFound from Update to exercise the 500 path.
type notFoundRepo struct{ fakeAdminUsersRepo }

func (n *notFoundRepo) Update(ctx context.Context, id string, p AdminUserPatch) (AdminUserOut, error) {
	return AdminUserOut{}, ErrAdminNotFound
}

// TestDeleteAdminUser_SuccessBody: DELETE → 200 { "success": true }.
func TestDeleteAdminUser_SuccessBody(t *testing.T) {
	h := newAdminUsersHandler(&fakeAdminUsersRepo{})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodDelete, "/admin/admin-users/x", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "x")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	h.Delete(rec, req)

	if rec.Code != 200 {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	if got := strings.TrimSpace(rec.Body.String()); got != `{"success":true}` {
		t.Fatalf("body = %q, want %q", got, `{"success":true}`)
	}
}
