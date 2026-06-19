package updates

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
)

// fakeAdminRelSvc satisfies adminHandlerService. Fields capture args; configured
// fields drive return values.
type fakeAdminRelSvc struct {
	// ListAll
	view    ReleasesView
	listErr error

	// UpsertChannel
	upRel      AppRelease
	upRelErr   error
	lastChanIn UpsertChannelInput

	// DeleteChannel
	deleteErr   error
	lastDelPlat string
	lastDelChan string

	// UpsertPolicy
	upPol     AppReleasePolicy
	upPolErr  error
	lastPolIn UpsertPolicyInput
}

func (f *fakeAdminRelSvc) ListAll(ctx context.Context) (ReleasesView, error) {
	return f.view, f.listErr
}

func (f *fakeAdminRelSvc) UpsertChannel(ctx context.Context, in UpsertChannelInput) (AppRelease, error) {
	f.lastChanIn = in
	return f.upRel, f.upRelErr
}

func (f *fakeAdminRelSvc) DeleteChannel(ctx context.Context, platform, channel string) error {
	f.lastDelPlat, f.lastDelChan = platform, channel
	return f.deleteErr
}

func (f *fakeAdminRelSvc) UpsertPolicy(ctx context.Context, in UpsertPolicyInput) (AppReleasePolicy, error) {
	f.lastPolIn = in
	return f.upPol, f.upPolErr
}

func newAdminRelH(svc *fakeAdminRelSvc) *AdminHandler {
	return &AdminHandler{svc: svc}
}

// routeWith attaches a chi RouteContext carrying the given params so
// chi.URLParam resolves inside the handler.
func routeWith(req *http.Request, params map[string]string) *http.Request {
	rctx := chi.NewRouteContext()
	for k, v := range params {
		rctx.URLParams.Add(k, v)
	}
	return req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
}

// ---------------------------------------------------------------------------
// R1 — GET /admin/releases
// ---------------------------------------------------------------------------

