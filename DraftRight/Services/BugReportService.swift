import Foundation
import AppKit

/// Submits bug reports to `POST /bug-reports`.
///
/// The endpoint accepts anonymous reports (no auth) and authenticated reports
/// (Bearer JWT). The macOS app passes the Keychain-stored access token when
/// available so reports get associated with the signed-in user.
///
/// Auto-fills `app_version` from `CFBundleShortVersionString`+`CFBundleVersion`
/// and `os_info` from `ProcessInfo.processInfo.operatingSystemVersionString`.
enum BugReportService {
    static let source = "macos"

    enum BugReportError: LocalizedError {
        case invalidURL
        case descriptionTooShort
        case screenshotTooLarge
        case httpError(Int, String)
        case emptyResponse
        case network(Error)

        var errorDescription: String? {
            switch self {
            case .invalidURL: return "Invalid backend URL."
            case .descriptionTooShort: return "Please describe the bug (at least 10 characters)."
            case .screenshotTooLarge: return "Screenshot is larger than 5 MB."
            case .httpError(let code, let body): return "HTTP \(code): \(body)"
            case .emptyResponse: return "No response from server."
            case .network(let err): return err.localizedDescription
            }
        }
    }

    /// Submits a bug report.
    /// - Parameters:
    ///   - description: Free-text user description (min 10 chars after trim).
    ///   - screenshot: Optional PNG/JPEG bytes + filename (e.g. `"shot.png"`).
    ///   - userEmail: Optional override email for anonymous flow.
    ///   - authToken: Optional Bearer JWT — pass non-empty access token when signed in.
    ///   - backendUrl: Base URL e.g. `https://api.draftright.info`.
    /// - Returns: Server-assigned report UUID.
    static func submitReport(
        description: String,
        screenshot: (data: Data, filename: String)? = nil,
        userEmail: String? = nil,
        authToken: String? = nil,
        backendUrl: String
    ) async throws -> UUID {
        let trimmed = description.trimmingCharacters(in: .whitespacesAndNewlines)
        guard trimmed.count >= 10 else {
            throw BugReportError.descriptionTooShort
        }
        if let shot = screenshot, shot.data.count > 5 * 1024 * 1024 {
            throw BugReportError.screenshotTooLarge
        }

        let base = backendUrl.strippingTrailingSlash
        guard let url = URL(string: "\(base)/bug-reports") else {
            throw BugReportError.invalidURL
        }

        // Build multipart body manually — Foundation has no built-in helper.
        let boundary = "----DraftRightBugReport-\(UUID().uuidString)"
        var request = URLRequest(url: url)
        request.httpMethod = "POST"
        request.timeoutInterval = 30
        request.setValue("multipart/form-data; boundary=\(boundary)",
                         forHTTPHeaderField: "Content-Type")
        if let token = authToken, !token.isEmpty {
            request.setValue("Bearer \(token)", forHTTPHeaderField: "Authorization")
        }

        let appVersion: String = {
            let info = Bundle.main.infoDictionary
            let short = info?["CFBundleShortVersionString"] as? String ?? "?"
            let build = info?["CFBundleVersion"] as? String ?? "?"
            return "\(short)+\(build)"
        }()
        let osInfo = ProcessInfo.processInfo.operatingSystemVersionString

        let context: [String: Any] = [
            "platform": "macos",
            "locale": Locale.current.identifier,
            "ts": ISO8601DateFormatter().string(from: Date())
        ]
        let contextJSON = (try? JSONSerialization.data(withJSONObject: context))
            .flatMap { String(data: $0, encoding: .utf8) } ?? "{}"

        var body = Data()
        body.appendFormField(name: "description", value: trimmed, boundary: boundary)
        body.appendFormField(name: "source", value: source, boundary: boundary)
        body.appendFormField(name: "app_version", value: appVersion, boundary: boundary)
        body.appendFormField(name: "os_info", value: osInfo, boundary: boundary)
        body.appendFormField(name: "context", value: contextJSON, boundary: boundary)
        if let email = userEmail?.trimmingCharacters(in: .whitespacesAndNewlines),
           !email.isEmpty {
            body.appendFormField(name: "user_email", value: email, boundary: boundary)
        }
        if let shot = screenshot {
            let mime = mimeType(forFilename: shot.filename)
            body.appendFileField(
                name: "screenshot",
                filename: shot.filename,
                mimeType: mime,
                fileData: shot.data,
                boundary: boundary
            )
        }
        body.append("--\(boundary)--\r\n".data(using: .utf8)!)
        request.httpBody = body

        let data: Data
        let response: URLResponse
        do {
            (data, response) = try await URLSession.shared.data(for: request)
        } catch {
            throw BugReportError.network(error)
        }

        guard let http = response as? HTTPURLResponse else {
            throw BugReportError.emptyResponse
        }
        if http.statusCode >= 400 {
            let bodyText = String(data: data, encoding: .utf8) ?? ""
            throw BugReportError.httpError(http.statusCode, bodyText)
        }

        guard let json = try? JSONSerialization.jsonObject(with: data) as? [String: Any],
              let idString = json["id"] as? String,
              let uuid = UUID(uuidString: idString) else {
            throw BugReportError.emptyResponse
        }
        return uuid
    }

    private static func mimeType(forFilename filename: String) -> String {
        let lower = filename.lowercased()
        if lower.hasSuffix(".png") { return "image/png" }
        if lower.hasSuffix(".jpg") || lower.hasSuffix(".jpeg") { return "image/jpeg" }
        return "application/octet-stream"
    }
}

private extension Data {
    mutating func appendFormField(name: String, value: String, boundary: String) {
        append("--\(boundary)\r\n".data(using: .utf8)!)
        append("Content-Disposition: form-data; name=\"\(name)\"\r\n\r\n"
            .data(using: .utf8)!)
        append("\(value)\r\n".data(using: .utf8)!)
    }

    mutating func appendFileField(
        name: String,
        filename: String,
        mimeType: String,
        fileData: Data,
        boundary: String
    ) {
        append("--\(boundary)\r\n".data(using: .utf8)!)
        append("Content-Disposition: form-data; name=\"\(name)\"; filename=\"\(filename)\"\r\n"
            .data(using: .utf8)!)
        append("Content-Type: \(mimeType)\r\n\r\n".data(using: .utf8)!)
        append(fileData)
        append("\r\n".data(using: .utf8)!)
    }
}
