import SwiftUI

struct AccountSettingsTab: View {
    @EnvironmentObject var appModel: AppModel
    @State private var loginEmail: String = ""
    @State private var loginPassword: String = ""
    @State private var isLoggingIn: Bool = false
    @State private var loginError: String? = nil
    @State private var showPassword: Bool = false

    var body: some View {
        VStack(spacing: 0) {
            Form {
            Section(header: Text("Account")) {
                if appModel.isLoggedIn {
                    HStack {
                        Image(systemName: "checkmark.seal.fill")
                            .foregroundColor(.green)
                        Text("Signed in")
                            .foregroundColor(.secondary)
                        Spacer()
                        Button("Sign Out", role: .destructive) {
                            appModel.logout()
                        }
                    }
                } else {
                    VStack(alignment: .leading, spacing: 8) {
                        TextField("Email", text: $loginEmail)
                        HStack {
                            if showPassword {
                                TextField("Password", text: $loginPassword)
                            } else {
                                SecureField("Password", text: $loginPassword)
                            }
                            Button(action: { showPassword.toggle() }) {
                                Image(systemName: showPassword ? "eye.slash" : "eye")
                                    .foregroundColor(.secondary)
                            }
                            .buttonStyle(.plain)
                        }
                        if let error = loginError {
                            Text(error)
                                .font(.caption)
                                .foregroundColor(.red)
                        }
                        HStack {
                            Spacer()
                            Button("Sign In") {
                                Task { await signIn() }
                            }
                            .disabled(loginEmail.isEmpty || loginPassword.isEmpty || isLoggingIn)
                            if isLoggingIn {
                                ProgressView()
                                    .scaleEffect(0.7)
                            }
                        }

                        Divider()

                        // Google Sign-In
                        Button(action: { Task { await signInWithGoogle() } }) {
                            HStack {
                                Image(systemName: "globe")
                                    .foregroundColor(.blue)
                                Text("Sign in with Google")
                                Spacer()
                                if isLoggingIn {
                                    ProgressView().scaleEffect(0.6)
                                }
                            }
                        }
                        .disabled(isLoggingIn)
                    }
                }
            }
        }
            Spacer(minLength: 0)
        }
        .padding(12)
    }

    private func signIn() async {
        isLoggingIn = true
        loginError = nil
        DRLogger.log("Sign-in attempt: email=\(loginEmail) backend=\(appModel.backendUrl)", category: .settings)
        do {
            let (access, refresh) = try await AuthNetworking.login(
                email: loginEmail,
                password: loginPassword,
                backendUrl: appModel.backendUrl
            )
            appModel.storeTokens(access: access, refresh: refresh)
            DRLogger.log("Sign-in SUCCESS", category: .settings)
            loginEmail = ""
            loginPassword = ""
        } catch {
            DRLogger.log("Sign-in FAILED: \(error.localizedDescription)", category: .settings)
            loginError = error.localizedDescription
        }
        isLoggingIn = false
    }

    private func signInWithGoogle() async {
        isLoggingIn = true
        loginError = nil
        DRLogger.log("Google Sign-In: starting OAuth flow", category: .settings)

        do {
            let idToken = try await GoogleOAuth.authenticate()
            DRLogger.log("Google Sign-In: got id_token, calling backend", category: .settings)

            let (access, refresh) = try await AuthNetworking.socialLogin(
                provider: "google",
                idToken: idToken,
                backendUrl: appModel.backendUrl
            )
            appModel.storeTokens(access: access, refresh: refresh)
            DRLogger.log("Google Sign-In: SUCCESS", category: .settings)
        } catch {
            DRLogger.log("Google Sign-In FAILED: \(error.localizedDescription)", category: .settings)
            loginError = error.localizedDescription
        }
        isLoggingIn = false
    }
}

// Lightweight auth networking for the macOS settings panel
enum AuthNetworking {
    private static func executeAuthRequest(_ request: URLRequest) async throws -> (String, String) {
        let (data, response) = try await URLSession.shared.data(for: request)
        if let httpResponse = response as? HTTPURLResponse, httpResponse.statusCode >= 400 {
            let bodyText = String(data: data, encoding: .utf8) ?? "Unknown error"
            throw NSError(domain: "AuthNetworking", code: httpResponse.statusCode,
                          userInfo: [NSLocalizedDescriptionKey: "HTTP \(httpResponse.statusCode): \(bodyText)"])
        }
        guard let json = try JSONSerialization.jsonObject(with: data) as? [String: Any],
              let access = json["access_token"] as? String,
              let refresh = json["refresh_token"] as? String else {
            throw NSError(domain: "AuthNetworking", code: -2,
                          userInfo: [NSLocalizedDescriptionKey: "Invalid server response"])
        }
        return (access, refresh)
    }

    static func login(email: String, password: String, backendUrl: String) async throws -> (String, String) {
        let base = backendUrl.strippingTrailingSlash
        guard let url = URL(string: "\(base)/auth/login") else {
            throw NSError(domain: "AuthNetworking", code: -1,
                          userInfo: [NSLocalizedDescriptionKey: "Invalid backend URL"])
        }

        var request = URLRequest(url: url)
        request.httpMethod = "POST"
        request.addValue("application/json", forHTTPHeaderField: "Content-Type")
        request.httpBody = try JSONSerialization.data(withJSONObject: [
            "email": email.trimmingCharacters(in: .whitespacesAndNewlines).lowercased(),
            "password": password
        ])

        return try await executeAuthRequest(request)
    }

    static func socialLogin(provider: String, idToken: String, backendUrl: String) async throws -> (String, String) {
        let base = backendUrl.strippingTrailingSlash
        guard let url = URL(string: "\(base)/auth/social") else {
            throw NSError(domain: "AuthNetworking", code: -1,
                          userInfo: [NSLocalizedDescriptionKey: "Invalid backend URL"])
        }

        var request = URLRequest(url: url)
        request.httpMethod = "POST"
        request.addValue("application/json", forHTTPHeaderField: "Content-Type")
        request.httpBody = try JSONSerialization.data(withJSONObject: [
            "provider": provider,
            "id_token": idToken
        ])

        return try await executeAuthRequest(request)
    }
}
