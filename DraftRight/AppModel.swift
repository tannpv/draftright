import Foundation
import SwiftUI
import Combine

/// Top-level interaction mode for the macOS app.
/// - `.advanced`: Pencil → diff panel with multiple tones (legacy behavior).
/// - `.oneClick`: Pencil → instant rewrite+replace with a preset tone, no panel.
enum AppMode: String, CaseIterable, Identifiable {
    case advanced
    case oneClick

    var id: String { rawValue }

    var displayName: String {
        switch self {
        case .advanced: return "Advanced"
        case .oneClick: return "One-Click"
        }
    }
}

@MainActor
final class AppModel: ObservableObject {
    @Published var accessToken: String = "" {
        didSet { KeychainHelper.save(accessToken, forKey: "accessToken") }
    }
    @Published var refreshToken: String = "" {
        didSet { KeychainHelper.save(refreshToken, forKey: "refreshToken") }
    }
    @Published var backendUrl: String {
        didSet { defaults.set(backendUrl, forKey: Keys.backendUrl) }
    }
    @Published var launchAtLogin: Bool {
        didSet { defaults.set(launchAtLogin, forKey: Keys.launchAtLogin) }
    }
    @Published var translateLanguage: String {
        didSet { defaults.set(translateLanguage, forKey: Keys.translateLanguage) }
    }
    /// Hotkey stored as "modifiers:keyCode" e.g. "cmd+shift:15" (15 = R key). Empty = hotkey disabled.
    @Published var hotkeyString: String {
        didSet { defaults.set(hotkeyString, forKey: Keys.hotkey) }
    }
    /// True when a hotkey is configured — pencil trigger is disabled, hotkey is active
    var hotkeyEnabled: Bool { !hotkeyString.isEmpty }
    /// Which tones are enabled in the panel
    @Published var enabledTones: Set<Tone> {
        didSet { defaults.set(enabledTones.map { $0.rawValue }, forKey: Keys.enabledTones) }
    }
    /// Default tone that auto-runs when panel opens
    @Published var defaultTone: Tone? {
        didSet { defaults.set(defaultTone?.rawValue ?? "", forKey: Keys.defaultTab) }
    }

    /// Top-level interaction mode. Defaults to `.advanced` on first launch so
    /// existing users see no behavior change after updating.
    @Published var appMode: AppMode {
        didSet { defaults.set(appMode.rawValue, forKey: Keys.appMode) }
    }

    /// Tone used by One-Click mode. Only consulted when `appMode == .oneClick`.
    @Published var oneClickTone: Tone {
        didSet { defaults.set(oneClickTone.rawValue, forKey: Keys.oneClickTone) }
    }

    /// Tones visible in the panel, in display order
    var visibleTones: [Tone] {
        Tone.allCases.filter { enabledTones.contains($0) }
    }

    /// The tone to auto-run when panel opens
    var autoRunTone: Tone? {
        guard let tone = defaultTone, enabledTones.contains(tone) else { return nil }
        return tone
    }

    @Published var isRewriting: Bool = false
    @Published var isLoggedIn: Bool = false
    @Published var backendStatus: BackendStatus = .offline
    var cancellables = Set<AnyCancellable>()

    private let defaults = UserDefaults.standard
    private let healthClient = BackendClient()
    private var healthTimer: Timer?
    var updateService: UpdateService?

    private enum Keys {
        static let backendUrl = "draftright.backendUrl"
        static let launchAtLogin = "draftright.launchAtLogin"
        static let translateLanguage = "draftright.translateLanguage"
        static let hotkey = "draftright.hotkey"
        static let enabledTones = "draftright.enabledTones"
        static let defaultTab = "draftright.defaultTab"
        static let appMode = "draftright.appMode"
        static let oneClickTone = "draftright.oneClickTone"
    }

