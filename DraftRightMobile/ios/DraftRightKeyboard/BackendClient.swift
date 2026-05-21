import Foundation

struct RewriteRequest: Codable {
    let text: String
    let tone: String
    let target_language: String?
}

struct RewriteResponse: Codable {
    let rewritten_text: String
    let usage_today: Int?
    let daily_limit: Int?
}

final class BackendClient {
    private enum Config {
        /// Backend caps rewrite input length; truncate client-side to match.
        static let maxInputChars = 3000
        /// Request timeout for the /rewrite call (seconds).
        static let timeoutSeconds: TimeInterval = 15
    }

    func rewrite(
        text: String,
        tone: Tone,
        settings: SharedSettings,
        completion: @escaping (Result<String, Error>) -> Void
    ) {
        // Prefer the long-lived dr_ext_* token; fall back to the access
        // JWT for users on a build older than the one that mints it.
        let bearerToken = settings.bearerToken
        guard !bearerToken.isEmpty else {
            completion(.failure(NSError(
                domain: "BackendClient", code: -1,
                userInfo: [NSLocalizedDescriptionKey: "Please login in DraftRight app"])))
            return
        }

        let backendUrl = settings.backendUrl.hasSuffix("/")
            ? String(settings.backendUrl.dropLast())
            : settings.backendUrl

        guard let url = URL(string: "\(backendUrl)/rewrite") else {
            completion(.failure(NSError(
                domain: "BackendClient", code: -2,
                userInfo: [NSLocalizedDescriptionKey: "Invalid backend URL"])))
            return
        }

        // Truncate defensively. Log so a user's "missing" tail isn't a
        // silent mystery during debugging.
        if text.count > Config.maxInputChars {
            NSLog("[DraftRight] rewrite input truncated from \(text.count) to \(Config.maxInputChars) chars")
        }
        let inputText = String(text.prefix(Config.maxInputChars))
        let targetLanguage = tone == .translate ? settings.translateLanguage : nil
        let body = RewriteRequest(text: inputText, tone: tone.apiValue, target_language: targetLanguage)

        var request = URLRequest(url: url)
        request.httpMethod = "POST"
        request.addValue("application/json", forHTTPHeaderField: "Content-Type")
        request.addValue("Bearer \(bearerToken)", forHTTPHeaderField: "Authorization")
        request.timeoutInterval = Config.timeoutSeconds

        do {
            request.httpBody = try JSONEncoder().encode(body)
        } catch {
            completion(.failure(error))
            return
        }

        URLSession.shared.dataTask(with: request) { data, response, error in
            if let error = error {
                completion(.failure(error))
                return
            }
            guard let data = data else {
                completion(.failure(NSError(
                    domain: "BackendClient", code: -3,
                    userInfo: [NSLocalizedDescriptionKey: "No data returned"])))
                return
            }
            if let httpResponse = response as? HTTPURLResponse, httpResponse.statusCode >= 400 {
                let bodyText = String(data: data, encoding: .utf8) ?? "Unknown error"
                completion(.failure(NSError(
                    domain: "BackendClient", code: httpResponse.statusCode,
                    userInfo: [NSLocalizedDescriptionKey: "HTTP \(httpResponse.statusCode): \(bodyText)"])))
                return
            }
            do {
                let decoded = try JSONDecoder().decode(RewriteResponse.self, from: data)
                completion(.success(decoded.rewritten_text.trimmingCharacters(in: .whitespacesAndNewlines)))
            } catch {
                completion(.failure(error))
            }
        }.resume()
    }
}
