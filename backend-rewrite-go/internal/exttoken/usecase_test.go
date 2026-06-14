package exttoken_test

import (
	"context"
	"errors"
	"testing"

	"github.com/tannpv/draftright-rewrite/internal/exttoken"
)

// fakeRepo records calls in order and returns canned values / errors.
type fakeRepo struct {
	calls []string

	revokeDeviceErr error

	insertRow exttoken.TokenRow
	insertErr error

	listRows []exttoken.TokenRow
	listErr  error

	revokeByIDErr error

	findActive *exttoken.ActiveToken
	findErr    error

	touchErr error
}

func (f *fakeRepo) RevokeActiveForDevice(ctx context.Context, userID, deviceID string) error {
	f.calls = append(f.calls, "RevokeActiveForDevice")
	return f.revokeDeviceErr
}

func (f *fakeRepo) Insert(ctx context.Context, userID, tokenHash, deviceID, deviceName string, scopes []string) (exttoken.TokenRow, error) {
	f.calls = append(f.calls, "Insert")
	return f.insertRow, f.insertErr
}

func (f *fakeRepo) ListActive(ctx context.Context, userID string) ([]exttoken.TokenRow, error) {
	f.calls = append(f.calls, "ListActive")
	return f.listRows, f.listErr
}

func (f *fakeRepo) RevokeByID(ctx context.Context, id, userID string) error {
	f.calls = append(f.calls, "RevokeByID")
	return f.revokeByIDErr
}

func (f *fakeRepo) FindActiveByHash(ctx context.Context, hash string) (*exttoken.ActiveToken, error) {
	f.calls = append(f.calls, "FindActiveByHash")
	return f.findActive, f.findErr
}

func (f *fakeRepo) TouchLastUsed(ctx context.Context, id string) error {
	f.calls = append(f.calls, "TouchLastUsed")
	return f.touchErr
}

func eq(t *testing.T, got, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("call order = %v, want %v", got, want)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("call order = %v, want %v", got, want)
		}
	}
}

func TestMint_RevokesThenInserts(t *testing.T) {
	repo := &fakeRepo{insertRow: exttoken.TokenRow{ID: "row-id-1"}}
	svc := exttoken.NewServiceWithGen(repo, func() (string, string, error) {
		return "dr_ext_PLAIN", "HASH", nil
	})

	res, err := svc.Mint(context.Background(), "user-1", "device-1", "iPhone")
	if err != nil {
		t.Fatalf("Mint err = %v", err)
	}
	if res.Token != "dr_ext_PLAIN" {
		t.Fatalf("Token = %q, want dr_ext_PLAIN", res.Token)
	}
	if res.ID != "row-id-1" {
		t.Fatalf("ID = %q, want row-id-1", res.ID)
	}
	eq(t, repo.calls, []string{"RevokeActiveForDevice", "Insert"})
}

func TestMint_RevokeError_NoInsert(t *testing.T) {
	sentinel := errors.New("revoke boom")
	repo := &fakeRepo{revokeDeviceErr: sentinel}
	svc := exttoken.NewServiceWithGen(repo, func() (string, string, error) {
		return "dr_ext_PLAIN", "HASH", nil
	})

	_, err := svc.Mint(context.Background(), "u", "d", "n")
	if !errors.Is(err, sentinel) {
		t.Fatalf("err = %v, want %v", err, sentinel)
	}
	eq(t, repo.calls, []string{"RevokeActiveForDevice"})
}

func TestMint_GenError(t *testing.T) {
	sentinel := errors.New("gen boom")
	repo := &fakeRepo{}
	svc := exttoken.NewServiceWithGen(repo, func() (string, string, error) {
		return "", "", sentinel
	})

	_, err := svc.Mint(context.Background(), "u", "d", "n")
	if !errors.Is(err, sentinel) {
		t.Fatalf("err = %v, want %v", err, sentinel)
	}
	eq(t, repo.calls, []string{"RevokeActiveForDevice"})
}

func TestMint_InsertError(t *testing.T) {
	sentinel := errors.New("insert boom")
	repo := &fakeRepo{insertErr: sentinel}
	svc := exttoken.NewServiceWithGen(repo, func() (string, string, error) {
		return "dr_ext_PLAIN", "HASH", nil
	})

	_, err := svc.Mint(context.Background(), "u", "d", "n")
	if !errors.Is(err, sentinel) {
		t.Fatalf("err = %v, want %v", err, sentinel)
	}
	eq(t, repo.calls, []string{"RevokeActiveForDevice", "Insert"})
}

