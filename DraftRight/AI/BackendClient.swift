import Foundation

extension String {
    var strippingTrailingSlash: String {
        hasSuffix("/") ? String(dropLast()) : self
    }
}

struct BackendRewriteRequest: Codable {
    let text: String
    let tone: String
    let target_language: String?
}

struct BackendRewriteResponse: Codable {
    let rewritten_text: String
    let usage_today: Int?
    let daily_limit: Int?
}

/// Subset of GET /auth/me we care about right now. Adding more flags
/// in the future = one extra Codable property; the rest of the
/// AuthFlags decode survives unknown server-side fields.
struct AuthMeFlags: Codable {
    let use_go_backend: Bool?
}
struct AuthMeResponse: Codable {
    let id: String
    let email: String
    let role: String
    let flags: AuthMeFlags?
}

/// Identifies which backend the client chose for an outbound /rewrite
/// call.  Server-side observability (Caddy access log + Prom counter)
/// uses the matching X-DraftRight-Backend header to verify routing
/// matches the cohort assignment computed in /auth/me.
///
/// Adding a third backend later (e.g. dedicated grpc / regional) =
/// one new case + one new `path` mapping + one new env-override
/// keyword. No scattered conditionals in callers (Rule #1 — open
/// for extension, closed for modification).
enum RewriteBackend: String, CaseIterable {
    case nestjs
    case go

    /// HTTP path on the API host that serves this backend.
    var path: String {
        switch self {
        case .nestjs: return "/rewrite"
        case .go:     return "/v1/rewrite"
        }
    }

    /// Parses an env-override string (the DRAFTRIGHT_FORCE_BACKEND
    /// QA escape hatch). Case-insensitive, whitespace-tolerant.
    /// Returns nil when the value doesn't name a known backend so
    /// the caller can fall back to the server-decided default.
    static func fromOverride(_ raw: String?) -> RewriteBackend? {
        guard let raw, !raw.isEmpty else { return nil }
        return RewriteBackend(
            rawValue: raw.trimmingCharacters(in: .whitespacesAndNewlines).lowercased()
        )
    }
}

enum BackendClientError: LocalizedError {
    case notLoggedIn
    case invalidURL
    case emptyResponse
    case httpError(Int, String)
    case timeout

    var errorDescription: String? {
        switch self {
        case .notLoggedIn: return "Not signed in. Open Settings to log in."
        case .invalidURL: return "Invalid backend URL."
        case .emptyResponse: return "No text returned from server."
        case .httpError(let code, let body): return "HTTP \(code): \(body)"
        case .timeout: return "Request timed out."
        }
    }
}

enum BackendStatus {
    case connected
    case notLoggedIn
    case offline
    case wrongServer
}

private struct HealthResponse: Codable {
    let app: String
    let version: String
    let status: String
    // Admin-controlled log verbosity; absent on older backends → nil.
    let client_log_level: String?
}

final class BackendClient {
    private let session: URLSession

    init() {
        let config = URLSessionConfiguration.default
        // Idle-gap timeout: connection killed if no bytes received
        // within this window. 30s catches dead TCP / wedged proxies
        // without punishing a real-but-slow upstream.
        config.timeoutIntervalForRequest = 30
        // Total ceiling. NestJS /rewrite waits synchronously for the
        // upstream LLM; Ollama Cloud free tier can burst to 60-120s
        // under shared-queue pressure (logged real-world max: 124s).
        // 180s covers ~99% of valid responses; bad network still
        // surfaces a timeout (just not at 60s, where the user used
        // to see "Request timed out" while the backend was still
        // producing a correct answer).
        config.timeoutIntervalForResource = 180
        self.session = URLSession(configuration: config)
    }

