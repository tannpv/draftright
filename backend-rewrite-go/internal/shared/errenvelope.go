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

// ErrorCode is a kebab-case error code. Naming every code as a constant gives
// the error contract ONE source of truth: a call site referencing CodeInvalidInput
// can't silently drift into a typo that StatusForCode then degrades to a 500 with
// no compile or test failure — the invisible-drift failure mode of #22 (#204).
type ErrorCode string

const (
	// CodeInternal is the generic 500 — deliberately NOT in statusByCode; it IS
	// the default.
	CodeInternal            ErrorCode = "internal"
	CodeInvalidInput        ErrorCode = "invalid-input"
	CodeInvalidToken        ErrorCode = "invalid-token"
	CodeUserNotFound        ErrorCode = "user-not-found"
	CodeQuotaExceeded       ErrorCode = "quota-exceeded"
	CodeForbidden           ErrorCode = "forbidden"
	CodeNotFound            ErrorCode = "not-found"
	CodeConflict            ErrorCode = "conflict"
	CodeRateLimited         ErrorCode = "rate-limited"
	CodeProviderFailed      ErrorCode = "provider-failed"
	CodeProviderUnavailable ErrorCode = "provider-unavailable"
)

// AllErrorCodes is the registry of every declared code. The guard test iterates
// it to assert each has an explicit status wired below (CodeInternal excepted —
// it is the default 500). Add a new Code* constant → add it here too.
var AllErrorCodes = []ErrorCode{
	CodeInternal, CodeInvalidInput, CodeInvalidToken, CodeUserNotFound,
	CodeQuotaExceeded, CodeForbidden, CodeNotFound, CodeConflict,
	CodeRateLimited, CodeProviderFailed, CodeProviderUnavailable,
}

// statusByCode is the single source for code→HTTP-status, reconciled
// byte-for-byte with the Node backend's httpStatusForCode. CodeInternal is
// intentionally absent — it maps to the default 500.
var statusByCode = map[ErrorCode]int{
	CodeInvalidInput:        http.StatusBadRequest,         // 400
	CodeInvalidToken:        http.StatusUnauthorized,       // 401
	CodeUserNotFound:        http.StatusUnauthorized,       // 401
	CodeQuotaExceeded:       http.StatusPaymentRequired,    // 402
	CodeForbidden:           http.StatusForbidden,          // 403
	CodeNotFound:            http.StatusNotFound,           // 404
	CodeConflict:            http.StatusConflict,           // 409
	CodeRateLimited:         http.StatusTooManyRequests,    // 429
	CodeProviderFailed:      http.StatusBadGateway,         // 502
	CodeProviderUnavailable: http.StatusServiceUnavailable, // 503
}

// StatusForCode maps a kebab-case error code to its HTTP status. Unlisted codes
// (including CodeInternal) default to 500 — same as Node.
func StatusForCode(code string) int {
	if s, ok := statusByCode[ErrorCode(code)]; ok {
		return s
	}
	return http.StatusInternalServerError
}

// WriteError writes the canonical error envelope. Status is derived
// from the code via StatusForCode, and request_id is pulled from the
// request context (set by the RequestID middleware). Every handler —
// current and future — emits errors through this single function.
func WriteError(w http.ResponseWriter, r *http.Request, code ErrorCode, message string) {
	WriteJSON(w, StatusForCode(string(code)), httpError{
		Error:     message,
		Code:      string(code),
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
func WriteBodyParseError(w http.ResponseWriter, code ErrorCode, message string) {
	WriteJSON(w, StatusForCode(string(code)), httpError{
		Error:     message,
		Code:      string(code),
		RequestID: "",
	})
}
