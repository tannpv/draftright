// Admin user usecase (Phase 4c-2). Drives the three admin user routes:
//
//	GET   /admin/users        list + per-row N+1 (active plan name, usage today)
//	GET   /admin/users/:id     composite (user, subscription, usage_today, recent_usage)
//	PATCH /admin/users/:id     partial update → re-read full entity
//
// Parity authority: src/admin/admin.controller.ts (listUsers/getUser/updateUser)
// + the services it calls. The N+1 fan-out (one active-sub + one usage-count per
// listed user) mirrors Node's Promise.all over result.users — faithful, not
// optimised, because the goal is byte-identical output, not fewer queries.
//
// Clean-arch: this usecase imports no pgx/chi/net/http. It declares its own
// small consumer ports (userAdminRepo, UsageCounter, SubReader,
// RecentUsageReader); main.go injects the concrete *AdminRepo + *usage.Counter.
// The nested plan in AdminSubView reuses plans.PlanEntity (user → plans is a
// legal one-way edge: plans imports only shared, so no cycle).
package user

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/tannpv/draftright-rewrite/internal/plans"
	"github.com/tannpv/draftright-rewrite/internal/shared"
)

// userAdminRepo is the persistence port for the admin user reads/writes. The
// concrete *AdminRepo (admin_repo_pg.go) satisfies it. Listed methods are 1:1
// what this service calls.
type userAdminRepo interface {
	ListUsers(ctx context.Context, p ListUsersParams) ([]UserListRow, int, error)
	GetFull(ctx context.Context, id string) (UserDetail, error)
	Update(ctx context.Context, id string, p UserPatchAdmin) error
}

// UsageCounter mirrors usageService.countTodayByUser. One method — the same
// shape auth's UsageCounter port uses; *usage.Counter satisfies both.
type UsageCounter interface {
	CountToday(ctx context.Context, userID string) (int, error)
}

// SubReader yields a user's newest ACTIVE subscription with its plan, or
// (nil, nil) when there is none (mirrors findActiveByUserId returning null).
type SubReader interface {
	ActiveSubByUser(ctx context.Context, userID string) (*AdminSubView, error)
}

// RecentUsageReader yields up to 20 of a user's most-recent usage rows,
// created_at DESC (findRecentByUser). Always a non-nil slice.
type RecentUsageReader interface {
	RecentUsageByUser(ctx context.Context, userID string) ([]RecentUsageRow, error)
}

// AdminService is the admin user usecase. It composes the repo with three
// sibling readers (usage count, active sub, recent usage) to reproduce the
// controller's per-route shape exactly.
type AdminService struct {
	repo   userAdminRepo
	usage  UsageCounter
	sub    SubReader
	recent RecentUsageReader
}

// NewAdminService wires the repo + the three sibling ports.
func NewAdminService(repo userAdminRepo, usage UsageCounter, sub SubReader, recent RecentUsageReader) *AdminService {
	return &AdminService{repo: repo, usage: usage, sub: sub, recent: recent}
}

// List runs the bespoke paginated list, then for each row the Node N+1:
// active-sub (for the plan name) + today's usage count. Plan name falls to
// "None" when there is no active sub OR the active sub's plan name is empty
// (Node `sub?.plan?.name || 'None'` — empty string is falsy too).
func (s *AdminService) List(ctx context.Context, p ListUsersParams) ([]AdminUserRow, int, error) {
	rows, total, err := s.repo.ListUsers(ctx, p)
	if err != nil {
		return nil, 0, err
	}
	out := make([]AdminUserRow, 0, len(rows))
	for _, u := range rows {
		sub, err := s.sub.ActiveSubByUser(ctx, u.ID)
		if err != nil {
			return nil, 0, err
		}
		usageToday, err := s.usage.CountToday(ctx, u.ID)
		if err != nil {
			return nil, 0, err
		}
		plan := "None"
		if sub != nil && sub.Plan.Name != "" {
			plan = sub.Plan.Name
		}
		out = append(out, AdminUserRow{
			ID:         u.ID,
			Email:      u.Email,
			Name:       u.Name,
			Role:       u.Role,
			IsActive:   u.IsActive,
			Plan:       plan,
			UsageToday: usageToday,
			CreatedAt:  u.CreatedAt,
		})
	}
	return out, total, nil
}

// Get builds the composite GET /admin/users/:id body. Node's findById returns
// null on a missing/malformed id (it does NOT throw), and the controller still
// calls the other three reads and returns 200 with `user: null`. So an
// ErrNotFound from GetFull is swallowed to a nil User pointer here — NOT
// surfaced as a 404 — matching Node byte-for-byte. The sub/usage/recent reads
// all tolerate a non-existent user_id (null / 0 / []).
func (s *AdminService) Get(ctx context.Context, id string) (GetUserResponse, error) {
	var userPtr *StrippedUserDetail
	u, err := s.repo.GetFull(ctx, id)
	if err == nil {
		userPtr = &StrippedUserDetail{u}
	} else if !errors.Is(err, ErrNotFound) {
		return GetUserResponse{}, err
	}

	sub, err := s.sub.ActiveSubByUser(ctx, id)
	if err != nil {
		return GetUserResponse{}, err
	}
	usageToday, err := s.usage.CountToday(ctx, id)
	if err != nil {
		return GetUserResponse{}, err
	}
	recent, err := s.recent.RecentUsageByUser(ctx, id)
	if err != nil {
		return GetUserResponse{}, err
	}
	if recent == nil {
		recent = []RecentUsageRow{}
	}
	return GetUserResponse{
		User:         userPtr,
		Subscription: sub,
		UsageToday:   usageToday,
		RecentUsage:  recent,
	}, nil
}

