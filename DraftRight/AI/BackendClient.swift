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

final class BackendClient {
    private let session: URLSession

    init() {
        let config = URLSessionConfiguration.default
        config.timeoutIntervalForRequest = 15
        self.session = URLSession(configuration: config)
    }

    func rewrite(
        text: String,
        tone: Tone,
        accessToken: String,
        backendUrl: String,
        targetLanguage: String = "English"
    ) async throws -> String {
        guard !accessToken.isEmpty else { throw BackendClientError.notLoggedIn }

        let base = backendUrl.hasSuffix("/") ? String(backendUrl.dropLast()) : backendUrl
        guard let url = URL(string: "\(base)/rewrite") else { throw BackendClientError.invalidURL }

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
            throw BackendClientError.timeout
        }

        if let httpResponse = response as? HTTPURLResponse, httpResponse.statusCode >= 400 {
            let bodyText = String(data: data, encoding: .utf8) ?? "Unknown error"
            throw BackendClientError.httpError(httpResponse.statusCode, bodyText)
        }

        let decoded = try JSONDecoder().decode(BackendRewriteResponse.self, from: data)
        return decoded.rewritten_text.trimmingCharacters(in: .whitespacesAndNewlines)
    }
}
