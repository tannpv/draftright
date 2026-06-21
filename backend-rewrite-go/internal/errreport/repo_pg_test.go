package errreport

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	sqlc "github.com/tannpv/draftright-rewrite/internal/shared/pg/sqlc"
)

// toUUID is pure logic (no DB). ResolveUserID/Insert are DB-bound and, like
// bugreports, are left to the live shadow gate. The admin read path maps a
// sqlc row → ErrorReportEntity, so the mapper + filter pass-through ARE
// covered here with a fake querier.

// fakeQuerier records the params it received and returns canned rows.
type fakeQuerier struct {
	listParams  sqlc.AdminListErrorsParams
	countParams sqlc.AdminCountErrorsParams
	listRows    []sqlc.ErrorReport
	count       int64
	getRow      sqlc.ErrorReport
	getErr      error
	deleted     int64
	statusParam sqlc.AdminSetErrorStatusRawParams
	statusErr   error
}

func (f *fakeQuerier) FindErrorByFingerprint(ctx context.Context, fp string) (sqlc.ErrorReport, error) {
	return sqlc.ErrorReport{}, nil
}
func (f *fakeQuerier) InsertErrorReport(ctx context.Context, arg sqlc.InsertErrorReportParams) (sqlc.InsertErrorReportRow, error) {
	return sqlc.InsertErrorReportRow{}, nil
}
func (f *fakeQuerier) BumpErrorReport(ctx context.Context, arg sqlc.BumpErrorReportParams) (sqlc.BumpErrorReportRow, error) {
	return sqlc.BumpErrorReportRow{}, nil
}
func (f *fakeQuerier) AdminListErrors(ctx context.Context, arg sqlc.AdminListErrorsParams) ([]sqlc.ErrorReport, error) {
	f.listParams = arg
	return f.listRows, nil
}
func (f *fakeQuerier) AdminCountErrors(ctx context.Context, arg sqlc.AdminCountErrorsParams) (int64, error) {
	f.countParams = arg
	return f.count, nil
}
func (f *fakeQuerier) AdminGetError(ctx context.Context, id pgtype.UUID) (sqlc.ErrorReport, error) {
	return f.getRow, f.getErr
}
func (f *fakeQuerier) AdminDeleteError(ctx context.Context, id pgtype.UUID) (int64, error) {
	return f.deleted, nil
}
func (f *fakeQuerier) AdminSetErrorStatusRaw(ctx context.Context, arg sqlc.AdminSetErrorStatusRawParams) (sqlc.ErrorReport, error) {
	f.statusParam = arg
	return f.getRow, f.statusErr
}
func (f *fakeQuerier) AdminSetErrorFixProposal(ctx context.Context, arg sqlc.AdminSetErrorFixProposalParams) (sqlc.ErrorReport, error) {
	return f.getRow, nil
}
func (f *fakeQuerier) AdminErrorFixCandidates(ctx context.Context, limit int32) ([]sqlc.ErrorReport, error) {
	return f.listRows, nil
}

func uuidPg(s string) pgtype.UUID {
	var u pgtype.UUID
	u.Bytes = uuid.MustParse(s)
	u.Valid = true
	return u
}

func TestToUUIDValid(t *testing.T) {
	id := "11111111-1111-1111-1111-111111111111"
	if u := toUUID(&id); !u.Valid {
		t.Fatal("expected Valid uuid")
	}
}

func TestToUUIDNilOrEmptyOrBad(t *testing.T) {
	empty := ""
	bad := "not-a-uuid"
	for _, in := range []*string{nil, &empty, &bad} {
		if u := toUUID(in); u.Valid {
			t.Fatalf("expected zero/invalid uuid for %v", in)
		}
	}
}

