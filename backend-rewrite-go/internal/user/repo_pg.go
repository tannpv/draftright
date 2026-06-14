package user

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	sqlc "github.com/tannpv/draftright-rewrite/internal/shared/pg/sqlc"
)

// pgQuerier is the sqlc subset the read/update methods need.
type pgQuerier interface {
	GetAuthUserByEmail(ctx context.Context, email string) (sqlc.GetAuthUserByEmailRow, error)
	GetAuthUserByID(ctx context.Context, id pgtype.UUID) (sqlc.GetAuthUserByIDRow, error)
	UpdateUserPasswordHash(ctx context.Context, arg sqlc.UpdateUserPasswordHashParams) error
	CreateUser(ctx context.Context, arg sqlc.CreateUserParams) (sqlc.CreateUserRow, error)
	GetUserAuthState(ctx context.Context, email string) (sqlc.GetUserAuthStateRow, error)
	UpdateUserVerification(ctx context.Context, arg sqlc.UpdateUserVerificationParams) error
	SetEmailVerificationCode(ctx context.Context, arg sqlc.SetEmailVerificationCodeParams) error
	SetPasswordResetCode(ctx context.Context, arg sqlc.SetPasswordResetCodeParams) error
	SetPasswordResetAttempts(ctx context.Context, arg sqlc.SetPasswordResetAttemptsParams) error
	ResetPasswordHash(ctx context.Context, arg sqlc.ResetPasswordHashParams) error
	FindUserByGoogleId(ctx context.Context, googleID *string) (sqlc.FindUserByGoogleIdRow, error)
	FindUserByFacebookId(ctx context.Context, facebookID *string) (sqlc.FindUserByFacebookIdRow, error)
	FindUserByTiktokId(ctx context.Context, tiktokID *string) (sqlc.FindUserByTiktokIdRow, error)
	FindUserByAppleId(ctx context.Context, appleID *string) (sqlc.FindUserByAppleIdRow, error)
	LinkSocialGoogle(ctx context.Context, arg sqlc.LinkSocialGoogleParams) error
	LinkSocialFacebook(ctx context.Context, arg sqlc.LinkSocialFacebookParams) error
	LinkSocialTiktok(ctx context.Context, arg sqlc.LinkSocialTiktokParams) error
	LinkSocialApple(ctx context.Context, arg sqlc.LinkSocialAppleParams) error
}

// PgRepo implements Repo over Postgres. The delete-cascade txn needs the
// pool directly (multi-statement), so PgRepo holds both.
type PgRepo struct {
	q    pgQuerier
	pool DeleteExecer
}

// NewPgRepo wires the sqlc querier + a transaction-capable executor.
func NewPgRepo(q pgQuerier, pool DeleteExecer) *PgRepo { return &PgRepo{q: q, pool: pool} }

func (r *PgRepo) ByEmail(ctx context.Context, email string) (User, error) {
	row, err := r.q.GetAuthUserByEmail(ctx, email)
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, ErrNotFound
	}
	if err != nil {
		return User{}, err
	}
	return User{
		ID: uuidStr(row.ID), Email: row.Email, PasswordHash: strOrEmpty(row.PasswordHash),
		Name: row.Name, IsActive: row.IsActive, Role: string(row.Role),
		AuthProvider: string(row.AuthProvider), EmailVerified: row.EmailVerified,
		LemonsqueezyCustomer: strOrEmpty(row.LemonsqueezyCustomerID),
	}, nil
}

func (r *PgRepo) ByID(ctx context.Context, id string) (User, error) {
	uid, err := parseUUID(id)
	if err != nil {
		return User{}, ErrNotFound
	}
	row, err := r.q.GetAuthUserByID(ctx, uid)
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, ErrNotFound
	}
	if err != nil {
		return User{}, err
	}
	return User{
		ID: uuidStr(row.ID), Email: row.Email, PasswordHash: strOrEmpty(row.PasswordHash),
		Name: row.Name, IsActive: row.IsActive, Role: string(row.Role),
		AuthProvider: string(row.AuthProvider), EmailVerified: row.EmailVerified,
		LemonsqueezyCustomer: strOrEmpty(row.LemonsqueezyCustomerID),
	}, nil
}

func (r *PgRepo) UpdatePasswordHash(ctx context.Context, id, hash string) error {
	uid, err := parseUUID(id)
	if err != nil {
		return ErrNotFound
	}
	return r.q.UpdateUserPasswordHash(ctx, sqlc.UpdateUserPasswordHashParams{
		ID: uid, PasswordHash: &hash,
	})
}