    init() {
        self.backendUrl = UserDefaults.standard.string(forKey: Keys.backendUrl) ?? "http://localhost:3000"
        self.launchAtLogin = UserDefaults.standard.bool(forKey: Keys.launchAtLogin)
        self.translateLanguage = UserDefaults.standard.string(forKey: Keys.translateLanguage) ?? "Vietnamese"
        self.hotkeyString = UserDefaults.standard.string(forKey: Keys.hotkey) ?? ""
        let allTones = Set(Tone.allCases)
        let savedToneStrings = UserDefaults.standard.stringArray(forKey: Keys.enabledTones)
        if let strings = savedToneStrings {
            self.enabledTones = Set(strings.compactMap { Tone(rawValue: $0) })
        } else {
            self.enabledTones = allTones
        }
        let savedDefault = UserDefaults.standard.string(forKey: Keys.defaultTab) ?? ""
        self.defaultTone = Tone(rawValue: savedDefault)
        let savedMode = UserDefaults.standard.string(forKey: Keys.appMode) ?? AppMode.advanced.rawValue
        self.appMode = AppMode(rawValue: savedMode) ?? .advanced
        let savedOneClick = UserDefaults.standard.string(forKey: Keys.oneClickTone) ?? Tone.polished.rawValue
        self.oneClickTone = Tone(rawValue: savedOneClick) ?? .polished
        self.accessToken = KeychainHelper.load(forKey: "accessToken") ?? ""
        self.refreshToken = KeychainHelper.load(forKey: "refreshToken") ?? ""
        self.isLoggedIn = !self.accessToken.isEmpty

        // Auto-login with test account for dev if not logged in
        #if DEBUG
        if !isLoggedIn {
            Task { @MainActor in
                await autoLoginForDev()
            }
        }
        #endif

        startHealthCheck()

        // Start update check — 10 seconds after launch
        let version = Bundle.main.infoDictionary?["CFBundleShortVersionString"] as? String ?? "1.0.0"
        updateService = UpdateService(currentVersion: version, backendUrl: backendUrl)
        DispatchQueue.main.asyncAfter(deadline: .now() + 10) { [weak self] in
            Task { @MainActor in
                await self?.updateService?.checkIfNeeded()
            }
        }
    }

    #if DEBUG
    private func autoLoginForDev() async {
        let base = backendUrl.strippingTrailingSlash
        DRLogger.log("autoLoginForDev: attempting login to \(base)/auth/login", category: .auth)
        guard let url = URL(string: "\(base)/auth/login") else {
            DRLogger.log("autoLoginForDev: invalid URL", category: .auth)
            return
        }
        var request = URLRequest(url: url)
        request.httpMethod = "POST"
        request.addValue("application/json", forHTTPHeaderField: "Content-Type")
        request.httpBody = try? JSONSerialization.data(withJSONObject: [
            "email": "test@test.com", "password": "test1234"
        ])
        do {
            let (data, response) = try await URLSession.shared.data(for: request)
            let httpStatus = (response as? HTTPURLResponse)?.statusCode ?? -1
            DRLogger.log("autoLoginForDev: HTTP \(httpStatus), body=\(String(data: data, encoding: .utf8)?.prefix(100) ?? "nil")", category: .auth)
            guard let json = try? JSONSerialization.jsonObject(with: data) as? [String: Any],
                  let access = json["access_token"] as? String,
                  let refresh = json["refresh_token"] as? String else {
                DRLogger.log("autoLoginForDev: failed to parse tokens from response", category: .auth)
                return
            }
            storeTokens(access: access, refresh: refresh)
            DRLogger.log("autoLoginForDev: SUCCESS — isLoggedIn=\(isLoggedIn)", category: .auth)
        } catch {
            DRLogger.log("autoLoginForDev: network error — \(error.localizedDescription)", category: .auth)
        }
    }
    #endif

    func logout() {
        accessToken = ""
        refreshToken = ""
        KeychainHelper.delete(forKey: "accessToken")
        KeychainHelper.delete(forKey: "refreshToken")
        isLoggedIn = false
    }

