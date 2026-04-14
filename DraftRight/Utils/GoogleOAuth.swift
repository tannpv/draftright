import AuthenticationServices
import AppKit

/// Handles Google OAuth flow using ASWebAuthenticationSession.
/// Opens a secure browser sheet, gets the auth code, exchanges it for an id_token.
enum GoogleOAuth {
    private static let clientId = "22951518033-gf853ftmf4emivffk0su2bik42j7cmai.apps.googleusercontent.com"
    private static let redirectURI = "com.draftright.app.v2:/oauth2callback"
    private static let tokenEndpoint = "https://oauth2.googleapis.com/token"

    /// Runs the full OAuth flow: browser → auth code → exchange → id_token
    @MainActor
    static func authenticate() async throws -> String {
        let authCode = try await getAuthCode()
        let idToken = try await exchangeCodeForToken(authCode)
        return idToken
    }

    // MARK: - Step 1: Get auth code via browser

    @MainActor
    private static func getAuthCode() async throws -> String {
        let scopes = "openid email profile"
        let authURL = "https://accounts.google.com/o/oauth2/v2/auth"
            + "?client_id=\(clientId)"
            + "&redirect_uri=\(redirectURI)"
            + "&response_type=code"
            + "&scope=\(scopes.addingPercentEncoding(withAllowedCharacters: .urlQueryAllowed) ?? scopes)"
            + "&access_type=offline"
            + "&prompt=consent"

        guard let url = URL(string: authURL) else {
            throw OAuthError.invalidURL
        }

        return try await withCheckedThrowingContinuation { continuation in
            let session = ASWebAuthenticationSession(
                url: url,
                callbackURLScheme: "com.draftright.app.v2"
            ) { callbackURL, error in
                if let error {
                    continuation.resume(throwing: error)
                    return
                }
                guard let callbackURL,
                      let components = URLComponents(url: callbackURL, resolvingAgainstBaseURL: false),
                      let code = components.queryItems?.first(where: { $0.name == "code" })?.value else {
                    continuation.resume(throwing: OAuthError.noAuthCode)
                    return
                }
                continuation.resume(returning: code)
            }

            // Use a window-based presentation context
            let presenter = OAuthPresenter()
            session.presentationContextProvider = presenter
            objc_setAssociatedObject(session, "presenter", presenter, .OBJC_ASSOCIATION_RETAIN)

            session.prefersEphemeralWebBrowserSession = false
            session.start()
        }
    }

    // MARK: - Step 2: Exchange auth code for id_token

    private static func exchangeCodeForToken(_ code: String) async throws -> String {
        guard let url = URL(string: tokenEndpoint) else {
            throw OAuthError.invalidURL
        }

        var request = URLRequest(url: url)
        request.httpMethod = "POST"
        request.addValue("application/x-www-form-urlencoded", forHTTPHeaderField: "Content-Type")

        let body = [
            "code=\(code)",
            "client_id=\(clientId)",
            "redirect_uri=\(redirectURI)",
            "grant_type=authorization_code",
        ].joined(separator: "&")
        request.httpBody = body.data(using: .utf8)

        let (data, response) = try await URLSession.shared.data(for: request)

        if let httpResponse = response as? HTTPURLResponse, httpResponse.statusCode >= 400 {
            let bodyText = String(data: data, encoding: .utf8) ?? "Unknown error"
            throw OAuthError.tokenExchangeFailed(bodyText)
        }

        guard let json = try JSONSerialization.jsonObject(with: data) as? [String: Any],
              let idToken = json["id_token"] as? String else {
            throw OAuthError.noIdToken
        }

        return idToken
    }

    // MARK: - Errors

    enum OAuthError: LocalizedError {
        case invalidURL
        case noAuthCode
        case noIdToken
        case tokenExchangeFailed(String)

        var errorDescription: String? {
            switch self {
            case .invalidURL: return "Invalid OAuth URL"
            case .noAuthCode: return "No authorization code received"
            case .noIdToken: return "No ID token in response"
            case .tokenExchangeFailed(let detail): return "Token exchange failed: \(detail)"
            }
        }
    }
}

// MARK: - Presentation context for ASWebAuthenticationSession

private class OAuthPresenter: NSObject, ASWebAuthenticationPresentationContextProviding {
    func presentationAnchor(for session: ASWebAuthenticationSession) -> ASPresentationAnchor {
        // Use any existing window, or create one
        return NSApp.windows.first(where: { $0.isVisible }) ?? NSApp.windows.first ?? ASPresentationAnchor()
    }
}
