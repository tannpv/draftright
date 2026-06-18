package email

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// TestEmailLogsLimit pins the limit clamp to Node's
// Math.min(parseInt(q.limit) || 50, 200). JS `||` treats only 0 and NaN as
// falsy; negatives pass through, so "-1" stays -1 (faithful to Node).
func TestEmailLogsLimit(t *testing.T) {
	cases := []struct {
		in   string
		want int
	}{
		{"", 50},      // absent → 50
		{"10", 10},    // plain
		{"500", 200},  // capped
		{"250", 200},  // capped
		{"200", 200},  // boundary
		{"0", 50},     // 0 || 50 = 50
		{"abc", 50},   // NaN → 50
		{"-1", -1},    // -1 || 50 = -1 → min(-1,200) = -1
		{"10abc", 10}, // JS parseInt prefix
	}
	for _, c := range cases {
		if got := emailLogsLimit(c.in); got != c.want {
			t.Errorf("emailLogsLimit(%q) = %d, want %d", c.in, got, c.want)
		}
	}
}

// TestEmailLogsPage pins the page clamp to Math.max(parseInt(q.page) || 1, 1)
// → always ≥ 1.
func TestEmailLogsPage(t *testing.T) {
	cases := []struct {
		in   string
		want int
	}{
		{"", 1},    // absent → 1
		{"3", 3},   // plain
		{"0", 1},   // 0 || 1 = 1
		{"-5", 1},  // -5 || 1 = -5 → max(-5,1) = 1
		{"abc", 1}, // NaN → 1
		{"1", 1},   // boundary
	}
	for _, c := range cases {
		if got := emailLogsPage(c.in); got != c.want {
			t.Errorf("emailLogsPage(%q) = %d, want %d", c.in, got, c.want)
		}
	}
}

// fakeAdminLogsRepo is a DB-free port double for the handler test.
type fakeAdminLogsRepo struct {
	rows  []EmailLogRow
	total int
}

func (f *fakeAdminLogsRepo) ListLogs(ctx context.Context, p AdminLogsParams) ([]EmailLogRow, int, error) {
	return f.rows, f.total, nil
}

// TestAdminLogsHandlerEmpty asserts the empty body serialises {"rows":[],"total":0}
// — rows is [] (non-nil) not null, and the wrapper key is `rows` not `logs`.
func TestAdminLogsHandlerEmpty(t *testing.T) {
	h := NewAdminLogsHandler(NewAdminLogsService(&fakeAdminLogsRepo{rows: []EmailLogRow{}, total: 0}))
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/admin/email-logs", nil)
	h.List(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	body := rr.Body.String()
	if !strings.Contains(body, `"rows":[]`) {
		t.Errorf("rows must serialise as [] (non-nil), got %s", body)
	}
	if !strings.Contains(body, `"total":0`) {
		t.Errorf("total missing/wrong, got %s", body)
	}
}

// TestAdminLogsHandlerRowKeyOrder asserts a populated row emits keys in entity
// declaration order: id, to_email, email_type, subject, status, provider_id,
// error, created_at; and that nullable provider_id/error serialise as null.
func TestAdminLogsHandlerRowKeyOrder(t *testing.T) {
	created := time.Date(2026, 6, 11, 8, 30, 0, 0, time.UTC)
	row := EmailLogRow{
		ID:         "abc",
		ToEmail:    "u@example.com",
		EmailType:  "welcome",
		Subject:    "Hi",
		Status:     "sent",
		ProviderID: nil,
		Error:      nil,
		CreatedAt:  created,
	}
	h := NewAdminLogsHandler(NewAdminLogsService(&fakeAdminLogsRepo{rows: []EmailLogRow{row}, total: 1}))
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/admin/email-logs", nil)
	h.List(rr, req)

	body := rr.Body.String()
	wantOrder := []string{
		`"id"`, `"to_email"`, `"email_type"`, `"subject"`,
		`"status"`, `"provider_id"`, `"error"`, `"created_at"`,
	}
	prev := -1
	for _, key := range wantOrder {
		idx := strings.Index(body, key)
		if idx < 0 {
			t.Fatalf("key %s missing from %s", key, body)
		}
		if idx < prev {
			t.Fatalf("key %s out of order in %s", key, body)
		}
		prev = idx
	}
	if !strings.Contains(body, `"provider_id":null`) {
		t.Errorf("provider_id must be null, got %s", body)
	}
	if !strings.Contains(body, `"error":null`) {
		t.Errorf("error must be null, got %s", body)
	}
	if !strings.Contains(body, `"created_at":"2026-06-11T08:30:00.000Z"`) {
		t.Errorf("created_at must be ISO-millis, got %s", body)
	}

	// Sanity: decode round-trips and total propagates.
	var decoded struct {
		Rows  []json.RawMessage `json:"rows"`
		Total int               `json:"total"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &decoded); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if decoded.Total != 1 || len(decoded.Rows) != 1 {
		t.Errorf("got total=%d rows=%d, want 1/1", decoded.Total, len(decoded.Rows))
	}
}
