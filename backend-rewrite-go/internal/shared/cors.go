package shared

import "net/http"

// corsAllowMethods mirrors the Node `cors` package default
// (methods: 'GET,HEAD,PUT,PATCH,POST,DELETE'). NestJS calls
// app.enableCors() with no options, so every cors default applies.
const corsAllowMethods = "GET,HEAD,PUT,PATCH,POST,DELETE"

// CORS replicates NestJS `app.enableCors()` (the `cors` npm package with
// its default options) so the Go port matches Node byte-for-byte:
//
//   - every response carries `Access-Control-Allow-Origin: *`
//   - an OPTIONS preflight returns 204 with Allow-Methods, the reflected
//     Allow-Headers, and `Vary: Access-Control-Request-Headers`
//
// Credentials are NOT enabled (cors default) so there is no
// Access-Control-Allow-Credentials header — matching Node. This runs as the
// outermost middleware so even 401/404 responses carry the ACAO header,
// exactly as Node's global cors middleware does (it executes before routing).
func CORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		h := w.Header()
		h.Set("Access-Control-Allow-Origin", "*")

		if req.Method == http.MethodOptions {
			// Preflight. The cors package always varies on the request
			// headers it reflects (allowedHeaders is unset by default) and
			// echoes the requested headers back, omitting Allow-Headers when
			// none were requested.
			h.Set("Access-Control-Allow-Methods", corsAllowMethods)
			h.Add("Vary", "Access-Control-Request-Headers")
			if reqHeaders := req.Header.Get("Access-Control-Request-Headers"); reqHeaders != "" {
				h.Set("Access-Control-Allow-Headers", reqHeaders)
			}
			h.Set("Content-Length", "0")
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, req)
	})
}