    func storeTokens(access: String, refresh: String) {
        accessToken = access
        refreshToken = refresh
        isLoggedIn = true
    }

    nonisolated deinit {
        healthTimer?.invalidate()
    }

    func startHealthCheck() {
        // Check immediately on launch
        Task { @MainActor in
            await performHealthCheck()
        }
        // Then every 30 seconds
        healthTimer = Timer.scheduledTimer(withTimeInterval: 30, repeats: true) { [weak self] _ in
            guard let self else { return }
            Task { @MainActor in
                await self.performHealthCheck()
            }
        }
    }

    private var lastAutoRecoveryAttempt: Date = .distantPast

    private func performHealthCheck() async {
        guard !isRewriting else { return }
        var status = await healthClient.checkHealth(
            backendUrl: backendUrl,
            accessToken: accessToken.isEmpty ? nil : accessToken
        )

        // Silent refresh: if the access token aged out but we still hold a refresh token,
        // exchange it before declaring the user logged out. This is the fix for the
        // "had to open Settings and click Sign In again" papercut after JWT expiry.
        if status == .notLoggedIn && !refreshToken.isEmpty {
            DRLogger.log("Access token rejected — attempting silent refresh", category: .auth)
            if let pair = await healthClient.refreshTokens(refreshToken: refreshToken, backendUrl: backendUrl) {
                storeTokens(access: pair.access, refresh: pair.refresh)
                status = await healthClient.checkHealth(
                    backendUrl: backendUrl,
                    accessToken: pair.access
                )
            } else {
                // Refresh failed → stale tokens are useless, clear them so the next
                // health check doesn't keep looping through the refresh path.
                DRLogger.log("Silent refresh failed — clearing stale tokens", category: .auth)
                accessToken = ""
                refreshToken = ""
            }
        }

        if backendStatus != status {
            DRLogger.log("Health status: \(backendStatus) → \(status)", category: .api)
        }
        backendStatus = status
        // Sync login state with health check result
        if status == .connected && !isLoggedIn {
            isLoggedIn = true
        } else if status == .notLoggedIn && isLoggedIn {
            isLoggedIn = false
        }

        // Auto-recovery: if offline and targeting localhost, try to start the backend
        if status == .offline && backendUrl.contains("localhost") {
            attemptAutoRecovery()
        }
    }

    /// Run start-server.sh to bring up Docker services when backend is offline.
    /// Throttled to at most once every 2 minutes to avoid spamming.
    private func attemptAutoRecovery() {
        let now = Date()
        guard now.timeIntervalSince(lastAutoRecoveryAttempt) > 120 else { return }
        lastAutoRecoveryAttempt = now

        let scriptPath = Bundle.main.bundleURL
            .deletingLastPathComponent()  // .app/Contents/MacOS/
            .deletingLastPathComponent()  // .app/Contents/
            .deletingLastPathComponent()  // .app/
            .deletingLastPathComponent()  // project root (dev) or parent dir
            .appendingPathComponent("start-server.sh")
            .path

        guard FileManager.default.isExecutableFile(atPath: scriptPath) else {
            DRLogger.log("Auto-recovery: start-server.sh not found at \(scriptPath)", category: .app)
            return
        }

        DRLogger.log("Auto-recovery: running start-server.sh", category: .app)
        DispatchQueue.global(qos: .utility).async {
            let process = Process()
            process.executableURL = URL(fileURLWithPath: "/bin/bash")
            process.arguments = [scriptPath]
            process.environment = [
                "PATH": "/usr/local/bin:/usr/bin:/bin:/opt/homebrew/bin"
            ]
            try? process.run()
            process.waitUntilExit()
            DRLogger.log("Auto-recovery: start-server.sh exited with code \(process.terminationStatus)", category: .app)
        }
    }
}
