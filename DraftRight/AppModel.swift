import Foundation
import SwiftUI

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
    @Published var isRewriting: Bool = false
    @Published var isLoggedIn: Bool = false

    private let defaults = UserDefaults.standard

    private enum Keys {
        static let backendUrl = "draftright.backendUrl"
        static let launchAtLogin = "draftright.launchAtLogin"
        static let translateLanguage = "draftright.translateLanguage"
    }

    init() {
        self.backendUrl = UserDefaults.standard.string(forKey: Keys.backendUrl) ?? "https://api.draftright.app"
        self.launchAtLogin = UserDefaults.standard.bool(forKey: Keys.launchAtLogin)
        self.translateLanguage = UserDefaults.standard.string(forKey: Keys.translateLanguage) ?? "Vietnamese"
        self.accessToken = KeychainHelper.load(forKey: "accessToken") ?? ""
        self.refreshToken = KeychainHelper.load(forKey: "refreshToken") ?? ""
        self.isLoggedIn = !self.accessToken.isEmpty
    }

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
}