func TestList_PassThrough(t *testing.T) {
	rows := []exttoken.TokenRow{{ID: "a"}, {ID: "b"}}
	repo := &fakeRepo{listRows: rows}
	svc := exttoken.NewService(repo)

	got, err := svc.List(context.Background(), "user-1")
	if err != nil {
		t.Fatalf("List err = %v", err)
	}
	if len(got) != 2 || got[0].ID != "a" || got[1].ID != "b" {
		t.Fatalf("List rows = %v, want %v", got, rows)
	}
	eq(t, repo.calls, []string{"ListActive"})
}

func TestList_Error(t *testing.T) {
	sentinel := errors.New("list boom")
	repo := &fakeRepo{listErr: sentinel}
	svc := exttoken.NewService(repo)

	_, err := svc.List(context.Background(), "u")
	if !errors.Is(err, sentinel) {
		t.Fatalf("err = %v, want %v", err, sentinel)
	}
}

func TestRevoke_PassThrough_Idempotent(t *testing.T) {
	repo := &fakeRepo{} // returns nil even for unknown id
	svc := exttoken.NewService(repo)

	if err := svc.Revoke(context.Background(), "unknown-id", "user-1"); err != nil {
		t.Fatalf("Revoke err = %v, want nil", err)
	}
	eq(t, repo.calls, []string{"RevokeByID"})
}

func TestVerify_EmptyRaw_MissingToken_NoRepoCall(t *testing.T) {
	repo := &fakeRepo{}
	svc := exttoken.NewService(repo)

	_, err := svc.Verify(context.Background(), "")
	if !errors.Is(err, exttoken.ErrMissingToken) {
		t.Fatalf("err = %v, want ErrMissingToken", err)
	}
	if len(repo.calls) != 0 {
		t.Fatalf("expected no repo calls, got %v", repo.calls)
	}
}

func TestVerify_UnknownHash_InvalidToken(t *testing.T) {
	repo := &fakeRepo{findActive: nil} // FindActiveByHash → nil, nil
	svc := exttoken.NewService(repo)

	_, err := svc.Verify(context.Background(), "dr_ext_whatever")
	if !errors.Is(err, exttoken.ErrInvalidToken) {
		t.Fatalf("err = %v, want ErrInvalidToken", err)
	}
	eq(t, repo.calls, []string{"FindActiveByHash"})
}

func TestVerify_FindError_Propagates(t *testing.T) {
	sentinel := errors.New("find boom")
	repo := &fakeRepo{findErr: sentinel}
	svc := exttoken.NewService(repo)

	_, err := svc.Verify(context.Background(), "dr_ext_whatever")
	if !errors.Is(err, sentinel) {
		t.Fatalf("err = %v, want %v", err, sentinel)
	}
}

func TestVerify_MissingScope(t *testing.T) {
	repo := &fakeRepo{findActive: &exttoken.ActiveToken{
		ID:     "tok-1",
		UserID: "user-1",
		Scopes: []string{"other"},
	}}
	svc := exttoken.NewService(repo)

	_, err := svc.Verify(context.Background(), "dr_ext_whatever")
	if !errors.Is(err, exttoken.ErrMissingScope) {
		t.Fatalf("err = %v, want ErrMissingScope", err)
	}
	// No last-used touch on the scope-reject path.
	eq(t, repo.calls, []string{"FindActiveByHash"})
}

func TestVerify_Valid_ReturnsUserID_FiresTouch(t *testing.T) {
	repo := &fakeRepo{findActive: &exttoken.ActiveToken{
		ID:     "tok-1",
		UserID: "user-1",
		Scopes: []string{exttoken.ScopeRewrite},
	}}
	svc := exttoken.NewService(repo)

	uid, err := svc.Verify(context.Background(), "dr_ext_whatever")
	if err != nil {
		t.Fatalf("Verify err = %v", err)
	}
	if uid != "user-1" {
		t.Fatalf("uid = %q, want user-1", uid)
	}
	eq(t, repo.calls, []string{"FindActiveByHash", "TouchLastUsed"})
}

func TestVerify_Valid_TouchErrorSwallowed(t *testing.T) {
	repo := &fakeRepo{
		findActive: &exttoken.ActiveToken{
			ID:     "tok-1",
			UserID: "user-1",
			Scopes: []string{exttoken.ScopeRewrite},
		},
		touchErr: errors.New("touch boom"),
	}
	svc := exttoken.NewService(repo)

	uid, err := svc.Verify(context.Background(), "dr_ext_whatever")
	if err != nil {
		t.Fatalf("Verify err = %v, want nil (touch error swallowed)", err)
	}
	if uid != "user-1" {
		t.Fatalf("uid = %q, want user-1", uid)
	}
	eq(t, repo.calls, []string{"FindActiveByHash", "TouchLastUsed"})
}