// Update applies the partial patch then re-reads the full entity (Node
// usersService.update → findOneOrFail). A missing id makes the re-read return
// ErrNotFound, which the handler maps to 500 internal (parity with TypeORM's
// findOneOrFail → AllExceptionsFilter, NOT a 404).
func (s *AdminService) Update(ctx context.Context, id string, p UserPatchAdmin) (UserDetail, error) {
	if err := s.repo.Update(ctx, id, p); err != nil {
		return UserDetail{}, err
	}
	return s.repo.GetFull(ctx, id)
}

// GetUserResponse is the composite GET /admin/users/:id body. Go marshals
// struct fields in declaration order, so plain json tags pin the top-level key
// order user, subscription, usage_today, recent_usage. User is a pointer so it
// serialises null when absent (Node findById → null). RecentUsage must be
// non-nil (the usecase guarantees it) to emit [] not null.
type GetUserResponse struct {
	User         *StrippedUserDetail `json:"user"`
	Subscription *AdminSubView       `json:"subscription"`
	UsageToday   int                 `json:"usage_today"`
	RecentUsage  []RecentUsageRow    `json:"recent_usage"`
}

// AdminUserRow is one row of GET /admin/users. Key order id, email, name, role,
// is_active, plan, usage_today, created_at (the controller's per-user map
// literal). created_at is an ISO-millis string.
type AdminUserRow struct {
	ID         string
	Email      string
	Name       string
	Role       string
	IsActive   bool
	Plan       string
	UsageToday int
	CreatedAt  time.Time
}

func (a AdminUserRow) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		ID         string `json:"id"`
		Email      string `json:"email"`
		Name       string `json:"name"`
		Role       string `json:"role"`
		IsActive   bool   `json:"is_active"`
		Plan       string `json:"plan"`
		UsageToday int    `json:"usage_today"`
		CreatedAt  string `json:"created_at"`
	}{
		ID: a.ID, Email: a.Email, Name: a.Name, Role: a.Role,
		IsActive: a.IsActive, Plan: a.Plan, UsageToday: a.UsageToday,
		CreatedAt: shared.ISOMillis(a.CreatedAt),
	})
}

// AdminSubView is the FULL Node Subscription entity as the admin composite
// serialises it, with the `user` relation OMITTED (findActiveByUserId loads
// only relations:['plan']) and `plan` nested. Key order matches the entity's
// declaration order minus user: id, user_id, plan_id, plan, status, store_type,
// store_transaction_id, started_at, expires_at, created_at, updated_at. plan is
// a nested plans.PlanEntity (self-marshalling). Nullable expires_at →
// null | ISO-millis; nullable store_transaction_id → null | string. The three
// non-null timestamps always render the ISO-millis string.
type AdminSubView struct {
	ID                 string
	UserID             string
	PlanID             string
	Plan               plans.PlanEntity
	Status             string
	StoreType          string
	StoreTransactionID *string
	StartedAt          time.Time
	ExpiresAt          *time.Time
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

func (s AdminSubView) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		ID                 string           `json:"id"`
		UserID             string           `json:"user_id"`
		PlanID             string           `json:"plan_id"`
		Plan               plans.PlanEntity `json:"plan"`
		Status             string           `json:"status"`
		StoreType          string           `json:"store_type"`
		StoreTransactionID *string          `json:"store_transaction_id"`
		StartedAt          string           `json:"started_at"`
		ExpiresAt          *string          `json:"expires_at"`
		CreatedAt          string           `json:"created_at"`
		UpdatedAt          string           `json:"updated_at"`
	}{
		ID: s.ID, UserID: s.UserID, PlanID: s.PlanID, Plan: s.Plan,
		Status: s.Status, StoreType: s.StoreType, StoreTransactionID: s.StoreTransactionID,
		StartedAt: shared.ISOMillis(s.StartedAt), ExpiresAt: isoPtr(s.ExpiresAt),
		CreatedAt: shared.ISOMillis(s.CreatedAt), UpdatedAt: shared.ISOMillis(s.UpdatedAt),
	})
}

// RecentUsageRow is one usage_logs row as the composite serialises it. The user
// and ai_provider relations are NOT loaded → omitted. Key order id, user_id,
// tone, input_length, output_length, ai_provider_id, response_time_ms,
// created_at (UsageLog entity declaration order minus relations).
type RecentUsageRow struct {
	ID             string
	UserID         string
	Tone           string
	InputLength    int
	OutputLength   int
	AiProviderID   string
	ResponseTimeMs int
	CreatedAt      time.Time
}

func (u RecentUsageRow) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		ID             string `json:"id"`
		UserID         string `json:"user_id"`
		Tone           string `json:"tone"`
		InputLength    int    `json:"input_length"`
		OutputLength   int    `json:"output_length"`
		AiProviderID   string `json:"ai_provider_id"`
		ResponseTimeMs int    `json:"response_time_ms"`
		CreatedAt      string `json:"created_at"`
	}{
		ID: u.ID, UserID: u.UserID, Tone: u.Tone, InputLength: u.InputLength,
		OutputLength: u.OutputLength, AiProviderID: u.AiProviderID,
		ResponseTimeMs: u.ResponseTimeMs, CreatedAt: shared.ISOMillis(u.CreatedAt),
	})
}
