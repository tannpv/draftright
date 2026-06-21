package shared

import "net/http"

// httpError is the wire shape every error response uses, identical to
// the Node AllExceptionsFilter envelope: { error, code, request_id }.
// One struct so error bodies never drift between handlers or backends
// (Rule #1 — one place owns the contract).
type httpError struct {
	Error     string `json:"error"`
	Code      string `json:"code"`
	RequestID string `json:"request_id"`
}

// StatusForCode maps a kebab-case error code to its HTTP status,
// reconciled byte-for-byte with the Node backend's
// httpStatusForCode (backend/src/common/error-codes.ts). Codes not
// listed default to 500 — same as Node.
func StatusForCode(code string) int {
	switch code {
	case "invalid-input":
		return http.StatusBadRequest // 400
	case "invalid-token", "user-not-found":
		return http.StatusUnauthorized // 401
	case "quota-exceeded":
		return http.StatusPaymentRequired // 402
	case "forbidden":
		return http.StatusForbidden // 403
	case "not-found":
		return http.StatusNotFound // 404
	case "conflict":
		return http.StatusConflict // 409
	case "rate-limited":
		return http.StatusTooManyRequests // 429
	case "provider-failed":
		return http.StatusBadGateway // 502
	case "provider-unavailable":
		return http.StatusServiceUnavailable // 503
	default:
		return http.StatusInternalServerError // 500
	}
}

// WriteError writes the canonical error envelope. Status is derived
// from the code via StatusForCode, and request_id is pulled from the
// request context (set by the RequestID middleware). Every handler —
// current and future — emits errors through this single function.
func WriteError(w http.ResponseWriter, r *http.Request, code, message string) {
	WriteJSON(w, StatusForCode(code), httpError{
		Error:     message,
		Code:      code,
		RequestID: RequestIDFromContext(r.Context()),
	})
}

// WriteBodyParseError writes the error envelope for a request rejected at
// the JSON body-parsing stage — the Go analogue of an Express body-parser
// SyntaxError. In Node the body-parser throws BEFORE the request-id
// middleware runs, so AllExceptionsFilter emits request_id:"" (empty). We
// mirror that empty request_id byte-for-byte; the populated context
// request-id is deliberately NOT used here. Status still derives from the
// code via StatusForCode.
func WriteBodyParseError(w http.ResponseWriter, code, message string) {
	WriteJSON(w, StatusForCode(code), httpError{
		Error:     message,
		Code:      code,
		RequestID: "",
	})
}
