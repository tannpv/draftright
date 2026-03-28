import Foundation
import SwiftUI

enum AIProvider: String, CaseIterable, Identifiable {
    case openai = "OpenAI"
    case custom = "Custom Server"

    var id: String { rawValue }
}

@MainActor
final class AppModel: ObservableObject {
    @Published var aiProvider: AIProvider {
        didSet { defaults.set(aiProvider.rawValue, forKey: Keys.aiProvider) }
    }
    @Published var apiKey: String {
        didSet { KeychainHelper.save(apiKey) }
    }
    @Published var endpoint: String {
        didSet { defaults.set(endpoint, forKey: Keys.endpoint) }
    }
    @Published var model: String {
        didSet { defaults.set(model, forKey: Keys.model) }
    }
    @Published var temperature: Double {
        didSet { defaults.set(temperature, forKey: Keys.temperature) }
    }
    @Published var launchAtLogin: Bool {
        didSet { defaults.set(launchAtLogin, forKey: Keys.launchAtLogin) }
    }
    @Published var translateLanguage: String {
        didSet { defaults.set(translateLanguage, forKey: Keys.translateLanguage) }
    }
    @Published var isRewriting: Bool = false

    private let defaults = UserDefaults.standard

    private enum Keys {
        static let aiProvider = "draftright.aiProvider"
        static let endpoint = "draftright.endpoint"
        static let model = "draftright.model"
        static let temperature = "draftright.temperature"
        static let launchAtLogin = "draftright.launchAtLogin"
        static let translateLanguage = "draftright.translateLanguage"
    }

    init() {
        let providerRaw = UserDefaults.standard.string(forKey: Keys.aiProvider) ?? AIProvider.openai.rawValue
        self.aiProvider = AIProvider(rawValue: providerRaw) ?? .openai
        self.apiKey = KeychainHelper.load() ?? ""
        self.endpoint = UserDefaults.standard.string(forKey: Keys.endpoint) ?? "https://api.openai.com/v1/chat/completions"
        self.model = UserDefaults.standard.string(forKey: Keys.model) ?? "gpt-4o-mini"
        self.temperature = UserDefaults.standard.object(forKey: Keys.temperature) as? Double ?? 0.3
        self.launchAtLogin = UserDefaults.standard.bool(forKey: Keys.launchAtLogin)
        self.translateLanguage = UserDefaults.standard.string(forKey: Keys.translateLanguage) ?? "Vietnamese"
    }
}
