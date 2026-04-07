import Foundation

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
}

final class BackendClient {
    private let session: URLSession

    init() {
        let config = URLSessionConfiguration.default
        config.timeoutIntervalForRequest = 60
        self.session = URLSession(configuration: config)
    }

    func rewrite(
        text: String,
        tone: Tone,
        accessToken: String,
        backendUrl: String,
        targetLanguage: String = "English"
    ) async throws -> String {
        DRLogger.log("rewrite request: tone=\(tone.apiValue) textLen=\(text.count) url=\(backendUrl)", category: .api)
        guard !accessToken.isEmpty else {
            DRLogger.log("rewrite FAILED: not logged in", category: .api)
            throw BackendClientError.notLoggedIn
        }

        let base = backendUrl.hasSuffix("/") ? String(backendUrl.dropLast()) : backendUrl
        guard let url = URL(string: "\(base)/rewrite") else {
            DRLogger.log("rewrite FAILED: invalid URL", category: .api)
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
        request.httpBody = try JSONEncoder().encode(body)

        let (data, response): (Data, URLResponse)
        do {
            (data, response) = try await session.data(for: request)
        } catch let error as URLError where error.code == .timedOut {
            DRLogger.log("rewrite FAILED: timeout", category: .api)
            throw BackendClientError.timeout
        }

        let httpStatus = (response as? HTTPURLResponse)?.statusCode ?? -1
        if let httpResponse = response as? HTTPURLResponse, httpResponse.statusCode >= 400 {
            let bodyText = String(data: data, encoding: .utf8) ?? "Unknown error"
            DRLogger.log("rewrite FAILED: HTTP \(httpResponse.statusCode) — \(bodyText.prefix(200))", category: .api)
            throw BackendClientError.httpError(httpResponse.statusCode, bodyText)
        }

        let decoded = try JSONDecoder().decode(BackendRewriteResponse.self, from: data)
        DRLogger.log("rewrite SUCCESS: HTTP \(httpStatus) resultLen=\(decoded.rewritten_text.count)", category: .api)
        return decoded.rewritten_text.trimmingCharacters(in: .whitespacesAndNewlines)
    }

    func checkHealth(backendUrl: String, accessToken: String?) async -> BackendStatus {
        let base = backendUrl.hasSuffix("/") ? String(backendUrl.dropLast()) : backendUrl
        guard let url = URL(string: "\(base)/auth/me") else {
            return .offline
        }

        var request = URLRequest(url: url)
        request.httpMethod = "GET"
        request.timeoutInterval = 5
        if let token = accessToken, !token.isEmpty {
            request.addValue("Bearer \(token)", forHTTPHeaderField: "Authorization")
        }

        do {
            let (_, response) = try await URLSession.shared.data(for: request)
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