    func rewrite(
        text: String,
        tone: Tone,
        accessToken: String,
        backendUrl: String,
        backend: RewriteBackend = .nestjs,
        targetLanguage: String = "English",
        refreshToken: String = "",
        onTokensRefreshed: ((String, String) -> Void)? = nil
    ) async throws -> String {
        DRLogger.log("rewrite request: backend=\(backend.rawValue) tone=\(tone.apiValue) textLen=\(text.count) url=\(backendUrl)", category: .api)
        guard !accessToken.isEmpty else {
            DRLogger.error("rewrite FAILED: not logged in", category: .api)
            throw BackendClientError.notLoggedIn
        }

        let base = backendUrl.strippingTrailingSlash
        guard let url = URL(string: "\(base)\(backend.path)") else {
            DRLogger.error("rewrite FAILED: invalid URL", category: .api)
            throw BackendClientError.invalidURL
        }

        let inputText = String(text.prefix(3000))
        let body = BackendRewriteRequest(
            text: inputText,
            tone: tone.apiValue,
            target_language: tone == .translate ? targetLanguage : nil
        )

        var request = URLRequest(url: url)
        request.httpMethod = "POST"
        request.addValue("application/json", forHTTPHeaderField: "Content-Type")
        request.addValue("Bearer \(accessToken)", forHTTPHeaderField: "Authorization")
        // Observability: server-side log + Prom counter on this header
        // checks routing matches the cohort assignment in /auth/me.
        request.addValue(backend.rawValue, forHTTPHeaderField: "X-DraftRight-Backend")
        request.httpBody = try JSONEncoder().encode(body)

        var (data, response): (Data, URLResponse)
        do {
            (data, response) = try await session.data(for: request)
        } catch let error as URLError where error.code == .timedOut {
            DRLogger.error("rewrite FAILED: timeout", category: .api)
            throw BackendClientError.timeout
        }

        // Auto-refresh + retry once on 401. The 30s health-check loop
        // also refreshes silently in the background, but a rewrite that
        // fires *between* health checks (right after access token expires)
        // would otherwise return 401 to the user. Closing that window.
        if let httpResponse = response as? HTTPURLResponse,
           httpResponse.statusCode == 401,
           !refreshToken.isEmpty {
            DRLogger.log("rewrite got 401 — attempting silent refresh + retry", category: .api)
            if case .success(let access, let refresh) = await refreshTokens(refreshToken: refreshToken, backendUrl: backendUrl) {
                onTokensRefreshed?(access, refresh)
                request.setValue("Bearer \(access)", forHTTPHeaderField: "Authorization")
                do {
                    (data, response) = try await session.data(for: request)
                } catch let error as URLError where error.code == .timedOut {
                    throw BackendClientError.timeout
                }
            }
        }

        let httpStatus = (response as? HTTPURLResponse)?.statusCode ?? -1
        if let httpResponse = response as? HTTPURLResponse, httpResponse.statusCode >= 400 {
            let bodyText = String(data: data, encoding: .utf8) ?? "Unknown error"
            DRLogger.error("rewrite FAILED: HTTP \(httpResponse.statusCode) — \(bodyText.prefix(200))", category: .api)
            throw BackendClientError.httpError(httpResponse.statusCode, bodyText)
        }

        // Check if this is a grammar check response
        if let json = try? JSONSerialization.jsonObject(with: data) as? [String: Any],
           let grammarDict = json["grammar"] {
            let grammarData = try JSONSerialization.data(withJSONObject: grammarDict)
            let jsonString = String(data: grammarData, encoding: .utf8) ?? "{}"
            DRLogger.log("rewrite SUCCESS: HTTP \(httpStatus) grammarCheck resultLen=\(jsonString.count)", category: .api)
            return jsonString
        }

        // Decode the rewrite payload. NestJS uses {"rewritten_text":"..."}
        // (legacy shape); the Go service returns {"text":"...","service":"rewrite-go"}.
        // Branch on the backend that was called so both shapes are accepted
        // transparently — clients don't need separate code paths.
        let rewrittenText: String
        switch backend {
        case .nestjs:
            let decoded = try JSONDecoder().decode(BackendRewriteResponse.self, from: data)
            rewrittenText = decoded.rewritten_text
        case .go:
            let decoded = try JSONDecoder().decode(GoRewriteResponse.self, from: data)
            rewrittenText = decoded.text
        }
        DRLogger.log("rewrite SUCCESS: HTTP \(httpStatus) backend=\(backend.rawValue) resultLen=\(rewrittenText.count)", category: .api)
        return rewrittenText.trimmingCharacters(in: .whitespacesAndNewlines)
    }

    /// Go service /v1/rewrite JSON response shape (non-SSE path).
    private struct GoRewriteResponse: Codable {
        let text: String
        let service: String?
    }

    /// Result of a refresh attempt. Distinguishes a server-rejected token (401)
    /// from a transient failure (network, 5xx, malformed body) so the caller can
    /// decide whether to wipe credentials or keep them and retry later.
    enum RefreshResult {
        case success(access: String, refresh: String)
        case unauthorized        // backend explicitly rejected the token — safe to clear
        case transient           // network / 5xx / decode error — DO NOT clear
    }

