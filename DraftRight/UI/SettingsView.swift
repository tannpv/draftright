import SwiftUI

struct SettingsView: View {
    @EnvironmentObject var appModel: AppModel
    @State private var loginEmail: String = ""
    @State private var loginPassword: String = ""
    @State private var isLoggingIn: Bool = false
    @State private var loginError: String? = nil
    @State private var showPassword: Bool = false
    @State private var isRecordingHotkey: Bool = false
    @State private var hotkeyMonitor: Any? = nil
    @State private var loggingEnabled: Bool = DRLogger.isEnabled

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

            Section(header: Text("Mode")) {
                Picker("Interaction Mode", selection: $appModel.appMode) {
                    ForEach(AppMode.allCases) { mode in
                        Text(mode.displayName).tag(mode)
                    }
                }
                .pickerStyle(.segmented)

                if appModel.appMode == .oneClick {
                    Picker("One-Click Tone", selection: $appModel.oneClickTone) {
                        ForEach(Tone.allCases) { tone in
                            Text(tone.displayName).tag(tone)
                        }
                    }
                    Text("A pencil appears when you select text. Click it to rewrite with the selected tone — no preview, no confirmation.")
                        .font(.caption)
                        .foregroundColor(.secondary)
                } else {
                    Text("Select text, then click the pencil (or use your hotkey) to open the rewrite panel with all tones.")
                        .font(.caption)
                        .foregroundColor(.secondary)
                }
            }

            Section(header: Text("Backend Server")) {
                TextField("Backend URL", text: $appModel.backendUrl)
                    .help("Leave default unless self-hosting")
            }

            Section(header: Text("Panel Tones")) {
                ForEach(Tone.allCases) { tone in
                    Toggle(tone.displayName, isOn: toneBinding(tone))
                }

                Picker("Auto-run on open", selection: Binding(
                    get: { appModel.defaultTone },
                    set: { appModel.defaultTone = $0 }
                )) {
                    ForEach(appModel.visibleTones) { tone in
                        Text(tone.displayName).tag(Optional(tone))
                    }
                    Text("None").tag(Tone?.none)
                }
                .help("Automatically run this tone when the panel opens")
            }

            Section(header: Text("Translation")) {
                Picker("Target Language", selection: $appModel.translateLanguage) {
                    ForEach(Self.languages, id: \.self) { lang in
                        Text(lang).tag(lang)
                    }
                }
                .help("Language used by the Translate tone option")
            }

            Section(header: Text("Trigger")) {
                HStack {
                    Text("Rewrite Trigger")
                    Spacer()
                    if appModel.hotkeyEnabled {
                        Text(SelectionMonitor.hotkeyDisplayName(appModel.hotkeyString))
                            .foregroundColor(.accentColor)
                    } else {
                        Text("Pencil Button")
                            .foregroundColor(.secondary)
                    }
                }
                HStack {
                    if appModel.hotkeyEnabled {
                        Button("Change Hotkey") { startRecordingHotkey() }
                        Button("Use Pencil Instead") {
                            appModel.hotkeyString = ""
                        }
                        .foregroundColor(.red)
                    } else {
                        Button("Set Hotkey") { startRecordingHotkey() }
                    }
                }
                if isRecordingHotkey {
                    HStack {
                        Text("Press key combo…")
                            .foregroundColor(.orange)
                        Spacer()
                        Button("Cancel") { stopRecordingHotkey() }
                    }
                }
                Text(appModel.hotkeyEnabled
                    ? "Select text, then press the hotkey to open the rewrite panel."
                    : "Highlight or double-click text to show the pencil button.")
                    .font(.caption)
                    .foregroundColor(.secondary)
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

            Section(header: Text("Updates")) {
                HStack {
                    Text("Version")
                    Spacer()
                    Text(Bundle.main.infoDictionary?["CFBundleShortVersionString"] as? String ?? "1.0.0")
                        .foregroundColor(.secondary)
                }
                Button("Check for Updates") {
                    Task {
                        await appModel.updateService?.checkNow()
                    }
                }
            }

            Section(header: Text("Logs")) {
                Toggle("Enable Logging", isOn: $loggingEnabled)
                    .onChange(of: loggingEnabled) { DRLogger.isEnabled = $0 }
                if loggingEnabled {
                    HStack {
                        Text(DRLogger.logFilePath)
                            .font(.caption)
                            .foregroundColor(.secondary)
                            .lineLimit(1)
                            .truncationMode(.middle)
                        Spacer()
                        Button("Open") {
                            NSWorkspace.shared.selectFile(DRLogger.logFilePath, inFileViewerRootedAtPath: "")
                        }
                        .font(.caption)
                        Button("Clear") {
                            DRLogger.clear()
                        }
                        .font(.caption)
                        .foregroundColor(.red)
                    }
                }
            }
        }
        .padding(12)
        .frame(width: 480)
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

    private func toneBinding(_ tone: Tone) -> Binding<Bool> {
        Binding(
            get: { appModel.enabledTones.contains(tone) },
            set: { enabled in
                if enabled {
                    appModel.enabledTones.insert(tone)
                } else if appModel.enabledTones.count > 1 {
                    appModel.enabledTones.remove(tone)
                    if appModel.defaultTone == tone {
                        appModel.defaultTone = appModel.enabledTones.first
                    }
                }
            }
        )
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

    private func startRecordingHotkey() {
        isRecordingHotkey = true
        hotkeyMonitor = NSEvent.addLocalMonitorForEvents(matching: .keyDown) { [self] event in
            let mask: NSEvent.ModifierFlags = [.command, .shift, .option, .control]
            let mods = event.modifierFlags.intersection(mask)
            // Require at least one modifier
            guard !mods.isEmpty else { return event }

            var parts: [String] = []
            if mods.contains(.command) { parts.append("cmd") }
            if mods.contains(.shift) { parts.append("shift") }
            if mods.contains(.option) { parts.append("opt") }
            if mods.contains(.control) { parts.append("ctrl") }
            let newHotkey = "\(parts.joined(separator: "+")):\(event.keyCode)"

            appModel.hotkeyString = newHotkey
            DRLogger.log("Hotkey changed to: \(SelectionMonitor.hotkeyDisplayName(newHotkey))", category: .settings)
            stopRecordingHotkey()
            return nil // consume the event
        }
    }

    private func stopRecordingHotkey() {
        isRecordingHotkey = false
        if let monitor = hotkeyMonitor {
            NSEvent.removeMonitor(monitor)
            hotkeyMonitor = nil
        }
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
