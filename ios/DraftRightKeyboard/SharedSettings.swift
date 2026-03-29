import Foundation

struct SharedSettings {
    private let defaults: UserDefaults?

    init() {
        defaults = UserDefaults(suiteName: "group.com.draftright.app")
    }

    var accessToken: String {
        defaults?.string(forKey: "draftright.accessToken") ?? ""
    }

    var backendUrl: String {
        defaults?.string(forKey: "draftright.backendUrl") ?? "https://api.draftright.app"
    }

    var translateLanguage: String {
        defaults?.string(forKey: "draftright.translateLanguage") ?? "Vietnamese"
    }
}
