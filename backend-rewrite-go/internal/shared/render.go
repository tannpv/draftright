package shared

import (
	"encoding/json"
	"net/http"
)

// WriteJSON serialises v as the response body with the given status.
// Single helper so every handler — across modules — emits the same
// content type + encoding (Rule #1: one place owns the wire write).
func WriteJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
