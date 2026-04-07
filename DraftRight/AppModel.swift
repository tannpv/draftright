import Foundation
import SwiftUI
import Combine

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
    @Published var isRewriting: Bool = false
    @Published var isLoggedIn: Bool = false
    @Published var backendStatus: BackendStatus = .offline
    var cancellables = Set<AnyCancellable>()

    private let defaults = UserDefaults.standard
    private let healthClient = BackendClient()
    private var healthTimer: Timer?

    private enum Keys {
        static let backendUrl = "draftright.backendUrl"
        static let launchAtLogin = "draftright.launchAtLogin"
        static let translateLanguage = "draftright.translateLanguage"
        static let hotkey = "draftright.hotkey"
    }

    init() {
        self.backendUrl = UserDefaults.standard.string(forKey: Keys.backendUrl) ?? "http://localhost:3000"
        self.launchAtLogin = UserDefaults.standard.bool(forKey: Keys.launchAtLogin)
        self.translateLanguage = UserDefaults.standard.string(forKey: Keys.translateLanguage) ?? "Vietnamese"
        self.hotkeyString = UserDefaults.standard.string(forKey: Keys.hotkey) ?? ""
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
    }

    #if DEBUG
    private func autoLoginForDev() async {
        let base = backendUrl.hasSuffix("/") ? String(backendUrl.dropLast()) : backendUrl
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

    private func performHealthCheck() async {
        guard !isRewriting else { return }
        let status = await healthClient.checkHealth(
            backendUrl: backendUrl,
            accessToken: accessToken.isEmpty ? nil : accessToken
        )
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
    }
}
