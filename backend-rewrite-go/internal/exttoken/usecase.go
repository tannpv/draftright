package exttoken

import "context"

// Store is the persistence port (satisfied by the concrete *Repo in
// repo_pg.go). Named Store (not Repo) because repo_pg.go already owns the
// concrete type name Repo — accept interface, return struct.
type Store interface {
	RevokeActiveForDevice(ctx context.Context, userID, deviceID string) error
	Insert(ctx context.Context, userID, tokenHash, deviceID, deviceName string, scopes []string) (TokenRow, error)
	ListActive(ctx context.Context, userID string) ([]TokenRow, error)
	RevokeByID(ctx context.Context, id, userID string) error
	FindActiveByHash(ctx context.Context, hash string) (*ActiveToken, error)
	TouchLastUsed(ctx context.Context, id string) error
}

// Service is the extension-token use case.
type Service struct {
	repo Store
	// gen lets tests inject a deterministic token; prod uses generateToken.
	gen func() (plain, hash string, err error)
}

// NewService wires the persistence port; production token generation.
func NewService(repo Store) *Service {
	return &Service{repo: repo, gen: generateToken}
}

// NewServiceWithGen is exported for deterministic tests — it injects a fake
// token generator so Mint's output is predictable. Production code uses
// NewService (which wires the real generateToken).
func NewServiceWithGen(repo Store, gen func() (plain, hash string, err error)) *Service {
	return &Service{repo: repo, gen: gen}
}

// Mint rotates any active token for (userID, deviceID), then issues a fresh
// dr_ext_ token scoped ['rewrite']. Returns the plaintext ONCE.
//
// Order mirrors Node extension-token.service.ts mint(): revoke the active
// (user, device) token FIRST (so the partial unique index doesn't collide),
// then insert the new row.
func (s *Service) Mint(ctx context.Context, userID, deviceID, deviceName string) (MintResult, error) {
	if err := s.repo.RevokeActiveForDevice(ctx, userID, deviceID); err != nil {
		return MintResult{}, err
	}
	plain, hash, err := s.gen()
	if err != nil {
		return MintResult{}, err
	}
	row, err := s.repo.Insert(ctx, userID, hash, deviceID, deviceName, []string{ScopeRewrite})
	if err != nil {
		return MintResult{}, err
	}
	return MintResult{Token: plain, ID: row.ID}, nil
}

// List returns the user's active tokens (no plaintext).
func (s *Service) List(ctx context.Context, userID string) ([]TokenRow, error) {
	return s.repo.ListActive(ctx, userID)
}

// Revoke is idempotent — always succeeds even if id is unknown/already revoked
// (matches Node: no revoked_at filter, controller returns 204 regardless).
func (s *Service) Revoke(ctx context.Context, id, userID string) error {
	return s.repo.RevokeByID(ctx, id, userID)
}

// Verify resolves a presented bearer token to its owner userID. Maps to the
// three sentinels mirroring Node's RewriteAuthGuard: empty → ErrMissingToken;
// unknown/inactive → ErrInvalidToken; lacks 'rewrite' → ErrMissingScope. On
// success it fires TouchLastUsed (best-effort write-behind; its error is
// swallowed, matching Node's .catch(() => undefined)).
func (s *Service) Verify(ctx context.Context, raw string) (string, error) {
	if raw == "" {
		return "", ErrMissingToken
	}
	at, err := s.repo.FindActiveByHash(ctx, hashToken(raw))
	if err != nil {
		return "", err
	}
	if at == nil {
		return "", ErrInvalidToken
	}
	if !hasScope(at.Scopes, ScopeRewrite) {
		return "", ErrMissingScope
	}
	_ = s.repo.TouchLastUsed(ctx, at.ID) // best-effort; ignore error
	return at.UserID, nil
}

func hasScope(scopes []string, want string) bool {
	for _, sc := range scopes {
		if sc == want {
			return true
		}
	}
	return false
}
