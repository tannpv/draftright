import Foundation
import SwiftUI

@MainActor
final class AppModel: ObservableObject {
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
    @Published var isRewriting: Bool = false

    private let defaults = UserDefaults.standard

    private enum Keys {
        static let endpoint = "draftright.endpoint"
        static let model = "draftright.model"
        static let temperature = "draftright.temperature"
        static let launchAtLogin = "draftright.launchAtLogin"
    }

    init() {
        self.apiKey = KeychainHelper.load() ?? ""
        self.endpoint = defaults.string(forKey: Keys.endpoint) ?? "https://api.openai.com/v1/chat/completions"
        self.model = defaults.string(forKey: Keys.model) ?? "gpt-4o-mini"
        self.temperature = defaults.object(forKey: Keys.temperature) as? Double ?? 0.3
        self.launchAtLogin = defaults.bool(forKey: Keys.launchAtLogin)
    }
}
