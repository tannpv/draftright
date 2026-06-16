// admin_middleware.go — RequireAdmin, the Go port of the NestJS
// RolesGuard('admin'). Mounted AFTER RequireAuth, which stamps the
// verified claims and owns the no-token 401; RequireAdmin only checks
// the role.
package shared

import "net/http"

// RequireAdmin allows the request iff the verified claims carry isAdmin=true OR
// role=="admin" (Node RolesGuard: `user.isAdmin || user.role === 'admin'`).
// Otherwise it writes the 403 envelope {error:"Admin access required",
// code:"forbidden", request_id} — byte-identical to the global filter's output
// for a bare ForbiddenException('Admin access required') (inferCode(403) =
// ERROR_CODES.forbidden). A missing claim (route not wrapped by RequireAuth)
// is treated as non-admin; wired admin routes always sit behind RequireAuth.
func RequireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, ok := ClaimsFromContext(r.Context())
		if !ok || !(c.IsAdminFlag || c.Role == "admin") {
			WriteError(w, r, "forbidden", "Admin access required")
			return
		}
		next.ServeHTTP(w, r)
	})
}
