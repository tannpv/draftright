package feedback

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/tannpv/draftright-rewrite/internal/shared"
	sqlc "github.com/tannpv/draftright-rewrite/internal/shared/pg/sqlc"
)

// Querier is the sqlc subset the feedback repo needs.
type Querier interface {
	UserExists(ctx context.Context, id pgtype.UUID) (bool, error)
	InsertFeedback(ctx context.Context, arg sqlc.InsertFeedbackParams) (sqlc.InsertFeedbackRow, error)
	FindFeature(ctx context.Context, id pgtype.UUID) (pgtype.UUID, error)
	FindVote(ctx context.Context, arg sqlc.FindVoteParams) (pgtype.UUID, error)
	InsertVote(ctx context.Context, arg sqlc.InsertVoteParams) error
	DeleteVote(ctx context.Context, arg sqlc.DeleteVoteParams) error
	CountVotes(ctx context.Context, featureID pgtype.UUID) (int64, error)
	UpdateFeatureVoteCount(ctx context.Context, arg sqlc.UpdateFeatureVoteCountParams) error
	CountFeatures(ctx context.Context, arg sqlc.CountFeaturesParams) (int64, error)
	ListPublicFeatures(ctx context.Context, arg sqlc.ListPublicFeaturesParams) ([]sqlc.ListPublicFeaturesRow, error)
	VotedFeatureIDs(ctx context.Context, arg sqlc.VotedFeatureIDsParams) ([]pgtype.UUID, error)
}

// PgRepo is the bug_reports + feature_votes adapter for the feedback board.
type PgRepo struct{ q Querier }

// NewPgRepo wires the querier.
func NewPgRepo(q Querier) *PgRepo { return &PgRepo{q: q} }

func toUUID(s *string) pgtype.UUID {
	var u pgtype.UUID
	if s == nil || *s == "" {
		return u
	}
	parsed, err := uuid.Parse(*s)
	if err != nil {
		return u
	}
	u.Bytes = parsed
	u.Valid = true
	return u
}

func uuidStr(u pgtype.UUID) string {
	if !u.Valid {
		return ""
	}
	return uuid.UUID(u.Bytes).String()
}

// ResolveUserID returns id when a users row exists for it, else nil. A JWT can
// outlive its user (account deleted); the user_id FK would 500 the insert on an
// orphan id — nulling it keeps the row (it still carries user_email). Mirrors
// BugReportsService.resolveUserId.
func (r *PgRepo) ResolveUserID(ctx context.Context, id *string) (*string, error) {
	if id == nil || *id == "" {
		return nil, nil
	}
	exists, err := r.q.UserExists(ctx, toUUID(id))
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, nil
	}
	return id, nil
}

// Insert creates a feedback bug_reports row (status='new', vote_count=0,
// is_public=true, context NULL) and returns its id + display_no + kind.
func (r *PgRepo) Insert(ctx context.Context, n NewRow) (Created, error) {
	row, err := r.q.InsertFeedback(ctx, sqlc.InsertFeedbackParams{
		Kind:           n.Kind,
		Title:          n.Title,
		TargetPlatform: n.TargetPlatform,
		Source:         n.Source,
		Description:    n.Description,
		AppVersion:     n.AppVersion,
		OsInfo:         n.OsInfo,
		UserID:         toUUID(n.UserID),
		UserEmail:      n.UserEmail,
		Context:        nil,
	})
	if err != nil {
		return Created{}, err
	}
	return Created{ID: uuidStr(row.ID), DisplayNo: row.DisplayNo, Kind: n.Kind}, nil
}

