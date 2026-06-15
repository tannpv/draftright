package bugreports

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	sqlc "github.com/tannpv/draftright-rewrite/internal/shared/pg/sqlc"
)

// NewRow is the validated insert payload (post-DTO, post-screenshot).
type NewRow struct {
	Source             string
	Description        string
	ScreenshotPath     *string
	ScreenshotFilename *string
	AppVersion         *string
	OsInfo             *string
	UserID             *string
	UserEmail          *string
	Context            []byte // raw jsonb, nil when absent
}

// Querier is the sqlc subset the bug-reports repo needs.
type Querier interface {
	UserExists(ctx context.Context, id pgtype.UUID) (bool, error)
	InsertBugReport(ctx context.Context, arg sqlc.InsertBugReportParams) (sqlc.InsertBugReportRow, error)
}

// PgRepo is the bug_reports adapter.
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

// ResolveUserID returns id when a users row exists for it, else nil. A JWT
// can outlive its user (account deleted); the user_id FK would make the
// insert 500 on an orphan id — nulling it keeps the report (it still carries
// user_email). Mirrors BugReportsService.resolveUserId.
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

// Insert creates a bug_reports row (status='new', kind='bug') and returns
// its id + display_no.
func (r *PgRepo) Insert(ctx context.Context, n NewRow) (Created, error) {
	row, err := r.q.InsertBugReport(ctx, sqlc.InsertBugReportParams{
		Source:             n.Source,
		Description:        n.Description,
		ScreenshotPath:     n.ScreenshotPath,
		ScreenshotFilename: n.ScreenshotFilename,
		AppVersion:         n.AppVersion,
		OsInfo:             n.OsInfo,
		UserID:             toUUID(n.UserID),
		UserEmail:          n.UserEmail,
		Context:            n.Context,
	})
	if err != nil {
		return Created{}, err
	}
	c := Created{DisplayNo: row.DisplayNo}
	if row.ID.Valid {
		c.ID = uuid.UUID(row.ID.Bytes).String()
	}
	return c, nil
}
