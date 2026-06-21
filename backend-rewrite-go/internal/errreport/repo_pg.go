package errreport

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/tannpv/draftright-rewrite/internal/shared"
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
	AdminListErrors(ctx context.Context, arg sqlc.AdminListErrorsParams) ([]sqlc.ErrorReport, error)
	AdminCountErrors(ctx context.Context, arg sqlc.AdminCountErrorsParams) (int64, error)
	AdminGetError(ctx context.Context, id pgtype.UUID) (sqlc.ErrorReport, error)
	AdminDeleteError(ctx context.Context, id pgtype.UUID) (int64, error)
	AdminSetErrorStatusRaw(ctx context.Context, arg sqlc.AdminSetErrorStatusRawParams) (sqlc.ErrorReport, error)
	AdminSetErrorFixProposal(ctx context.Context, arg sqlc.AdminSetErrorFixProposalParams) (sqlc.ErrorReport, error)
	AdminErrorFixCandidates(ctx context.Context, limit int32) ([]sqlc.ErrorReport, error)
}

// PgRepo is the error_reports adapter.
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

// FindByFingerprint returns the existing group or (nil,nil).
func (r *PgRepo) FindByFingerprint(ctx context.Context, fp string) (*Existing, error) {
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
func (r *PgRepo) Insert(ctx context.Context, n NewRow) (*Existing, error) {
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
func (r *PgRepo) BumpDedup(ctx context.Context, fp string, appVersion, userID, deviceID *string, context []byte) (*Existing, error) {
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

// mapEntity converts a sqlc error_reports row to the admin entity. Used by
// every admin read/write path so the JSON projection is identical.
func mapEntity(r sqlc.ErrorReport) ErrorReportEntity {
	e := ErrorReportEntity{
		DisplayNo:     strconv.FormatInt(r.DisplayNo, 10),
		Platform:      r.Platform,
		AppVersion:    r.AppVersion,
		Severity:      r.Severity,
		ErrorType:     r.ErrorType,
		Message:       r.Message,
		StackTrace:    r.StackTrace,
		Context:       json.RawMessage(r.Context),
		DeviceID:      r.DeviceID,
		Fingerprint:   r.Fingerprint,
		Count:         int(r.Count),
		Status:        int(r.Status),
		AiFixProposal: r.AiFixProposal,
		ResolvedBy:    r.ResolvedBy,
	}
	if r.ID.Valid {
		e.ID = uuid.UUID(r.ID.Bytes).String()
	}
	if r.UserID.Valid {
		s := uuid.UUID(r.UserID.Bytes).String()
		e.UserID = &s
	}
	if r.ResolvedAt.Valid {
		s := shared.ISOMillis(r.ResolvedAt.Time)
		e.ResolvedAt = &s
	}
	if r.FirstSeenAt.Valid {
		e.FirstSeenAt = shared.ISOMillis(r.FirstSeenAt.Time)
	}
	if r.LastSeenAt.Valid {
		e.LastSeenAt = shared.ISOMillis(r.LastSeenAt.Time)
	}
	return e
}

// AdminList returns the filtered page of error rows + the total count
// (Node ErrorsService.list → getManyAndCount). last_seen_at DESC.
func (r *PgRepo) AdminList(ctx context.Context, f AdminListFilter) ([]ErrorReportEntity, int, error) {
	var status *int32
	if f.Status != nil {
		s := int32(*f.Status)
		status = &s
	}
	rows, err := r.q.AdminListErrors(ctx, sqlc.AdminListErrorsParams{
		Limit:    int32(f.Limit),
		Offset:   int32(f.Offset),
		Platform: f.Platform,
		Status:   status,
		Severity: f.Severity,
	})
	if err != nil {
		return nil, 0, err
	}
	total, err := r.q.AdminCountErrors(ctx, sqlc.AdminCountErrorsParams{
		Platform: f.Platform,
		Status:   status,
		Severity: f.Severity,
	})
	if err != nil {
		return nil, 0, err
	}
	items := make([]ErrorReportEntity, 0, len(rows))
	for _, row := range rows {
		items = append(items, mapEntity(row))
	}
	return items, int(total), nil
}

// AdminGet loads one error row, returning ErrNotFound when absent.
func (r *PgRepo) AdminGet(ctx context.Context, id string) (ErrorReportEntity, error) {
	row, err := r.q.AdminGetError(ctx, toUUID(&id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrorReportEntity{}, ErrNotFound
		}
		return ErrorReportEntity{}, err
	}
	return mapEntity(row), nil
}

// AdminDelete hard-deletes by id; returns whether a row was removed
// (Node deleteOne → {id, deleted}; idempotent, no error when absent).
func (r *PgRepo) AdminDelete(ctx context.Context, id string) (bool, error) {
	affected, err := r.q.AdminDeleteError(ctx, toUUID(&id))
	if err != nil {
		return false, err
	}
	return affected > 0, nil
}

// AdminSetStatusRaw binds the raw status TEXT (statusText) to a
// `::text::integer` UPDATE so Postgres coerces it exactly as node-pg does —
// reproducing the int4-input 500 for non-numeric values and the not-null
// violation for a nil (json null) value (#37). resolved_at/resolved_by are
// overwritten only when setResolved is true (status 4/5); otherwise the stored
// values are preserved (Node repo.save() re-persists the loaded row's columns).
func (r *PgRepo) AdminSetStatusRaw(ctx context.Context, id string, statusText *string, setResolved bool, resolvedAt *time.Time, resolvedBy *string) (ErrorReportEntity, error) {
	var ra pgtype.Timestamptz
	if resolvedAt != nil {
		ra.Time = *resolvedAt
		ra.Valid = true
	}
	row, err := r.q.AdminSetErrorStatusRaw(ctx, sqlc.AdminSetErrorStatusRawParams{
		ID:          toUUID(&id),
		StatusText:  statusText,
		SetResolved: setResolved,
		ResolvedAt:  ra,
		ResolvedBy:  resolvedBy,
	})
	if err != nil {
		// pgx's PgError.Error() is "ERROR: <msg> (SQLSTATE <code>)", but
		// Node/TypeORM surfaces the BARE PG message in its 500 envelope.
		// Unwrap to .Message so the byte-for-byte error body matches.
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			return ErrorReportEntity{}, errors.New(pgErr.Message)
		}
		return ErrorReportEntity{}, err
	}
	return mapEntity(row), nil
}

// AdminSetFixProposal stores the AI proposal + new status, returning the
// saved row (Node suggestFix → ai_fix_proposal + status=3).
func (r *PgRepo) AdminSetFixProposal(ctx context.Context, id string, proposal string, status int) (ErrorReportEntity, error) {
	row, err := r.q.AdminSetErrorFixProposal(ctx, sqlc.AdminSetErrorFixProposalParams{
		ID:            toUUID(&id),
		AiFixProposal: &proposal,
		Status:        int32(status),
	})
	if err != nil {
		return ErrorReportEntity{}, err
	}
	return mapEntity(row), nil
}

// AdminFixCandidates returns the ids of the top error groups eligible for
// AI analysis (Node cron: status=0, ai_fix_proposal NULL, count>=2).
func (r *PgRepo) AdminFixCandidates(ctx context.Context, limit int32) ([]string, error) {
	rows, err := r.q.AdminErrorFixCandidates(ctx, limit)
	if err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(rows))
	for _, row := range rows {
		if row.ID.Valid {
			ids = append(ids, uuid.UUID(row.ID.Bytes).String())
		}
	}
	return ids, nil
}
