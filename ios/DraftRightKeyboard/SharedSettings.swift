import Foundation

struct SharedSettings {
    private let defaults: UserDefaults?

    init() {
        defaults = UserDefaults(suiteName: "group.com.draftright.app")
    }

    var aiProvider: String {
        defaults?.string(forKey: "draftright.aiProvider") ?? "openai"
    }

    var apiKey: String {
        defaults?.string(forKey: "draftright.apiKey") ?? ""
    }

    var endpoint: String {
        defaults?.string(forKey: "draftright.endpoint") ?? "https://api.openai.com/v1/chat/completions"
    }

    var model: String {
        defaults?.string(forKey: "draftright.model") ?? "gpt-4o-mini"
    }

    var temperature: Double {
        defaults?.double(forKey: "draftright.temperature") ?? 0.3
    }

    var translateLanguage: String {
        defaults?.string(forKey: "draftright.translateLanguage") ?? "Vietnamese"
    }
}
