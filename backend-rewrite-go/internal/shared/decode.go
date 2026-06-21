package shared

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
)

// DecodeMode controls tolerance for empty / malformed request bodies.
// A top-level literal `null` body is ALWAYS rejected (Node Express
// strict-body-parser parity), regardless of mode.
type DecodeMode int

const (
	// DecodeStrict: empty OR malformed body → 400. Mirrors a call site
	// that did `if err != nil { 400 }`.
	DecodeStrict DecodeMode = iota
	// DecodeOptional: empty body proceeds (zero value); malformed → 400.
	// Mirrors `if err != nil && err != io.EOF { 400 }`.
	DecodeOptional
	// DecodeLenient: empty OR malformed body proceeds. Mirrors `_ = Decode(...)`.
	DecodeLenient
)

// nullBodyMessage is the exact SyntaxError Express's strict body-parser
// surfaces for a top-level literal `null` body, mapped through Nest's
// AllExceptionsFilter to 400 invalid-input. Pinned to the running prod
// Node version — the live shadow gate (Node :3200) is the authority. If
// the gate reports a mismatch, replace this with Node's actual bytes.
const nullBodyMessage = `Unexpected token 'n', "null" is not valid JSON`

// DecodeJSON reads the request body and unmarshals JSON into dst. It
// intercepts a top-level literal `null` body — which Go's encoding/json
// silently no-ops into a struct pointer (no error), but Node rejects —
// and emits Node's 400. Every other path replays the original
// json.NewDecoder(r.Body).Decode(dst) byte-for-byte so empty / malformed
// / trailing-data behavior is unchanged; only mode decides how the
// replayed error is handled. Returns true to continue, false if an error
// response was already written.
func DecodeJSON(w http.ResponseWriter, r *http.Request, dst any, mode DecodeMode) bool {
	buf, readErr := io.ReadAll(r.Body)
	if readErr != nil {
		if mode == DecodeLenient {
			return true
		}
		WriteError(w, r, "invalid-input", "Invalid request body")
		return false
	}

	// Top-level literal null: the one case Go's decoder accepts but Node
	// rejects. Whitespace-tolerant (Node trims), matches Node's strict mode.
	// This is a body-parser-level rejection — Node throws here BEFORE its
	// request-id middleware runs, so the envelope carries request_id:"".
	// WriteBodyParseError mirrors that empty request_id byte-for-byte.
	if string(bytes.TrimSpace(buf)) == "null" {
		WriteBodyParseError(w, "invalid-input", nullBodyMessage)
		return false
	}

	// Replay the exact original decode (Decoder, NOT Unmarshal — Decoder
	// stops after the first JSON value, tolerating trailing data exactly
	// as the prior call sites did).
	err := json.NewDecoder(bytes.NewReader(buf)).Decode(dst)
	switch mode {
	case DecodeStrict:
		if err != nil {
			WriteError(w, r, "invalid-input", "Invalid request body")
			return false
		}
	case DecodeOptional:
		if err != nil && err != io.EOF {
			WriteError(w, r, "invalid-input", "Invalid request body")
			return false
		}
	case DecodeLenient:
		// errors ignored — proceed with whatever decoded (or zero value).
	}
	return true
}