func TestMapEntity_FullRow(t *testing.T) {
	uid := "22222222-2222-2222-2222-222222222222"
	first := time.Date(2026, 6, 19, 10, 0, 0, 0, time.UTC)
	last := time.Date(2026, 6, 19, 12, 0, 0, 0, time.UTC)
	resolved := time.Date(2026, 6, 19, 11, 0, 0, 0, time.UTC)
	av := "1.2.3"
	et := "TypeError"
	row := sqlc.ErrorReport{
		ID:          uuidPg("11111111-1111-1111-1111-111111111111"),
		DisplayNo:   203,
		Platform:    "ios",
		AppVersion:  &av,
		Severity:    "error",
		ErrorType:   &et,
		Context:     []byte(`{"a":1}`),
		UserID:      uuidPg(uid),
		Fingerprint: "fp",
		Count:       5,
		Status:      4,
		ResolvedAt:  pgtype.Timestamptz{Time: resolved, Valid: true},
		FirstSeenAt: pgtype.Timestamptz{Time: first, Valid: true},
		LastSeenAt:  pgtype.Timestamptz{Time: last, Valid: true},
	}
	e := mapEntity(row)
	if e.ID != "11111111-1111-1111-1111-111111111111" {
		t.Fatalf("id = %q", e.ID)
	}
	if e.DisplayNo != "203" || e.Platform != "ios" || e.Severity != "error" {
		t.Fatalf("scalar mismatch: %+v", e)
	}
	if e.AppVersion == nil || *e.AppVersion != "1.2.3" {
		t.Fatalf("app_version = %v", e.AppVersion)
	}
	if e.UserID == nil || *e.UserID != uid {
		t.Fatalf("user_id = %v", e.UserID)
	}
	if string(e.Context) != `{"a":1}` {
		t.Fatalf("context = %q", string(e.Context))
	}
	if e.Count != 5 || e.Status != 4 {
		t.Fatalf("count/status = %d/%d", e.Count, e.Status)
	}
	if e.ResolvedAt == nil || *e.ResolvedAt != "2026-06-19T11:00:00.000Z" {
		t.Fatalf("resolved_at = %v", e.ResolvedAt)
	}
	if e.FirstSeenAt != "2026-06-19T10:00:00.000Z" || e.LastSeenAt != "2026-06-19T12:00:00.000Z" {
		t.Fatalf("seen times = %q / %q", e.FirstSeenAt, e.LastSeenAt)
	}
}

func TestMapEntity_NullsAndJSONOrder(t *testing.T) {
	row := sqlc.ErrorReport{
		ID:          uuidPg("11111111-1111-1111-1111-111111111111"),
		DisplayNo:   1,
		Platform:    "web",
		Severity:    "error",
		Fingerprint: "fp",
		Count:       1,
		Status:      0,
		FirstSeenAt: pgtype.Timestamptz{Time: time.Unix(0, 0).UTC(), Valid: true},
		LastSeenAt:  pgtype.Timestamptz{Time: time.Unix(0, 0).UTC(), Valid: true},
		// Context nil → JSON null; user_id/app_version/etc nil → JSON null.
	}
	e := mapEntity(row)
	if e.UserID != nil || e.AppVersion != nil || e.ResolvedAt != nil {
		t.Fatalf("expected nil pointers, got %+v", e)
	}
	b, err := json.Marshal(e)
	if err != nil {
		t.Fatal(err)
	}
	// JSON key order must mirror the Node entity verbatim (spec §4). display_no
	// is a quoted string (TypeORM bigint over the pg driver) and the timestamps
	// carry exactly 3 fractional digits + trailing Z (Date.toISOString()).
	got := string(b)
	want := `{"id":"11111111-1111-1111-1111-111111111111","display_no":"1","platform":"web","app_version":null,"severity":"error","error_type":null,"message":null,"stack_trace":null,"context":null,"user_id":null,"device_id":null,"fingerprint":"fp","count":1,"status":0,"ai_fix_proposal":null,"resolved_by":null,"resolved_at":null,"first_seen_at":"1970-01-01T00:00:00.000Z","last_seen_at":"1970-01-01T00:00:00.000Z"}`
	if got != want {
		t.Fatalf("json key order/null mismatch:\n got=%s\nwant=%s", got, want)
	}
}

func TestAdminList_FilterPassthrough(t *testing.T) {
	plat := "ios"
	sev := "error"
	st := 0
	f := &fakeQuerier{count: 7, listRows: []sqlc.ErrorReport{
		{ID: uuidPg("11111111-1111-1111-1111-111111111111"), DisplayNo: 1, Platform: "ios", Severity: "error", Fingerprint: "fp", Count: 1, Status: 0,
			FirstSeenAt: pgtype.Timestamptz{Time: time.Unix(0, 0), Valid: true}, LastSeenAt: pgtype.Timestamptz{Time: time.Unix(0, 0), Valid: true}},
	}}
	repo := NewPgRepo(f)
	items, total, err := repo.AdminList(context.Background(), AdminListFilter{
		Platform: &plat, Severity: &sev, Status: &st, Limit: 25, Offset: 50,
	})
	if err != nil {
		t.Fatal(err)
	}
	if total != 7 || len(items) != 1 {
		t.Fatalf("total=%d items=%d", total, len(items))
	}
	// list params pass through verbatim
	if f.listParams.Limit != 25 || f.listParams.Offset != 50 {
		t.Fatalf("limit/offset = %d/%d", f.listParams.Limit, f.listParams.Offset)
	}
	if f.listParams.Platform == nil || *f.listParams.Platform != "ios" {
		t.Fatalf("platform = %v", f.listParams.Platform)
	}
	if f.listParams.Severity == nil || *f.listParams.Severity != "error" {
		t.Fatalf("severity = %v", f.listParams.Severity)
	}
	if f.listParams.Status == nil || *f.listParams.Status != 0 {
		t.Fatalf("status = %v", f.listParams.Status)
	}
	// count params carry the same filters (no limit/offset)
	if f.countParams.Platform == nil || *f.countParams.Platform != "ios" {
		t.Fatalf("count platform = %v", f.countParams.Platform)
	}
	if f.countParams.Status == nil || *f.countParams.Status != 0 {
		t.Fatalf("count status = %v", f.countParams.Status)
	}
}

