package shared_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	stdhttp "net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/tannpv/draftright-rewrite/internal/shared"
)

// Sentinels mirroring exttoken's (re-declared so this test does not
// import exttoken — shared must never import exttoken, that's a cycle).
var (
	errExtInvalid = errors.New("Invalid extension token")
	errExtScope   = errors.New("Token missing rewrite scope")
)

// fakeExt records the raw token handed to Verify and returns a scripted
// (uid, err). Satisfies shared.ExtVerifier.
type fakeExt struct {
	called bool
	gotRaw string
	uid    string
	err    error
}

func (f *fakeExt) Verify(_ context.Context, raw string) (string, error) {
	f.called = true
	f.gotRaw = raw
	return f.uid, f.err
}

// recordingJWT is a fake JWT middleware. It records that it ran, then
// delegates to next. ExtOrJWT must only invoke it for non-ext bearer
// tokens (Node's JwtAuthGuard fallthrough).
type recordingJWT struct{ ran bool }

func (j *recordingJWT) mw(next stdhttp.Handler) stdhttp.Handler {
	return stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		j.ran = true
		next.ServeHTTP(w, r)
	})
}

// sentinelNext records whether it ran and captures the Sub the
// upstream auth path injected into the context.
type sentinelNext struct {
	ran    bool
	gotSub string
	hadOK  bool
}

func (s *sentinelNext) ServeHTTP(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	s.ran = true
	if c, ok := shared.ClaimsFromContext(r.Context()); ok {
		s.hadOK = true
		s.gotSub = c.Sub
	}
	w.WriteHeader(stdhttp.StatusOK)
	_, _ = io.WriteString(w, "ok")
}

func newReq(authHeader string) *stdhttp.Request {
	req := httptest.NewRequest(stdhttp.MethodPost, "/v1/rewrite", nil)
	if authHeader != "" {
		req.Header.Set("Authorization", authHeader)
	}
	return req
}

func decodeEnvelope(t *testing.T, body []byte) map[string]any {
	t.Helper()
	var m map[string]any
	require.NoError(t, json.Unmarshal(body, &m))
	return m
}

//  1. dr_ext_ valid token → Verify called, next runs with injected
//     Claims{Sub: uid}, JWT middleware NOT invoked.
func TestExtOrJWT_ValidExtToken_InjectsClaims_SkipsJWT(t *testing.T) {
	ext := &fakeExt{uid: "user-123"}
	jwt := &recordingJWT{}
	next := &sentinelNext{}

	h := shared.ExtOrJWT(ext, jwt.mw, slog.Default())(next)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, newReq("Bearer dr_ext_validtoken"))

	require.Equal(t, stdhttp.StatusOK, rec.Code)
	require.True(t, ext.called, "ext.Verify must be called")
	require.Equal(t, "dr_ext_validtoken", ext.gotRaw)
	require.True(t, next.ran, "next handler must run")
	require.True(t, next.hadOK, "claims must be in context")
	require.Equal(t, "user-123", next.gotSub)
	require.False(t, jwt.ran, "JWT middleware must NOT run for an ext token")
}

//  2. dr_ext_ invalid → 401 envelope {error,code,request_id}, code
//     invalid-token, message "Invalid extension token", next NOT run,
//     JWT NOT invoked.
func TestExtOrJWT_InvalidExtToken_401_NextNotRun(t *testing.T) {
	ext := &fakeExt{err: errExtInvalid}
	jwt := &recordingJWT{}
	next := &sentinelNext{}

	h := shared.ExtOrJWT(ext, jwt.mw, slog.Default())(next)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, newReq("Bearer dr_ext_bad"))

	require.Equal(t, stdhttp.StatusUnauthorized, rec.Code)
	env := decodeEnvelope(t, rec.Body.Bytes())
	require.Equal(t, "Invalid extension token", env["error"])
	require.Equal(t, "invalid-token", env["code"])
	_, hasReqID := env["request_id"]
	require.True(t, hasReqID, "envelope must carry request_id")
	require.False(t, next.ran, "next must NOT run on invalid token")
	require.False(t, jwt.ran, "JWT middleware must NOT run for an ext token")
}

