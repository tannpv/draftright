package domain_test

import (
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/tannpv/draftright-rewrite/internal/domain"
)

// userWith builds a User with the given plan limit + today's usage.
// Helper keeps the table tests below short.
func userWith(limit int32, used int64) *domain.User {
	return &domain.User{
		ID:        domain.UserID(uuid.New()),
		Email:     "test@example.com",
		Role:      "user",
		Plan:      domain.Plan{ID: uuid.New(), Name: "test-plan", DailyLimit: limit},
		UsedToday: used,
	}
}

func TestUser_CheckQuota_UnderLimit(t *testing.T) {
	require.NoError(t, userWith(100, 50).CheckQuota())
}

func TestUser_CheckQuota_AtLimit(t *testing.T) {
	err := userWith(100, 100).CheckQuota()
	require.ErrorIs(t, err, domain.ErrQuotaExceeded)
}

func TestUser_CheckQuota_OverLimit(t *testing.T) {
	err := userWith(100, 101).CheckQuota()
	require.ErrorIs(t, err, domain.ErrQuotaExceeded)
}

func TestUser_CheckQuota_UnlimitedZeroLimit(t *testing.T) {
	// Convention: DailyLimit == 0 means unlimited (matches NestJS).
	require.NoError(t, userWith(0, 999_999).CheckQuota())
}

func TestUser_CheckQuota_UnlimitedNegativeLimit(t *testing.T) {
	// Defensive: a negative DailyLimit (bad seed data) is treated as
	// unlimited rather than locking out the user.
	require.NoError(t, userWith(-1, 9001).CheckQuota())
}

func TestParseUserID_RoundTrip(t *testing.T) {
	original := uuid.New().String()
	parsed, err := domain.ParseUserID(original)
	require.NoError(t, err)
	require.Equal(t, original, parsed.String())
}

func TestParseUserID_RejectsGarbage(t *testing.T) {
	_, err := domain.ParseUserID("not-a-uuid")
	require.Error(t, err)
	require.True(t, errors.Is(err, domain.ErrInvalidInput),
		"want ErrInvalidInput, got %v", err)
}