func TestAdminList_NilFilters(t *testing.T) {
	f := &fakeQuerier{}
	repo := NewPgRepo(f)
	if _, _, err := repo.AdminList(context.Background(), AdminListFilter{Limit: 50}); err != nil {
		t.Fatal(err)
	}
	if f.listParams.Platform != nil || f.listParams.Severity != nil || f.listParams.Status != nil {
		t.Fatalf("expected nil filter params, got %+v", f.listParams)
	}
}

func TestAdminGet_NotFound(t *testing.T) {
	f := &fakeQuerier{getErr: pgx.ErrNoRows}
	repo := NewPgRepo(f)
	_, err := repo.AdminGet(context.Background(), "11111111-1111-1111-1111-111111111111")
	if err != ErrNotFound {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestAdminDelete_Affected(t *testing.T) {
	repo := NewPgRepo(&fakeQuerier{deleted: 1})
	ok, err := repo.AdminDelete(context.Background(), "11111111-1111-1111-1111-111111111111")
	if err != nil || !ok {
		t.Fatalf("expected deleted=true, got %v %v", ok, err)
	}
	repo = NewPgRepo(&fakeQuerier{deleted: 0})
	ok, _ = repo.AdminDelete(context.Background(), "11111111-1111-1111-1111-111111111111")
	if ok {
		t.Fatal("expected deleted=false when absent")
	}
}

// AdminSetStatusRaw must unwrap a pgconn.PgError to its BARE .Message: pgx's
// err.Error() is "ERROR: <msg> (SQLSTATE <code>)", but Node/TypeORM surfaces
// only the message in its 500 body. The 500 envelope is byte-compared, so the
// SQLSTATE suffix would break parity (#37).
func TestAdminSetStatusRaw_UnwrapsPgError(t *testing.T) {
	f := &fakeQuerier{statusErr: &pgconn.PgError{
		Severity: "ERROR",
		Code:     "22P02",
		Message:  `invalid input syntax for type integer: "foo"`,
	}}
	repo := NewPgRepo(f)
	foo := "foo"
	_, err := repo.AdminSetStatusRaw(context.Background(), "11111111-1111-1111-1111-111111111111", &foo, false, nil, nil)
	if err == nil {
		t.Fatal("expected error")
	}
	if err.Error() != `invalid input syntax for type integer: "foo"` {
		t.Fatalf("err = %q, want bare PG message (no SQLSTATE)", err.Error())
	}
}

func TestAdminSetStatusRaw_ForwardsParams(t *testing.T) {
	f := &fakeQuerier{}
	repo := NewPgRepo(f)
	three := "3"
	_, err := repo.AdminSetStatusRaw(context.Background(), "11111111-1111-1111-1111-111111111111", &three, false, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if f.statusParam.StatusText == nil || *f.statusParam.StatusText != "3" {
		t.Fatalf("StatusText = %v, want \"3\"", f.statusParam.StatusText)
	}
	if f.statusParam.SetResolved {
		t.Fatal("SetResolved should be false")
	}
}

func TestAdminFixCandidates_IDsOnly(t *testing.T) {
	f := &fakeQuerier{listRows: []sqlc.ErrorReport{
		{ID: uuidPg("11111111-1111-1111-1111-111111111111")},
		{ID: uuidPg("22222222-2222-2222-2222-222222222222")},
	}}
	repo := NewPgRepo(f)
	ids, err := repo.AdminFixCandidates(context.Background(), 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(ids) != 2 || ids[0] != "11111111-1111-1111-1111-111111111111" {
		t.Fatalf("ids = %v", ids)
	}
}