// 2b. dr_ext_ missing scope → 401, message "Token missing rewrite scope".
func TestExtOrJWT_ExtMissingScope_401Message(t *testing.T) {
	ext := &fakeExt{err: errExtScope}
	jwt := &recordingJWT{}
	next := &sentinelNext{}

	h := shared.ExtOrJWT(ext, jwt.mw, slog.Default())(next)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, newReq("Bearer dr_ext_noscope"))

	require.Equal(t, stdhttp.StatusUnauthorized, rec.Code)
	env := decodeEnvelope(t, rec.Body.Bytes())
	require.Equal(t, "Token missing rewrite scope", env["error"])
	require.Equal(t, "invalid-token", env["code"])
	require.False(t, next.ran)
	require.False(t, jwt.ran)
}

//  3. Normal JWT token (no dr_ext_ prefix) → ext NOT called, falls
//     through to the JWT middleware (which records it ran).
func TestExtOrJWT_JWTToken_FallsThroughToJWT(t *testing.T) {
	ext := &fakeExt{uid: "should-not-be-used"}
	jwt := &recordingJWT{}
	next := &sentinelNext{}

	h := shared.ExtOrJWT(ext, jwt.mw, slog.Default())(next)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, newReq("Bearer some.jwt.token"))

	require.Equal(t, stdhttp.StatusOK, rec.Code)
	require.False(t, ext.called, "ext.Verify must NOT be called for a JWT")
	require.True(t, jwt.ran, "JWT middleware must run for a non-ext token")
	require.True(t, next.ran)
}

//  4. No Authorization header → Node's RewriteAuthGuard throws
//     "Missing bearer token" BEFORE the ext/JWT split (it does NOT fall
//     through to the JWT branch). Match it: 401, message "Missing bearer
//     token", ext NOT called, JWT NOT invoked.
func TestExtOrJWT_NoHeader_MissingBearerToken(t *testing.T) {
	ext := &fakeExt{}
	jwt := &recordingJWT{}
	next := &sentinelNext{}

	h := shared.ExtOrJWT(ext, jwt.mw, slog.Default())(next)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, newReq(""))

	require.Equal(t, stdhttp.StatusUnauthorized, rec.Code)
	env := decodeEnvelope(t, rec.Body.Bytes())
	require.Equal(t, "Missing bearer token", env["error"])
	require.Equal(t, "invalid-token", env["code"])
	require.False(t, ext.called, "ext must NOT be called when header is absent")
	require.False(t, jwt.ran, "JWT must NOT run when header is absent")
	require.False(t, next.ran)
}

// 4b. Non-Bearer / malformed header → same "Missing bearer token".
func TestExtOrJWT_MalformedHeader_MissingBearerToken(t *testing.T) {
	ext := &fakeExt{}
	jwt := &recordingJWT{}
	next := &sentinelNext{}

	h := shared.ExtOrJWT(ext, jwt.mw, slog.Default())(next)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, newReq("Basic abc123"))

	require.Equal(t, stdhttp.StatusUnauthorized, rec.Code)
	env := decodeEnvelope(t, rec.Body.Bytes())
	require.Equal(t, "Missing bearer token", env["error"])
	require.False(t, ext.called)
	require.False(t, jwt.ran)
}

//  5. ExtVerifier nil + dr_ext_ token → rejected as invalid extension
//     token (no service to verify against). JWT NOT invoked (a dr_ext_
//     opaque string is never a valid JWT).
func TestExtOrJWT_NilExt_ExtTokenRejected(t *testing.T) {
	jwt := &recordingJWT{}
	next := &sentinelNext{}

	h := shared.ExtOrJWT(nil, jwt.mw, slog.Default())(next)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, newReq("Bearer dr_ext_whatever"))

	require.Equal(t, stdhttp.StatusUnauthorized, rec.Code)
	env := decodeEnvelope(t, rec.Body.Bytes())
	require.Equal(t, "Invalid extension token", env["error"])
	require.False(t, jwt.ran)
	require.False(t, next.ran)
}

// 5b. ExtVerifier nil + JWT token → falls through to JWT path unchanged.
func TestExtOrJWT_NilExt_JWTFallsThrough(t *testing.T) {
	jwt := &recordingJWT{}
	next := &sentinelNext{}

	h := shared.ExtOrJWT(nil, jwt.mw, slog.Default())(next)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, newReq("Bearer a.jwt.token"))

	require.Equal(t, stdhttp.StatusOK, rec.Code)
	require.True(t, jwt.ran, "JWT path must run when ext is nil and token is a JWT")
	require.True(t, next.ran)
}
