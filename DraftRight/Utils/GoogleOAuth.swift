import AuthenticationServices
import AppKit
import CryptoKit

/// Google OAuth flow for a *native* macOS app — iOS-type client + PKCE +
/// reversed-scheme redirect. Web-type clients are blocked by Google for
/// custom-scheme callbacks (returns "redirect_uri_mismatch" or "Error 400:
/// invalid_request"), so the client must be registered as iOS in the Google
/// Cloud Console. PKCE replaces the client_secret as the proof-of-possession,
/// so this code has no secret embedded.
///
/// Flow:
///   1. Generate PKCE verifier + S256 challenge.
///   2. Open ASWebAuthenticationSession with the reversed-scheme redirect.
///   3. Browser hits `com.googleusercontent.apps.<id>:/oauth2callback?code=…`.
///   4. Exchange the auth code + verifier for an id_token at /token.
enum GoogleOAuth {
    // iOS-type OAuth client (project 22951518033). Reversed scheme is the
    // bundle-id-shaped value Google issues alongside the client_id for native
    // apps; it's the only redirect scheme an iOS-type client accepts.
    private static let clientId = "22951518033-dvkn61dhibse9fu83ohh51mlovd7269a.apps.googleusercontent.com"
    private static let reversedClientScheme = "com.googleusercontent.apps.22951518033-dvkn61dhibse9fu83ohh51mlovd7269a"
    private static let redirectURI = "\(reversedClientScheme):/oauth2callback"
    private static let authEndpoint = "https://accounts.google.com/o/oauth2/v2/auth"
    private static let tokenEndpoint = "https://oauth2.googleapis.com/token"

    /// Runs the full OAuth flow: browser → auth code → exchange → id_token.
    @MainActor
    static func authenticate() async throws -> String {
        let verifier = pkceVerifier()
        let challenge = pkceChallenge(verifier)
        let authCode = try await getAuthCode(challenge: challenge)
        let idToken = try await exchangeCodeForToken(code: authCode, verifier: verifier)
        return idToken
    }

    // MARK: - Step 1: Get auth code via browser

    @MainActor
    private static func getAuthCode(challenge: String) async throws -> String {
        let scopes = "openid email profile"
        let escapedScopes = scopes.addingPercentEncoding(withAllowedCharacters: .urlQueryAllowed) ?? scopes
        let escapedRedirect = redirectURI.addingPercentEncoding(withAllowedCharacters: .urlQueryAllowed) ?? redirectURI
        let authURL = authEndpoint
            + "?client_id=\(clientId)"
            + "&redirect_uri=\(escapedRedirect)"
            + "&response_type=code"
            + "&scope=\(escapedScopes)"
            + "&code_challenge=\(challenge)"
            + "&code_challenge_method=S256"

        guard let url = URL(string: authURL) else {
            throw OAuthError.invalidURL
        }

        return try await withCheckedThrowingContinuation { continuation in
            let session = ASWebAuthenticationSession(
                url: url,
                callbackURLScheme: reversedClientScheme
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

            let presenter = OAuthPresenter()
            session.presentationContextProvider = presenter
            objc_setAssociatedObject(session, "presenter", presenter, .OBJC_ASSOCIATION_RETAIN)

            session.prefersEphemeralWebBrowserSession = false
            session.start()
        }
    }

    // MARK: - Step 2: Exchange auth code for id_token

    private static func exchangeCodeForToken(code: String, verifier: String) async throws -> String {
        guard let url = URL(string: tokenEndpoint) else {
            throw OAuthError.invalidURL
        }

        var request = URLRequest(url: url)
        request.httpMethod = "POST"
        request.addValue("application/x-www-form-urlencoded", forHTTPHeaderField: "Content-Type")

        // iOS-type clients are *public* — no client_secret. PKCE's code_verifier
        // is the proof-of-possession instead.
        let body = [
            "code=\(code)",
            "client_id=\(clientId)",
            "redirect_uri=\(redirectURI)",
            "grant_type=authorization_code",
            "code_verifier=\(verifier)",
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

    // MARK: - PKCE helpers

    private static func pkceVerifier() -> String {
        var bytes = [UInt8](repeating: 0, count: 32)
        _ = SecRandomCopyBytes(kSecRandomDefault, bytes.count, &bytes)
        return base64URLEncode(Data(bytes))
    }

    private static func pkceChallenge(_ verifier: String) -> String {
        let hash = SHA256.hash(data: Data(verifier.utf8))
        return base64URLEncode(Data(hash))
    }

    /// base64url without padding (the encoding OAuth/PKCE require).
    private static func base64URLEncode(_ data: Data) -> String {
        data.base64EncodedString()
            .replacingOccurrences(of: "+", with: "-")
            .replacingOccurrences(of: "/", with: "_")
            .replacingOccurrences(of: "=", with: "")
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
        return NSApp.windows.first(where: { $0.isVisible }) ?? NSApp.windows.first ?? ASPresentationAnchor()
    }
}
