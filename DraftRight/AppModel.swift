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
        case .oneClick: return "Simple"
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
        didSet {
            defaults.set(launchAtLogin, forKey: Keys.launchAtLogin)
            // Sync the on-disk launchd agent. Toggle ON installs the
            // agent with RunAtLoad=true + KeepAlive (respawn on crash).
            // Toggle OFF uninstalls the agent entirely so the app
            // neither boots at login nor respawns mid-session.
            KeepAliveAgent.reconcile(desiredRunAtLoad: launchAtLogin)
        }
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
    /// True when backend explicitly rejected the refresh token (HTTP 401/403).
    /// Tells the UI to show "Session expired — please sign in again". Cleared
    /// on successful sign-in. Distinct from a transient network failure where
    /// the user just needs to wait, not re-authenticate.
    @Published var sessionExpired: Bool = false
    /// Guards the auto-popup so the "session expired" alert fires at most once
    /// per session-loss event. Reset when the user successfully signs in.
    private var didPromptForReauth: Bool = false
    /// Callback invoked when `sessionExpired` transitions false→true so the
    /// hosting scene can pop a modal. Set by `DraftRightApp` at launch.
    var onSessionExpired: (() -> Void)?
    /// Mirrors `updateService.availableUpdate` so SwiftUI views (menu bar,
    /// Settings) can show an "Update X available" affordance.
    @Published var availableUpdate: ResolvedUpdate?
    /// Mirrors `updateService.updateStaged` — true once the new DMG is
    /// pre-downloaded and "install" is instant.
    @Published var updateStaged: Bool = false
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
        static let lastSeenVersion = "draftright.lastSeenVersion"
    }

    init() {
        // Reconcile the launchd KeepAlive agent on every launch so the
        // installed plist tracks the current executable path (the app may
        // have moved since last launch) and matches the user's stored
        // preference. Idempotent — no-op when state is already correct.
        let storedLaunchAtLogin = UserDefaults.standard.bool(forKey: Keys.launchAtLogin)
        KeepAliveAgent.reconcile(desiredRunAtLoad: storedLaunchAtLogin)

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

        // Wire crash reporter — unhandled NSExceptions go to /errors for triage.
        ErrorReporter.install(
            backendUrl: backendUrl,
            bearerTokenProvider: { [weak self] in self?.accessToken }
        )

        // Start update check — 10 seconds after launch
        let version = Bundle.main.infoDictionary?["CFBundleShortVersionString"] as? String ?? "1.0.0"
        let svc = UpdateService(currentVersion: version, backendUrl: backendUrl)
        updateService = svc
        svc.$availableUpdate
            .receive(on: DispatchQueue.main)
            .sink { [weak self] in self?.availableUpdate = $0 }
            .store(in: &cancellables)
        svc.$updateStaged
            .receive(on: DispatchQueue.main)
            .sink { [weak self] in self?.updateStaged = $0 }
            .store(in: &cancellables)
        DispatchQueue.main.asyncAfter(deadline: .now() + 10) { [weak self] in
            Task { @MainActor in
                await self?.updateService?.checkIfNeeded()
            }
        }

        // Post-update "What's New": if the running version changed since the
        // last launch, show the release notes once. Record the version now so
        // the notice can't repeat; skip on a fresh install (no prior version).
        let lastSeen = defaults.string(forKey: Keys.lastSeenVersion) ?? ""
        if lastSeen != version {
            defaults.set(version, forKey: Keys.lastSeenVersion)
            if !lastSeen.isEmpty {
                DispatchQueue.main.asyncAfter(deadline: .now() + 11) { [weak self] in
                    Task { @MainActor in await self?.checkForWhatsNew(version: version) }
                }
            }
        }
    }

    /// Fetches and shows the release notes for the now-running version. The
    /// notes only appear if the backend's latest mac release still matches this
    /// version (so a stale/newer note is never shown).
    @MainActor
    private func checkForWhatsNew(version: String) async {
        guard let notes = await updateService?.releaseNotesForVersion(version),
              !notes.isEmpty else {
            DRLogger.log("Post-update: no release notes for \(version) — skipping What's New", category: .app)
            return
        }
        let alert = NSAlert()
        alert.messageText = "What's new in DraftRight v\(version)"
        alert.informativeText = notes
        alert.alertStyle = .informational
        alert.addButton(withTitle: "Got it")
        NSApp.activate(ignoringOtherApps: true)
        alert.runModal()
    }

    #if DEBUG
    private func autoLoginForDev() async {
        let base = backendUrl.strippingTrailingSlash
        DRLogger.log("autoLoginForDev: attempting login to \(base)/auth/login", category: .auth)
        guard let url = URL(string: "\(base)/auth/login") else {
            DRLogger.warn("autoLoginForDev: invalid URL", category: .auth)
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
                DRLogger.warn("autoLoginForDev: failed to parse tokens from response", category: .auth)
                return
            }
            storeTokens(access: access, refresh: refresh)
            DRLogger.log("autoLoginForDev: SUCCESS — isLoggedIn=\(isLoggedIn)", category: .auth)
        } catch {
            DRLogger.warn("autoLoginForDev: network error — \(error.localizedDescription)", category: .auth)
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
        sessionExpired = false
        didPromptForReauth = false
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
        // exchange it before declaring the user logged out. Classify the failure mode —
        // only wipe credentials when the BACKEND explicitly rejects them. A network blip
        // or 5xx must NOT destroy a valid 90-day token.
        if status == .notLoggedIn && !refreshToken.isEmpty {
            DRLogger.log("Access token rejected — attempting silent refresh", category: .auth)
            switch await healthClient.refreshTokens(refreshToken: refreshToken, backendUrl: backendUrl) {
            case .success(let access, let refresh):
                storeTokens(access: access, refresh: refresh)
                sessionExpired = false
                status = await healthClient.checkHealth(
                    backendUrl: backendUrl,
                    accessToken: access
                )
            case .unauthorized:
                DRLogger.error("Refresh rejected by backend — clearing tokens, surfacing sessionExpired", category: .auth)
                accessToken = ""
                refreshToken = ""
                let wasExpired = sessionExpired
                sessionExpired = true
                if !wasExpired && !didPromptForReauth {
                    didPromptForReauth = true
                    onSessionExpired?()
                }
            case .transient:
                DRLogger.warn("Refresh transient failure — keeping tokens, will retry next cycle", category: .auth)
                // Intentionally do NOT clear. Next 30s health check tries again.
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