    /// POSTs to /auth/refresh with the refresh token.
    /// Caller is responsible for persisting the new tokens.
    func refreshTokens(refreshToken: String, backendUrl: String) async -> RefreshResult {
        guard !refreshToken.isEmpty else { return .unauthorized }
        let base = backendUrl.strippingTrailingSlash
        guard let url = URL(string: "\(base)/auth/refresh") else { return .transient }

        var request = URLRequest(url: url)
        request.httpMethod = "POST"
        request.timeoutInterval = 10
        request.addValue("application/json", forHTTPHeaderField: "Content-Type")
        request.httpBody = try? JSONSerialization.data(withJSONObject: ["refresh_token": refreshToken])

        do {
            let (data, response) = try await session.data(for: request)
            guard let http = response as? HTTPURLResponse else {
                DRLogger.error("refreshTokens FAILED: non-HTTP response", category: .auth)
                return .transient
            }
            switch http.statusCode {
            case 200...299:
                guard let json = try? JSONSerialization.jsonObject(with: data) as? [String: Any],
                      let access = json["access_token"] as? String,
                      let refresh = json["refresh_token"] as? String else {
                    DRLogger.error("refreshTokens FAILED: bad response shape", category: .auth)
                    return .transient
                }
                DRLogger.log("refreshTokens SUCCESS", category: .auth)
                return .success(access: access, refresh: refresh)
            case 401, 403:
                DRLogger.error("refreshTokens UNAUTHORIZED: HTTP \(http.statusCode) — clearing", category: .auth)
                return .unauthorized
            default:
                DRLogger.warn("refreshTokens TRANSIENT: HTTP \(http.statusCode) — keeping tokens", category: .auth)
                return .transient
            }
        } catch {
            DRLogger.warn("refreshTokens TRANSIENT: \(error.localizedDescription) — keeping tokens", category: .auth)
            return .transient
        }
    }

    /// Fetch GET /auth/me with the supplied access token. Returns the
    /// parsed body on 200, nil on any other status / parse failure /
    /// network error. Used by AppModel to refresh the server-controlled
    /// feature flags (currently flags.use_go_backend) so the next
    /// rewrite call can route to the correct backend.
    func fetchAuthMe(backendUrl: String, accessToken: String) async -> AuthMeResponse? {
        let base = backendUrl.strippingTrailingSlash
        guard !accessToken.isEmpty,
              let url = URL(string: "\(base)/auth/me") else {
            return nil
        }
        var request = URLRequest(url: url)
        request.httpMethod = "GET"
        request.timeoutInterval = 5
        request.addValue("Bearer \(accessToken)", forHTTPHeaderField: "Authorization")
        do {
            let (data, response) = try await session.data(for: request)
            guard let http = response as? HTTPURLResponse, http.statusCode == 200 else {
                return nil
            }
            return try? JSONDecoder().decode(AuthMeResponse.self, from: data)
        } catch {
            return nil
        }
    }

    func checkHealth(backendUrl: String, accessToken: String?) async -> BackendStatus {
        let base = backendUrl.strippingTrailingSlash

        // Step 1: Check /health for app identity
        guard let healthUrl = URL(string: "\(base)/health") else {
            return .offline
        }

        var healthRequest = URLRequest(url: healthUrl)
        healthRequest.httpMethod = "GET"
        healthRequest.timeoutInterval = 5

        do {
            let (data, response) = try await URLSession.shared.data(for: healthRequest)
            guard let http = response as? HTTPURLResponse, http.statusCode == 200 else {
                return .offline
            }

            let health = try JSONDecoder().decode(HealthResponse.self, from: data)
            guard health.app == "draftright" else {
                return .wrongServer
            }
            // Apply admin-controlled log verbosity only once we've confirmed
            // this is really a DraftRight backend (a rogue server shouldn't be
            // able to silence our logs).
            DRLogger.setMinLevelFromServer(health.client_log_level)
        } catch {
            return .offline
        }

        // Step 2: Check /auth/me for login state
        guard let authUrl = URL(string: "\(base)/auth/me") else {
            return .offline
        }

        var authRequest = URLRequest(url: authUrl)
        authRequest.httpMethod = "GET"
        authRequest.timeoutInterval = 5
        if let token = accessToken, !token.isEmpty {
            authRequest.addValue("Bearer \(token)", forHTTPHeaderField: "Authorization")
        }

        do {
            let (_, response) = try await URLSession.shared.data(for: authRequest)
            guard let http = response as? HTTPURLResponse else { return .offline }
            switch http.statusCode {
            case 200:
                return .connected
            case 401:
                return .notLoggedIn
            default:
                return .offline
            }
        } catch {
            return .offline
        }
    }
}
