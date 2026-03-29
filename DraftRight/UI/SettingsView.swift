import SwiftUI

struct SettingsView: View {
    @EnvironmentObject var appModel: AppModel
    @State private var loginEmail: String = ""
    @State private var loginPassword: String = ""
    @State private var isLoggingIn: Bool = false
    @State private var loginError: String? = nil

    var body: some View {
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
                        SecureField("Password", text: $loginPassword)
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
                    }
                }
            }

            Section(header: Text("Backend Server")) {
                TextField("Backend URL", text: $appModel.backendUrl)
                    .help("Leave default unless self-hosting")
            }

            Section(header: Text("Translation")) {
                Picker("Target Language", selection: $appModel.translateLanguage) {
                    ForEach(Self.languages, id: \.self) { lang in
                        Text(lang).tag(lang)
                    }
                }
                .help("Language used by the Translate tone option")
            }

            Section(header: Text("General")) {
                Toggle("Launch at Login", isOn: $appModel.launchAtLogin)
            }

            Section(header: Text("Services")) {
                Text("After launching DraftRight, the rewrite options appear in the right-click → Services menu of any app.")
                    .font(.caption)
                    .foregroundColor(.secondary)

                Button("Refresh Services") {
                    NSUpdateDynamicServices()
                }
                .help("Force macOS to re-scan available services")
            }
        }
        .padding(12)
        .frame(width: 480)
    }

    private func signIn() async {
        isLoggingIn = true
        loginError = nil
        do {
            let (access, refresh) = try await AuthNetworking.login(
                email: loginEmail,
                password: loginPassword,
                backendUrl: appModel.backendUrl
            )
            appModel.storeTokens(access: access, refresh: refresh)
            loginEmail = ""
            loginPassword = ""
        } catch {
            loginError = error.localizedDescription
        }
        isLoggingIn = false
    }

    private static let languages = [
        "Arabic", "Chinese (Simplified)", "Chinese (Traditional)",
        "Czech", "Danish", "Dutch", "English", "Finnish", "French",
        "German", "Greek", "Hebrew", "Hindi", "Hungarian",
        "Indonesian", "Italian", "Japanese", "Korean", "Malay",
        "Norwegian", "Polish", "Portuguese", "Romanian", "Russian",
        "Spanish", "Swedish", "Thai", "Turkish", "Ukrainian", "Vietnamese"
    ]
}

// Lightweight auth networking for the macOS settings panel
enum AuthNetworking {
    static func login(email: String, password: String, backendUrl: String) async throws -> (String, String) {
        let base = backendUrl.hasSuffix("/") ? String(backendUrl.dropLast()) : backendUrl
        guard let url = URL(string: "\(base)/auth/login") else {
            throw NSError(domain: "AuthNetworking", code: -1,
                          userInfo: [NSLocalizedDescriptionKey: "Invalid backend URL"])
        }

        var request = URLRequest(url: url)
        request.httpMethod = "POST"
        request.addValue("application/json", forHTTPHeaderField: "Content-Type")
        request.httpBody = try JSONSerialization.data(withJSONObject: [
            "email": email, "password": password
        ])

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
}
