package shared

import (
	"context"
	"log/slog"
	"net/http"
	"strings"

	auth "github.com/tannpv/draftright-rewrite/internal/platform/auth"
)

// extTokenPrefix mirrors exttoken.TokenPrefix ("dr_ext_"). It is
// re-declared here, not imported, on purpose: the exttoken package
// imports shared (its handler uses shared.WriteError), so shared
// importing exttoken would be an import cycle. The ExtVerifier port
// below is declared here for the same reason — the consumer owns the
// interface (Rule: accept interfaces). Keep this constant in sync with
// exttoken.TokenPrefix; a domain_test in exttoken pins that value.
const extTokenPrefix = "dr_ext_"

// ExtVerifier is the consumer-side port for extension-token
// verification. Satisfied structurally by *exttoken.Service.Verify.
// The returned error's Error() string is surfaced verbatim as the 401
// message, byte-for-byte matching Node's UnauthorizedException text
// ("Invalid extension token" / "Token missing rewrite scope" /
// "Missing bearer token").
type ExtVerifier interface {
	Verify(ctx context.Context, raw string) (userID string, err error)
}

// ExtOrJWT guards /v1/rewrite with dual auth, mirroring Node's
// RewriteAuthGuard exactly:
//
//  1. No "Bearer <token>" header → 401 "Missing bearer token". Node
//     throws this from the guard itself BEFORE the ext/JWT split, so it
//     is reproduced here (NOT delegated to the JWT middleware, which
//     would emit the generic "Unauthorized"). When ext is non-nil this
//     comes back as the verifier's ErrMissingToken (raw == ""); when
//     ext is nil we emit the same message inline so the two mounts agree.
//  2. Token prefixed dr_ext_ → verified as an extension token; on
//     success the resolved owner is injected as auth.Claims{Sub: uid}
//     under the same context key RequireAuth uses, so the rewrite
//     handler is agnostic to which path authenticated. On failure the
//     verifier's sentinel message is surfaced as a 401.
//  3. Any other (JWT) token → delegated UNCHANGED to the jwt middleware
//     (the RequireAuth-wrapped handler), preserving byte-identical JWT
//     behavior — including its "Unauthorized" message on a bad/expired
//     token.
//
// jwt is the already-built RequireAuth middleware value applied to next.
func ExtOrJWT(ext ExtVerifier, jwt func(http.Handler) http.Handler, log *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		// The unchanged RequireAuth-wrapped handler for the JWT branch.
		jwtPath := jwt(next)
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			raw, ok := extractBearer(r.Header.Get("Authorization"))
			if !ok {
				// Node: no/!Bearer header → UnauthorizedException
				// ('Missing bearer token'). code = inferCode(401) =
				// invalid-token. Match it verbatim.
				WriteError(w, r, "invalid-token", "Missing bearer token")
				return
			}
			if strings.HasPrefix(raw, extTokenPrefix) {
				if ext == nil {
					// A dr_ext_ token arrived but no extension service is
					// wired (no DB). Node would always have the service;
					// the closest parity is to reject as an invalid
					// extension token rather than try JWT (a dr_ext_
					// opaque string is never a valid JWT).
					WriteError(w, r, "invalid-token", "Invalid extension token")
					return
				}
				uid, err := ext.Verify(r.Context(), raw)
				if err != nil {
					// 401 with the sentinel message verbatim; code is
					// invalid-token (Node inferCode(401)). Logged at
					// debug like RequireAuth — auth failures are normal.
					log.Debug("rewrite-auth: extension token rejected", "err", err.Error())
					WriteError(w, r, "invalid-token", err.Error())
					return
				}
				// Inject the resolved owner under the SAME key RequireAuth
				// uses so ClaimsFromContext works downstream. Only Sub is
				// known (and only Sub is read by the rewrite handler);
				// Node's ext path likewise only resolves the user id.
				ctx := ContextWithClaims(r.Context(), &auth.Claims{Sub: uid})
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}
			// Non-ext bearer token → identical JWT behavior.
			jwtPath.ServeHTTP(w, r)
		})
	}
}