// Create inserts a new user. AuthProvider defaults to "local"; exactly one
// social *_id column is set based on SocialProvider.
func (r *PgRepo) Create(ctx context.Context, in NewUser) (User, error) {
	ap := in.AuthProvider
	if ap == "" {
		ap = "local"
	}
	args := sqlc.CreateUserParams{
		Email: in.Email, PasswordHash: ptr(in.PasswordHash), Name: in.Name,
		AuthProvider: sqlc.UsersAuthProviderEnum(ap), AvatarUrl: ptr(in.AvatarURL),
		EmailVerified:            in.EmailVerified,
		EmailVerificationCode:    ptr(in.EmailVerificationCode),
		EmailVerificationExpires: ts(in.EmailVerificationExpires),
	}
	switch in.SocialProvider {
	case "google":
		args.GoogleID = ptr(in.SocialID)
	case "facebook":
		args.FacebookID = ptr(in.SocialID)
	case "tiktok":
		args.TiktokID = ptr(in.SocialID)
	case "apple":
		args.AppleID = ptr(in.SocialID)
	}
	row, err := r.q.CreateUser(ctx, args)
	if err != nil {
		return User{}, err
	}
	return User{ID: uuidStr(row.ID), Email: row.Email, Name: row.Name,
		AvatarURL: strOrEmpty(row.AvatarUrl), EmailVerified: row.EmailVerified}, nil
}

// Update applies a partial patch, dispatching to the narrowest sqlc query
// that covers the set fields.
func (r *PgRepo) Update(ctx context.Context, id string, p UserPatch) error {
	uid, err := parseUUID(id)
	if err != nil {
		return ErrNotFound
	}
	switch {
	case p.EmailVerified != nil && p.EmailVerificationCode.Set:
		return r.q.UpdateUserVerification(ctx, sqlc.UpdateUserVerificationParams{
			ID: uid, EmailVerified: *p.EmailVerified,
			EmailVerificationCode:    p.EmailVerificationCode.Value,
			EmailVerificationExpires: ts(p.EmailVerificationExpires.Value),
		})
	case p.EmailVerificationCode.Set:
		return r.q.SetEmailVerificationCode(ctx, sqlc.SetEmailVerificationCodeParams{
			ID: uid, EmailVerificationCode: p.EmailVerificationCode.Value,
			EmailVerificationExpires: ts(p.EmailVerificationExpires.Value),
		})
	case p.PasswordResetCode.Set:
		var att int32
		if p.PasswordResetAttempts != nil {
			att = int32(*p.PasswordResetAttempts)
		}
		return r.q.SetPasswordResetCode(ctx, sqlc.SetPasswordResetCodeParams{
			ID: uid, PasswordResetCode: p.PasswordResetCode.Value,
			PasswordResetExpires:  ts(p.PasswordResetExpires.Value),
			PasswordResetAttempts: att,
		})
	case p.PasswordResetAttempts != nil:
		return r.q.SetPasswordResetAttempts(ctx, sqlc.SetPasswordResetAttemptsParams{
			ID: uid, PasswordResetAttempts: int32(*p.PasswordResetAttempts),
		})
	case p.PasswordHash != nil:
		return r.q.ResetPasswordHash(ctx, sqlc.ResetPasswordHashParams{
			ID: uid, PasswordHash: p.PasswordHash,
		})
	case p.SocialProvider != "":
		return r.linkSocial(ctx, uid, p) // implemented in Part B
	}
	return nil
}

// FindBySocialId looks a user up by one of the four social provider id
// columns. Unknown provider or no row → ErrNotFound.
func (r *PgRepo) FindBySocialId(ctx context.Context, provider, socialID string) (User, error) {
	switch provider {
	case "google":
		row, err := r.q.FindUserByGoogleId(ctx, ptr(socialID))
		if errors.Is(err, pgx.ErrNoRows) {
			return User{}, ErrNotFound
		}
		if err != nil {
			return User{}, err
		}
		return socialRowToUser(row.ID, row.Email, row.PasswordHash, row.Name, row.IsActive,
			string(row.Role), string(row.AuthProvider), row.EmailVerified,
			row.LemonsqueezyCustomerID, row.AvatarUrl), nil
	case "facebook":
		row, err := r.q.FindUserByFacebookId(ctx, ptr(socialID))
		if errors.Is(err, pgx.ErrNoRows) {
			return User{}, ErrNotFound
		}
		if err != nil {
			return User{}, err
		}
		return socialRowToUser(row.ID, row.Email, row.PasswordHash, row.Name, row.IsActive,
			string(row.Role), string(row.AuthProvider), row.EmailVerified,
			row.LemonsqueezyCustomerID, row.AvatarUrl), nil
	case "tiktok":
		row, err := r.q.FindUserByTiktokId(ctx, ptr(socialID))
		if errors.Is(err, pgx.ErrNoRows) {
			return User{}, ErrNotFound
		}
		if err != nil {
			return User{}, err
		}
		return socialRowToUser(row.ID, row.Email, row.PasswordHash, row.Name, row.IsActive,
			string(row.Role), string(row.AuthProvider), row.EmailVerified,
			row.LemonsqueezyCustomerID, row.AvatarUrl), nil
	case "apple":
		row, err := r.q.FindUserByAppleId(ctx, ptr(socialID))
		if errors.Is(err, pgx.ErrNoRows) {
			return User{}, ErrNotFound
		}
		if err != nil {
			return User{}, err
		}
		return socialRowToUser(row.ID, row.Email, row.PasswordHash, row.Name, row.IsActive,
			string(row.Role), string(row.AuthProvider), row.EmailVerified,
			row.LemonsqueezyCustomerID, row.AvatarUrl), nil
	default:
		return User{}, ErrNotFound
	}
}

