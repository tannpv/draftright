package bugreports

import (
	"bytes"
	"context"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// fakeRepo records inserts and resolves user ids without a DB.
type fakeRepo struct {
	inserted    *NewRow
	insertCalls int
	created     Created
	resolveTo   *string
}

func (f *fakeRepo) Insert(_ context.Context, n NewRow) (Created, error) {
	cp := n
	f.inserted = &cp
	f.insertCalls++
	if f.created.ID == "" && f.created.DisplayNo == 0 {
		return Created{ID: "11111111-1111-1111-1111-111111111111", DisplayNo: 7}, nil
	}
	return f.created, nil
}

func (f *fakeRepo) ResolveUserID(_ context.Context, id *string) (*string, error) {
	return f.resolveTo, nil
}

func newTestHandler(t *testing.T, repo Repo) *Handler {
	t.Helper()
	store := NewStorage(t.TempDir(), fixedClock(time.Date(2026, 6, 15, 12, 0, 0, 0, time.UTC)))
	svc := NewService(repo, store)
	return NewHandler(svc, nil)
}

// buildMultipart constructs a multipart/form-data body with the given text
// fields and an optional file part. Returns the body + content-type header.
func buildMultipart(t *testing.T, fields map[string]string, fileField, fileName, fileMime string, fileData []byte) (*bytes.Buffer, string) {
	t.Helper()
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	for k, v := range fields {
		if err := w.WriteField(k, v); err != nil {
			t.Fatal(err)
		}
	}
	if fileField != "" {
		h := make(map[string][]string)
		h["Content-Disposition"] = []string{`form-data; name="` + fileField + `"; filename="` + fileName + `"`}
		h["Content-Type"] = []string{fileMime}
		part, err := w.CreatePart(h)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := part.Write(fileData); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return &buf, w.FormDataContentType()
}

func doRequest(t *testing.T, h *Handler, body *bytes.Buffer, contentType string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/bug-reports", body)
	req.Header.Set("Content-Type", contentType)
	h.Create(rec, req)
	return rec
}

// (a) Honeypot: website non-empty → 201 {"id":null,"status":"received"}, no insert.
func TestBugReports_HoneypotDropsSilently(t *testing.T) {
	repo := &fakeRepo{}
	h := newTestHandler(t, repo)
	body, ct := buildMultipart(t, map[string]string{
		"description": "real bug",
		"source":      "web",
		"website":     "spam.example.com",
	}, "", "", "", nil)
	rec := doRequest(t, h, body, ct)

	if rec.Code != 201 {
		t.Fatalf("status = %d, want 201", rec.Code)
	}
	raw := rec.Body.String()
	assertKeyOrder(t, raw, "id", "status")
	if strings.Contains(raw, `"ref"`) || strings.Contains(raw, `"message"`) {
		t.Errorf("honeypot body must not contain ref/message: %s", raw)
	}
	var m map[string]any
	json.Unmarshal([]byte(raw), &m)
	if _, ok := m["id"]; !ok {
		t.Fatalf("honeypot body missing id key: %s", raw)
	}
	if m["id"] != nil {
		t.Errorf("honeypot id must be JSON null, got %v", m["id"])
	}
	if m["status"] != "received" {
		t.Errorf("honeypot status = %v, want received", m["status"])
	}
	if repo.insertCalls != 0 {
		t.Errorf("honeypot must not insert, got %d calls", repo.insertCalls)
	}
}

// (b) Missing description → 400 with IsString + humanized MinLength messages.
func TestBugReports_MissingDescription400(t *testing.T) {
	repo := &fakeRepo{}
	h := newTestHandler(t, repo)
	body, ct := buildMultipart(t, map[string]string{"source": "web"}, "", "", "", nil)
	rec := doRequest(t, h, body, ct)

	if rec.Code != 400 {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
	var m map[string]any
	json.Unmarshal(rec.Body.Bytes(), &m)
	if m["code"] != "invalid-input" {
		t.Fatalf("code = %v, want invalid-input", m["code"])
	}
	want := "description must be a string. Description must be at least 1 characters."
	if m["error"] != want {
		t.Fatalf("error = %q, want %q", m["error"], want)
	}
	if repo.insertCalls != 0 {
		t.Error("invalid input must not insert")
	}
}

// (b2) Empty description → 400 humanized MinLength only.
func TestBugReports_EmptyDescription400(t *testing.T) {
	repo := &fakeRepo{}
	h := newTestHandler(t, repo)
	body, ct := buildMultipart(t, map[string]string{"description": "", "source": "web"}, "", "", "", nil)
	rec := doRequest(t, h, body, ct)

	if rec.Code != 400 {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
	var m map[string]any
	json.Unmarshal(rec.Body.Bytes(), &m)
	want := "Description must be at least 1 characters."
	if m["error"] != want {
		t.Fatalf("error = %q, want %q", m["error"], want)
	}
}

// (c) source > 50 → 400 MaxLength verbatim.
func TestBugReports_SourceTooLong400(t *testing.T) {
	repo := &fakeRepo{}
	h := newTestHandler(t, repo)
	body, ct := buildMultipart(t, map[string]string{
		"description": "x",
		"source":      strings.Repeat("y", 51),
	}, "", "", "", nil)
	rec := doRequest(t, h, body, ct)

	if rec.Code != 400 {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
	var m map[string]any
	json.Unmarshal(rec.Body.Bytes(), &m)
	want := "source must be shorter than or equal to 50 characters"
	if m["error"] != want {
		t.Fatalf("error = %q, want %q", m["error"], want)
	}
}

// (d) Disallowed mime (image/bmp — not in Node's ALLOWED_MIMES) → 400.
// NOTE: Node's allow-list is png/jpeg/jpg/webp/heic/heif/gif. image/webp is
// ACCEPTED by Node (task guess was wrong). image/bmp is genuinely rejected.
func TestBugReports_DisallowedMime400(t *testing.T) {
	repo := &fakeRepo{}
	h := newTestHandler(t, repo)
	body, ct := buildMultipart(t, map[string]string{
		"description": "with bad file",
		"source":      "web",
	}, "screenshot", "x.bmp", "image/bmp", []byte("BMdata"))
	rec := doRequest(t, h, body, ct)

	if rec.Code != 400 {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
	var m map[string]any
	json.Unmarshal(rec.Body.Bytes(), &m)
	if m["code"] != "invalid-input" {
		t.Fatalf("code = %v, want invalid-input", m["code"])
	}
	want := "only PNG or JPEG screenshots are accepted"
	if m["error"] != want {
		t.Fatalf("error = %q, want %q", m["error"], want)
	}
	if repo.insertCalls != 0 {
		t.Error("rejected mime must not insert")
	}
}

// (d2) image/webp IS allowed by Node → must NOT 400 on the mime gate.
func TestBugReports_WebpAllowed(t *testing.T) {
	repo := &fakeRepo{}
	h := newTestHandler(t, repo)
	body, ct := buildMultipart(t, map[string]string{
		"description": "webp shot",
		"source":      "web",
	}, "screenshot", "x.webp", "image/webp", []byte("RIFFwebp"))
	rec := doRequest(t, h, body, ct)

	// webp passes the mime gate; the storage extensionFor only knows
	// png/jpeg, so Save returns errUnsupportedMime → 400 SAME string, but
	// the point is it is NOT rejected at the controller mime allow-list.
	// Node behaviour: webp passes fileFilter, then service.extensionFor
	// throws 'only PNG or JPEG screenshots are accepted'. So → 400.
	if rec.Code != 400 {
		t.Fatalf("status = %d, want 400 (webp passes filter, fails extensionFor)", rec.Code)
	}
	var m map[string]any
	json.Unmarshal(rec.Body.Bytes(), &m)
	if m["error"] != "only PNG or JPEG screenshots are accepted" {
		t.Fatalf("error = %q", m["error"])
	}
}

// (e) Happy path PNG → 201 {"id","ref","message"} ordered.
func TestBugReports_HappyPathPNG(t *testing.T) {
	repo := &fakeRepo{created: Created{ID: "abc-123", DisplayNo: 42}}
	h := newTestHandler(t, repo)
	body, ct := buildMultipart(t, map[string]string{
		"description": "  it crashed  ",
		"source":      "web",
		"os_info":     "iOS 18",
		"context":     `{"k":"v"}`,
	}, "screenshot", "shot.png", "image/png", []byte("\x89PNGdata"))
	rec := doRequest(t, h, body, ct)

	if rec.Code != 201 {
		t.Fatalf("status = %d, want 201: %s", rec.Code, rec.Body.String())
	}
	raw := rec.Body.String()
	assertKeyOrder(t, raw, "id", "ref", "message")
	var m map[string]any
	json.Unmarshal([]byte(raw), &m)
	if m["id"] != "abc-123" {
		t.Errorf("id = %v, want abc-123", m["id"])
	}
	if m["ref"] != "BUG-42" {
		t.Errorf("ref = %v, want BUG-42", m["ref"])
	}
	if m["message"] != "Bug report received. Thanks! Reference: BUG-42" {
		t.Errorf("message = %v", m["message"])
	}
	if repo.inserted == nil {
		t.Fatal("expected an insert")
	}
	if repo.inserted.Description != "it crashed" {
		t.Errorf("description not trimmed: %q", repo.inserted.Description)
	}
	if repo.inserted.ScreenshotPath == nil {
		t.Error("expected a screenshot path")
	}
	if repo.inserted.Context == nil || string(repo.inserted.Context) != `{"k":"v"}` {
		t.Errorf("context = %s, want raw json", string(repo.inserted.Context))
	}
}

// (e2) context that is not valid JSON → {"raw": <original>} fallback.
func TestBugReports_ContextRawFallback(t *testing.T) {
	repo := &fakeRepo{created: Created{ID: "id", DisplayNo: 1}}
	h := newTestHandler(t, repo)
	body, ct := buildMultipart(t, map[string]string{
		"description": "x",
		"source":      "web",
		"context":     "not json",
	}, "", "", "", nil)
	rec := doRequest(t, h, body, ct)

	if rec.Code != 201 {
		t.Fatalf("status = %d, want 201", rec.Code)
	}
	if repo.inserted == nil || repo.inserted.Context == nil {
		t.Fatal("expected context set")
	}
	var got map[string]any
	json.Unmarshal(repo.inserted.Context, &got)
	if got["raw"] != "not json" {
		t.Errorf("context fallback = %s, want {\"raw\":\"not json\"}", string(repo.inserted.Context))
	}
}

// (f) Oversize file > 5MB → Node multer LIMIT_FILE_SIZE → NestJS
// transformException wraps it in PayloadTooLargeException (HTTP 413).
// AllExceptionsFilter: status 413, code 'http-413' (inferCode default for
// 413), message 'File too large' (multer's LIMIT_FILE_SIZE text).
func TestBugReports_OversizeFile413(t *testing.T) {
	repo := &fakeRepo{}
	h := newTestHandler(t, repo)
	big := bytes.Repeat([]byte("a"), 5*1024*1024+1)
	body, ct := buildMultipart(t, map[string]string{
		"description": "huge shot",
		"source":      "web",
	}, "screenshot", "big.png", "image/png", big)
	rec := doRequest(t, h, body, ct)

	if rec.Code != 413 {
		t.Fatalf("status = %d, want 413", rec.Code)
	}
	var m map[string]any
	json.Unmarshal(rec.Body.Bytes(), &m)
	if m["code"] != "http-413" {
		t.Fatalf("code = %v, want http-413", m["code"])
	}
	if m["error"] != "File too large" {
		t.Fatalf("error = %v, want 'File too large'", m["error"])
	}
	if repo.insertCalls != 0 {
		t.Error("oversize must not insert")
	}
}

// assertKeyOrder fails unless the JSON keys appear in raw in the given order.
func assertKeyOrder(t *testing.T, raw string, keys ...string) {
	t.Helper()
	prev := -1
	for _, k := range keys {
		idx := strings.Index(raw, `"`+k+`"`)
		if idx < 0 {
			t.Fatalf("key %q missing from body: %s", k, raw)
		}
		if idx <= prev {
			t.Fatalf("key %q out of order in body: %s", k, raw)
		}
		prev = idx
	}
}
