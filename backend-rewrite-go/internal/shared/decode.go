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

// isNullBody reports whether buf is a top-level literal `null` (whitespace
// trimmed) — the one body Go's encoding/json no-ops into a struct pointer
// without error, but Node's strict Express body-parser rejects with 400.
func isNullBody(buf []byte) bool { return string(bytes.TrimSpace(buf)) == "null" }

// WriteNullBodyError writes the exact Node 400 for a top-level `null` body.
// Node's Express body-parser throws the SyntaxError BEFORE its request-id
// middleware runs, so AllExceptionsFilter emits request_id:"" (empty) —
// WriteBodyParseError mirrors that byte-for-byte. Single owner of the null
// message + its empty request_id so DecodeJSON and the bespoke-decoder sites
// (RejectNullBody callers) never drift (Rule #1).
func WriteNullBodyError(w http.ResponseWriter) {
	WriteBodyParseError(w, "invalid-input", nullBodyMessage)
}

// RejectNullBody buffers r.Body so a caller with a BESPOKE json.Decoder
// (UseNumber / DisallowUnknownFields / a domain-error return) can gain the
// top-level-`null` guard #57 centralized for the uniform sites WITHOUT giving
// up its decoder config. Unlike DecodeJSON it does NOT decode and does NOT
// write on null — the caller owns both, so each site keeps its exact parity.
//
// Returns:
//   - buf:     the fully buffered body; feed it to your own decoder via
//     json.NewDecoder(bytes.NewReader(buf)).
//   - isNull:  true iff the trimmed body is exactly `null`. The caller emits
//     the 400 via WriteNullBodyError (or maps it into its own error channel).
//   - readErr: any error reading r.Body. When r.Body is a http.MaxBytesReader
//     this surfaces *http.MaxBytesError so oversize-handling parity is
//     preserved (e.g. errreport → 500 "request entity too large"). On
//     readErr != nil, isNull is false and buf may be partial — the caller
//     maps readErr exactly as it did when it owned the io.ReadAll.
func RejectNullBody(r *http.Request) (buf []byte, isNull bool, readErr error) {
	buf, readErr = io.ReadAll(r.Body)
	if readErr != nil {
		return buf, false, readErr
	}
	return buf, isNullBody(buf), nil
}

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
	if isNullBody(buf) {
		WriteNullBodyError(w)
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