// socialRowToUser maps the (column-identical) social-lookup row projection
// to a domain User. All four FindUserBy*Id rows share these field types.
func socialRowToUser(id pgtype.UUID, email string, passwordHash *string, name string, isActive bool, role, authProvider string, emailVerified bool, lemon, avatar *string) User {
	return User{
		ID: uuidStr(id), Email: email, PasswordHash: strOrEmpty(passwordHash),
		Name: name, IsActive: isActive, Role: role, AuthProvider: authProvider,
		EmailVerified: emailVerified, LemonsqueezyCustomer: strOrEmpty(lemon),
		AvatarURL: strOrEmpty(avatar),
	}
}

// linkSocial stamps the social id + avatar onto an existing user and marks
// the email verified (social emails are provider-verified). Unknown
// provider → no-op nil. A missing/empty avatar writes NULL.
func (r *PgRepo) linkSocial(ctx context.Context, uid pgtype.UUID, p UserPatch) error {
	avatar := ptr("")
	if p.AvatarURL != nil {
		avatar = ptr(*p.AvatarURL)
	}
	switch p.SocialProvider {
	case "google":
		return r.q.LinkSocialGoogle(ctx, sqlc.LinkSocialGoogleParams{ID: uid, GoogleID: ptr(p.SocialID), AvatarUrl: avatar})
	case "facebook":
		return r.q.LinkSocialFacebook(ctx, sqlc.LinkSocialFacebookParams{ID: uid, FacebookID: ptr(p.SocialID), AvatarUrl: avatar})
	case "tiktok":
		return r.q.LinkSocialTiktok(ctx, sqlc.LinkSocialTiktokParams{ID: uid, TiktokID: ptr(p.SocialID), AvatarUrl: avatar})
	case "apple":
		return r.q.LinkSocialApple(ctx, sqlc.LinkSocialAppleParams{ID: uid, AppleID: ptr(p.SocialID), AvatarUrl: avatar})
	}
	return nil
}

// AuthState reads the verification/reset projection for an email.
func (r *PgRepo) AuthState(ctx context.Context, email string) (AuthState, error) {
	row, err := r.q.GetUserAuthState(ctx, email)
	if errors.Is(err, pgx.ErrNoRows) {
		return AuthState{}, ErrNotFound
	}
	if err != nil {
		return AuthState{}, err
	}
	return AuthState{
		ID: uuidStr(row.ID), Email: row.Email, Name: row.Name,
		PasswordHash: strOrEmpty(row.PasswordHash), AuthProvider: string(row.AuthProvider),
		IsActive: row.IsActive, EmailVerified: row.EmailVerified,
		EmailVerificationCode:    strOrEmpty(row.EmailVerificationCode),
		EmailVerificationExpires: tsValue(row.EmailVerificationExpires),
		PasswordResetCode:        strOrEmpty(row.PasswordResetCode),
		PasswordResetExpires:     tsValue(row.PasswordResetExpires),
		PasswordResetAttempts:    int(row.PasswordResetAttempts),
	}, nil
}

// ptr returns nil for "" (NULL column) else a pointer to s.
func ptr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// ts maps a *time.Time to pgtype.Timestamptz (nil → NULL).
func ts(t *time.Time) pgtype.Timestamptz {
	var v pgtype.Timestamptz
	if t != nil {
		v.Time = *t
		v.Valid = true
	}
	return v
}

// tsValue maps a pgtype.Timestamptz back to *time.Time (NULL → nil).
func tsValue(v pgtype.Timestamptz) *time.Time {
	if !v.Valid {
		return nil
	}
	t := v.Time
	return &t
}

func strOrEmpty(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

func parseUUID(s string) (pgtype.UUID, error) {
	var u pgtype.UUID
	err := u.Scan(s)
	return u, err
}

// uuidStr renders a pgtype.UUID as the canonical lowercase hyphenated
// form (8-4-4-4-12). pgtype.UUID.String() already does exactly this for
// valid values, so we delegate to it.
func uuidStr(u pgtype.UUID) string { return u.String() }
