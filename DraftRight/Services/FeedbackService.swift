import Foundation

/// Posts feature requests to the backend `POST /feedback` endpoint
/// (JSON body; no screenshot). Mirrors `BugReportService` but for feature
/// requests rather than bug reports.
enum FeedbackService {
    static let source = "macos-app"

    enum FeedbackError: LocalizedError {
        case invalidURL
        case httpError(Int, String)
        case emptyResponse
        case network(Error)

        var errorDescription: String? {
            switch self {
            case .invalidURL: return "Invalid backend URL."
            case .httpError(let code, let body): return "HTTP \(code): \(body)"
            case .emptyResponse: return "No response from server."
            case .network(let err): return err.localizedDescription
            }
        }
    }

    struct CreateResponse: Decodable { let id: String }

    /// Submit a feature request.
    /// - Parameters:
    ///   - title: One-line summary of the requested feature.
    ///   - targetPlatform: One of playground|mobile|windows|mac|linux.
    ///   - description: Detailed description.
    ///   - userEmail: Optional email for anonymous flow (ignored when `authToken` present).
    ///   - authToken: Optional Bearer JWT — pass non-empty access token when signed in.
    ///   - backendUrl: Base URL e.g. `https://api.draftright.info`.
    /// - Returns: Server-assigned record ID (may be empty string if server omits it).
    static func submitFeatureRequest(
        title: String,
        targetPlatform: String,
        description: String,
        userEmail: String?,
        authToken: String?,
        backendUrl: String
    ) async throws -> String {
        let base = backendUrl.strippingTrailingSlash
        guard let url = URL(string: "\(base)/feedback") else {
            throw FeedbackError.invalidURL
        }

        var request = URLRequest(url: url)
        request.httpMethod = "POST"
        request.timeoutInterval = 30
        request.setValue("application/json", forHTTPHeaderField: "Content-Type")
        if let token = authToken, !token.isEmpty {
            request.setValue("Bearer \(token)", forHTTPHeaderField: "Authorization")
        }

        var body: [String: Any] = [
            "kind": "feature",
            "title": title.trimmingCharacters(in: .whitespacesAndNewlines),
            "target_platform": targetPlatform,
            "description": description.trimmingCharacters(in: .whitespacesAndNewlines),
            "source": source,
        ]
        // Only include email in anonymous flow — authenticated requests bind to the account
        if (authToken ?? "").isEmpty,
           let email = userEmail?.trimmingCharacters(in: .whitespaces),
           !email.isEmpty {
            body["user_email"] = email
        }

        let data: Data
        let response: URLResponse
        do {
            request.httpBody = try JSONSerialization.data(withJSONObject: body)
            (data, response) = try await URLSession.shared.data(for: request)
        } catch let err as FeedbackError {
            throw err
        } catch {
            throw FeedbackError.network(error)
        }

        guard let http = response as? HTTPURLResponse else {
            throw FeedbackError.emptyResponse
        }
        if http.statusCode >= 400 {
            let bodyText = String(data: data, encoding: .utf8) ?? ""
            throw FeedbackError.httpError(http.statusCode, bodyText)
        }

        return (try? JSONDecoder().decode(CreateResponse.self, from: data))?.id ?? ""
    }
}