func TestAdminReleasesList_KeyOrder(t *testing.T) {
	svc := &fakeAdminRelSvc{}
	h := newAdminRelH(svc)

	rec := httptest.NewRecorder()
	h.ListReleases(rec, httptest.NewRequest(http.MethodGet, "/admin/releases", nil))

	if rec.Code != 200 {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	raw := rec.Body.String()
	// Top-level platform order: mac, windows, linux, android, ios.
	order := []string{`"mac"`, `"windows"`, `"linux"`, `"android"`, `"ios"`}
	prev := -1
	for _, key := range order {
		idx := strings.Index(raw, key)
		if idx < 0 {
			t.Fatalf("missing key %s in %s", key, raw)
		}
		if idx <= prev {
			t.Fatalf("key %s out of order in %s", key, raw)
		}
		prev = idx
	}
	// Each platform card: { policy, channels:{ direct, store } }.
	if !strings.Contains(raw, `{"policy":null,"channels":{"direct":null,"store":null}}`) {
		t.Fatalf("platform card shape wrong: %s", raw)
	}
}

// ---------------------------------------------------------------------------
// R2 — POST /admin/releases
// ---------------------------------------------------------------------------

func TestAdminReleaseUpsert_201(t *testing.T) {
	svc := &fakeAdminRelSvc{upRel: AppRelease{Platform: "mac", Channel: "direct", Version: "1.0.0"}}
	h := newAdminRelH(svc)

	body := `{"platform":"mac","version":"1.0.0","download_url":"https://x/app"}`
	rec := httptest.NewRecorder()
	h.UpsertRelease(rec, httptest.NewRequest(http.MethodPost, "/admin/releases", strings.NewReader(body)))

	if rec.Code != 201 {
		t.Fatalf("status = %d, want 201; body=%s", rec.Code, rec.Body.String())
	}
	// Missing channel defaults to "direct" before the use case is called.
	if svc.lastChanIn.Channel != "direct" {
		t.Fatalf("channel default = %q, want direct", svc.lastChanIn.Channel)
	}
	if svc.lastChanIn.Platform != "mac" || svc.lastChanIn.Version != "1.0.0" || svc.lastChanIn.DownloadURL != "https://x/app" {
		t.Fatalf("body not forwarded: %+v", svc.lastChanIn)
	}
	if !strings.Contains(rec.Body.String(), `"platform":"mac"`) {
		t.Fatalf("response missing upserted release: %s", rec.Body.String())
	}
}

func TestAdminReleaseUpsert_ChannelPassthrough(t *testing.T) {
	svc := &fakeAdminRelSvc{}
	h := newAdminRelH(svc)

	body := `{"platform":"mac","channel":"store","version":"1.0.0","download_url":"https://x/app"}`
	rec := httptest.NewRecorder()
	h.UpsertRelease(rec, httptest.NewRequest(http.MethodPost, "/admin/releases", strings.NewReader(body)))

	if svc.lastChanIn.Channel != "store" {
		t.Fatalf("channel = %q, want store", svc.lastChanIn.Channel)
	}
}

func TestAdminReleaseUpsert_ValidationError400(t *testing.T) {
	svc := &fakeAdminRelSvc{upRelErr: errBadPlatform()}
	h := newAdminRelH(svc)

	body := `{"platform":"bogus","version":"1.0.0","download_url":"https://x/app"}`
	rec := httptest.NewRecorder()
	h.UpsertRelease(rec, httptest.NewRequest(http.MethodPost, "/admin/releases", strings.NewReader(body)))

	if rec.Code != 400 {
		t.Fatalf("status = %d, want 400; body=%s", rec.Code, rec.Body.String())
	}
	raw := rec.Body.String()
	if !strings.Contains(raw, `"error":"platform must be one of: mac, windows, linux, android, ios"`) {
		t.Fatalf("validation message wrong: %s", raw)
	}
	if !strings.Contains(raw, `"code":"invalid-input"`) {
		t.Fatalf("validation code wrong: %s", raw)
	}
}

// ---------------------------------------------------------------------------
// R3 — DELETE /admin/releases/{platform}/{channel}
// ---------------------------------------------------------------------------

func TestAdminReleaseDelete_OK(t *testing.T) {
	svc := &fakeAdminRelSvc{}
	h := newAdminRelH(svc)

	req := routeWith(httptest.NewRequest(http.MethodDelete, "/admin/releases/mac/direct", nil),
		map[string]string{"platform": "mac", "channel": "direct"})
	rec := httptest.NewRecorder()
	h.DeleteRelease(rec, req)

	if rec.Code != 200 {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	if strings.TrimSpace(rec.Body.String()) != `{"ok":true}` {
		t.Fatalf("body = %s, want {\"ok\":true}", rec.Body.String())
	}
	if svc.lastDelPlat != "mac" || svc.lastDelChan != "direct" {
		t.Fatalf("params not forwarded: %s/%s", svc.lastDelPlat, svc.lastDelChan)
	}
}

func TestAdminReleaseDelete_NotFound404(t *testing.T) {
	svc := &fakeAdminRelSvc{deleteErr: releaseNotFoundError{msg: "No release row for windows/store"}}
	h := newAdminRelH(svc)

	req := routeWith(httptest.NewRequest(http.MethodDelete, "/admin/releases/windows/store", nil),
		map[string]string{"platform": "windows", "channel": "store"})
	rec := httptest.NewRecorder()
	h.DeleteRelease(rec, req)

	if rec.Code != 404 {
		t.Fatalf("status = %d, want 404; body=%s", rec.Code, rec.Body.String())
	}
	raw := rec.Body.String()
	if !strings.Contains(raw, `"error":"No release row for windows/store"`) {
		t.Fatalf("not-found message wrong: %s", raw)
	}
	if !strings.Contains(raw, `"code":"not-found"`) {
		t.Fatalf("not-found code wrong: %s", raw)
	}
}

// ---------------------------------------------------------------------------
// R4 — POST /admin/release-policies
// ---------------------------------------------------------------------------

func TestAdminPolicyUpsert_201(t *testing.T) {
	svc := &fakeAdminRelSvc{upPol: AppReleasePolicy{Platform: "mac", Preferred: "direct", StoreStatus: "not_submitted"}}
	h := newAdminRelH(svc)

	body := `{"platform":"mac","preferred":"store"}`
	rec := httptest.NewRecorder()
	h.UpsertPolicy(rec, httptest.NewRequest(http.MethodPost, "/admin/release-policies", strings.NewReader(body)))

	if rec.Code != 201 {
		t.Fatalf("status = %d, want 201; body=%s", rec.Code, rec.Body.String())
	}
	if svc.lastPolIn.Platform != "mac" || svc.lastPolIn.Preferred == nil || *svc.lastPolIn.Preferred != "store" {
		t.Fatalf("body not forwarded: %+v", svc.lastPolIn)
	}
	if !strings.Contains(rec.Body.String(), `"platform":"mac"`) {
		t.Fatalf("response missing policy: %s", rec.Body.String())
	}
}

func TestAdminPolicyUpsert_ValidationError400(t *testing.T) {
	svc := &fakeAdminRelSvc{upPolErr: errBadPreferred()}
	h := newAdminRelH(svc)

	body := `{"platform":"mac","preferred":"bogus"}`
	rec := httptest.NewRecorder()
	h.UpsertPolicy(rec, httptest.NewRequest(http.MethodPost, "/admin/release-policies", strings.NewReader(body)))

	if rec.Code != 400 {
		t.Fatalf("status = %d, want 400; body=%s", rec.Code, rec.Body.String())
	}
	raw := rec.Body.String()
	if !strings.Contains(raw, `"error":"preferred must be one of: direct, store"`) {
		t.Fatalf("validation message wrong: %s", raw)
	}
	if !strings.Contains(raw, `"code":"invalid-input"`) {
		t.Fatalf("validation code wrong: %s", raw)
	}
}

// errBadPlatform / errBadPreferred build the exact validation errors the use
// case returns, so the handler test asserts the verbatim 400 message.
func errBadPlatform() error {
	_, err := (&AdminService{}).UpsertChannel(context.Background(),
		UpsertChannelInput{Platform: "bogus", Version: "1", DownloadURL: "u"})
	return err
}

func errBadPreferred() error {
	pref := "bogus"
	_, err := (&AdminService{}).UpsertPolicy(context.Background(),
		UpsertPolicyInput{Platform: "mac", Preferred: &pref})
	return err
}