// FeatureExists reports whether a kind='feature' row exists for the id. Mirrors
// findFeatureById's row.kind !== 'feature' → NotFound guard.
func (r *PgRepo) FeatureExists(ctx context.Context, featureID string) (bool, error) {
	_, err := r.q.FindFeature(ctx, toUUID(&featureID))
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

// VoteExists reports whether the caller already upvoted the feature.
func (r *PgRepo) VoteExists(ctx context.Context, featureID, userID string) (bool, error) {
	_, err := r.q.FindVote(ctx, sqlc.FindVoteParams{
		FeatureID: toUUID(&featureID),
		UserID:    toUUID(&userID),
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

// InsertVote records the caller's upvote.
func (r *PgRepo) InsertVote(ctx context.Context, featureID, userID string) error {
	return r.q.InsertVote(ctx, sqlc.InsertVoteParams{
		FeatureID: toUUID(&featureID),
		UserID:    toUUID(&userID),
	})
}

// DeleteVote removes the caller's upvote.
func (r *PgRepo) DeleteVote(ctx context.Context, featureID, userID string) error {
	return r.q.DeleteVote(ctx, sqlc.DeleteVoteParams{
		FeatureID: toUUID(&featureID),
		UserID:    toUUID(&userID),
	})
}

// CountVotes returns COUNT(feature_votes) for the feature.
func (r *PgRepo) CountVotes(ctx context.Context, featureID string) (int, error) {
	n, err := r.q.CountVotes(ctx, toUUID(&featureID))
	return int(n), err
}

// UpdateVoteCount persists the recomputed vote_count on the bug_reports row.
func (r *PgRepo) UpdateVoteCount(ctx context.Context, featureID string, count int) error {
	return r.q.UpdateFeatureVoteCount(ctx, sqlc.UpdateFeatureVoteCountParams{
		ID:        toUUID(&featureID),
		VoteCount: int32(count),
	})
}

// CountFeatures returns the board total (kind='feature', is_public, optional
// status/platform filters).
func (r *PgRepo) CountFeatures(ctx context.Context, status, platform *string) (int64, error) {
	return r.q.CountFeatures(ctx, sqlc.CountFeaturesParams{
		Status:         status,
		TargetPlatform: platform,
	})
}

// ListFeatures returns one page of the board, mapped into FeatureRow (entity
// JSON shape). viewerHasVoted is left false here; the Service stamps it.
func (r *PgRepo) ListFeatures(ctx context.Context, status, platform *string, limit, offset int) ([]FeatureRow, error) {
	rows, err := r.q.ListPublicFeatures(ctx, sqlc.ListPublicFeaturesParams{
		Limit:          int32(limit),
		Offset:         int32(offset),
		Status:         status,
		TargetPlatform: platform,
	})
	if err != nil {
		return nil, err
	}
	out := make([]FeatureRow, len(rows))
	for i, row := range rows {
		out[i] = featureRowFromSQL(row)
	}
	return out, nil
}

// VotedFeatureIDs returns the subset of ids the caller has voted for.
func (r *PgRepo) VotedFeatureIDs(ctx context.Context, ids []string, userID string) (map[string]bool, error) {
	uuids := make([]pgtype.UUID, len(ids))
	for i, id := range ids {
		uuids[i] = toUUID(&id)
	}
	rows, err := r.q.VotedFeatureIDs(ctx, sqlc.VotedFeatureIDsParams{
		FeatureIds: uuids,
		UserID:     toUUID(&userID),
	})
	if err != nil {
		return nil, err
	}
	out := make(map[string]bool, len(rows))
	for _, u := range rows {
		out[uuidStr(u)] = true
	}
	return out, nil
}

// featureRowFromSQL maps a sqlc board row into the entity-shaped FeatureRow.
// display_no → string (TypeORM bigint → JS string); created_at/updated_at →
// ISOMillis; context jsonb → decoded object (so it serializes as an object, not
// base64 bytes).
func featureRowFromSQL(row sqlc.ListPublicFeaturesRow) FeatureRow {
	fr := FeatureRow{
		ID:                 uuidStr(row.ID),
		DisplayNo:          strconv.FormatInt(row.DisplayNo, 10),
		Source:             row.Source,
		Description:        row.Description,
		ScreenshotPath:     row.ScreenshotPath,
		ScreenshotFilename: row.ScreenshotFilename,
		AppVersion:         row.AppVersion,
		OsInfo:             row.OsInfo,
		UserEmail:          row.UserEmail,
		Context:            decodeJSON(row.Context),
		Status:             row.Status,
		Kind:               row.Kind,
		Title:              row.Title,
		TargetPlatform:     row.TargetPlatform,
		VoteCount:          row.VoteCount,
		IsPublic:           row.IsPublic,
		AdminNotes:         row.AdminNotes,
		AiFixProposal:      row.AiFixProposal,
	}
	if row.UserID.Valid {
		s := uuidStr(row.UserID)
		fr.UserID = &s
	}
	if row.AiFixProposedAt.Valid {
		s := shared.ISOMillis(row.AiFixProposedAt.Time)
		fr.AiFixProposedAt = &s
	}
	if row.CreatedAt.Valid {
		fr.CreatedAt = shared.ISOMillis(row.CreatedAt.Time)
	}
	if row.UpdatedAt.Valid {
		fr.UpdatedAt = shared.ISOMillis(row.UpdatedAt.Time)
	}
	return fr
}

// decodeJSON turns a raw jsonb byte slice into a Go value so it re-serializes as
// the original object (not base64). nil/empty → nil (JSON null), matching a NULL
// context column. A feature row's context is always NULL on this route, but the
// board SELECTs the column for every kind='feature' row, so decode defensively.
func decodeJSON(b []byte) any {
	if len(b) == 0 {
		return nil
	}
	var v any
	if err := json.Unmarshal(b, &v); err != nil {
		return nil
	}
	return v
}
