package errreport

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	sqlc "github.com/tannpv/draftright-rewrite/internal/shared/pg/sqlc"
)

// Existing is the dedup/insert projection the handler echoes back.
type Existing struct {
	ID          string
	DisplayNo   int64
	Count       int32
	Fingerprint string
	FirstSeenAt time.Time
}

// NewRow is a fresh insert payload.
type NewRow struct {
	Platform    string
	AppVersion  *string
	Severity    string
	ErrorType   *string
	Message     *string
	StackTrace  *string
	Context     []byte
	UserID      *string
	DeviceID    *string
	Fingerprint string
}

// Querier is the sqlc subset the errors repo needs.
type Querier interface {
	FindErrorByFingerprint(ctx context.Context, fingerprint string) (sqlc.ErrorReport, error)
	InsertErrorReport(ctx context.Context, arg sqlc.InsertErrorReportParams) (sqlc.InsertErrorReportRow, error)
	BumpErrorReport(ctx context.Context, arg sqlc.BumpErrorReportParams) (sqlc.BumpErrorReportRow, error)
}

// Repo is the error_reports adapter.
type Repo struct{ q Querier }

// NewPgRepo wires the querier.
func NewPgRepo(q Querier) *Repo { return &Repo{q: q} }

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

// FindByFingerprint returns the existing group or (nil,nil).
func (r *Repo) FindByFingerprint(ctx context.Context, fp string) (*Existing, error) {
	row, err := r.q.FindErrorByFingerprint(ctx, fp)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return existingFromRow(row.ID, row.DisplayNo, row.Count, row.Fingerprint, row.FirstSeenAt), nil
}

// Insert creates a new group (count=1, status=0).
func (r *Repo) Insert(ctx context.Context, n NewRow) (*Existing, error) {
	row, err := r.q.InsertErrorReport(ctx, sqlc.InsertErrorReportParams{
		Platform: n.Platform, AppVersion: n.AppVersion, Severity: n.Severity,
		ErrorType: n.ErrorType, Message: n.Message, StackTrace: n.StackTrace,
		Context: n.Context, UserID: toUUID(n.UserID), DeviceID: n.DeviceID, Fingerprint: n.Fingerprint,
	})
	if err != nil {
		return nil, err
	}
	return existingFromRow(row.ID, row.DisplayNo, row.Count, n.Fingerprint, row.FirstSeenAt), nil
}

// BumpDedup increments count + conditionally refreshes fields.
func (r *Repo) BumpDedup(ctx context.Context, fp string, appVersion, userID, deviceID *string, context []byte) (*Existing, error) {
	row, err := r.q.BumpErrorReport(ctx, sqlc.BumpErrorReportParams{
		Fingerprint: fp, AppVersion: appVersion, UserID: toUUID(userID), DeviceID: deviceID, Context: context,
	})
	if err != nil {
		return nil, err
	}
	return existingFromRow(row.ID, row.DisplayNo, row.Count, fp, row.FirstSeenAt), nil
}

func existingFromRow(id pgtype.UUID, displayNo int64, count int32, fp string, firstSeen pgtype.Timestamptz) *Existing {
	e := &Existing{DisplayNo: displayNo, Count: count, Fingerprint: fp}
	if id.Valid {
		e.ID = uuid.UUID(id.Bytes).String()
	}
	if firstSeen.Valid {
		e.FirstSeenAt = firstSeen.Time
	}
	return e
}
