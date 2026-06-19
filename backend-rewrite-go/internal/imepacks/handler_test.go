package imepacks

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestManifest_200AndShape(t *testing.T) {
	h := NewHandler()
	rec := httptest.NewRecorder()
	h.Manifest(rec, httptest.NewRequest(http.MethodGet, "/ime-packs/manifest", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var body struct {
		Languages []LanguageModule `json:"languages"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(body.Languages) != 10 {
		t.Fatalf("languages len = %d, want 10", len(body.Languages))
	}
	// es (index 3) is bundled with no packs → neither key present in JSON.
	raw := rec.Body.String()
	esObj := raw[strings.Index(raw, `"id":"es"`):]
	esObj = esObj[:strings.Index(esObj, "}")+1]
	if strings.Contains(esObj, "pack") {
		t.Errorf("es entry must omit pack/wordlistPack, got %s", esObj)
	}
}
