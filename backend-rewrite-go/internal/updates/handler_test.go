package updates

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

type stubSvc struct{ all map[string]*Release }

func (s stubSvc) listEffective(context.Context, string) (map[string]*Release, error) {
	return s.all, nil
}

func TestLatest_ShapeAndPlatformsOmitNulls(t *testing.T) {
	all := map[string]*Release{
		"mac":     {Platform: "mac", Version: "2.2.9", DownloadURL: "https://x/mac", SHA256: "abc", ReleaseNotes: "n", Required: false, Channel: "direct"},
		"windows": nil, "linux": nil, "android": nil, "ios": nil,
	}
	h := &Handler{svc: stubSvc{all: all}}
	rec := httptest.NewRecorder()
	h.Latest(rec, httptest.NewRequest(http.MethodGet, "/updates/latest", nil))
	if rec.Code != 200 {
		t.Fatalf("status = %d", rec.Code)
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if body["mac_url"] != "https://x/mac" || body["windows_url"] != "" {
		t.Errorf("top-level urls wrong: %v", body)
	}
	plats := body["platforms"].(map[string]any)
	if _, ok := plats["mac"]; !ok {
		t.Error("mac must be in platforms")
	}
	if _, ok := plats["windows"]; ok {
		t.Error("null windows must be omitted from platforms")
	}
}

func TestLatest_EmptyDB200(t *testing.T) {
	all := map[string]*Release{"mac": nil, "windows": nil, "linux": nil, "android": nil, "ios": nil}
	h := &Handler{svc: stubSvc{all: all}}
	rec := httptest.NewRecorder()
	h.Latest(rec, httptest.NewRequest(http.MethodGet, "/updates/latest", nil))
	if rec.Code != 200 {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if body["version"] != "" {
		t.Errorf("version = %v, want empty", body["version"])
	}
	if len(body["platforms"].(map[string]any)) != 0 {
		t.Errorf("platforms must be empty")
	}
}
