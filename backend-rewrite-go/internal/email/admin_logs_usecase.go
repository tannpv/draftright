// Admin email-logs list (bespoke pagination, NOT the shared listquery helper).
// Mirrors Node admin.controller.ts emailLogs:
//
//	const limit = Math.min(parseInt(q.limit) || 50, 200);
//	const page  = Math.max(parseInt(q.page)  || 1, 1);
//	const where = q.status ? { status: q.status } : {};   // status filter ONLY
//	findAndCount({ where, order:{created_at:'DESC'}, take:limit, skip:(page-1)*limit })
//	return { rows, total };                                // wrapper key = `rows`
//
// def 50 / max 200 (NOT the 10/100 of the shared listquery). The filter is
// status-only — there is NO email_type filter in Node, so none here.
package email

import (
	"context"
	"encoding/json"
	"time"

	"github.com/tannpv/draftright-rewrite/internal/shared"
)

// EmailLogRow is one raw EmailLog entity as Node's `rows` serialises it. Key
// order is the entity declaration order; MarshalJSON pins it. provider_id and
// error are nullable → *string (emit JSON null when absent). created_at renders
// as an ISO-millis string (shared.ISOMillis), matching TypeORM's timestamptz
// JSON.
type EmailLogRow struct {
	ID         string
	ToEmail    string
	EmailType  string
	Subject    string
	Status     string
	ProviderID *string
	Error      *string
	CreatedAt  time.Time
}

// MarshalJSON emits keys in exactly: id, to_email, email_type, subject, status,
// provider_id, error, created_at.
func (e EmailLogRow) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		ID         string  `json:"id"`
		ToEmail    string  `json:"to_email"`
		EmailType  string  `json:"email_type"`
		Subject    string  `json:"subject"`
		Status     string  `json:"status"`
		ProviderID *string `json:"provider_id"`
		Error      *string `json:"error"`
		CreatedAt  string  `json:"created_at"`
	}{
		ID: e.ID, ToEmail: e.ToEmail, EmailType: e.EmailType,
		Subject: e.Subject, Status: e.Status, ProviderID: e.ProviderID,
		Error: e.Error, CreatedAt: shared.ISOMillis(e.CreatedAt),
	})
}

// AdminLogsParams carries the bespoke-list inputs. Page/Limit are already
// clamped by the handler (Node's min/max); Status is "" for no filter.
type AdminLogsParams struct {
	Status string
	Page   int
	Limit  int
}

// adminLogsRepo is the service's consumer-side port; *AdminLogsRepo satisfies
// it. One method mirrors Node findAndCount (rows + total over the same WHERE).
type adminLogsRepo interface {
	ListLogs(ctx context.Context, p AdminLogsParams) ([]EmailLogRow, int, error)
}

// AdminLogsService is the email-logs list use case.
type AdminLogsService struct {
	repo adminLogsRepo
}

// NewAdminLogsService wires the repo port.
func NewAdminLogsService(repo adminLogsRepo) *AdminLogsService {
	return &AdminLogsService{repo: repo}
}

// List returns the page of rows plus the total over the same WHERE. It returns
// a non-nil empty slice when no rows match so the handler emits [] not null.
func (s *AdminLogsService) List(ctx context.Context, p AdminLogsParams) ([]EmailLogRow, int, error) {
	rows, total, err := s.repo.ListLogs(ctx, p)
	if err != nil {
		return nil, 0, err
	}
	if rows == nil {
		rows = []EmailLogRow{}
	}
	return rows, total, nil
}
