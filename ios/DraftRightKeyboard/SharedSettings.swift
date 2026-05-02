import Foundation

struct SharedSettings {
    private let defaults: UserDefaults?

    init() {
        defaults = UserDefaults(suiteName: "group.com.draftright.v2")
    }

    var accessToken: String {
        defaults?.string(forKey: "draftright.accessToken") ?? ""
    }

    var backendUrl: String {
        #if DEBUG
        return defaults?.string(forKey: "draftright.backendUrl") ?? "http://localhost:3000"
        #else
        return defaults?.string(forKey: "draftright.backendUrl") ?? "https://api.draftright.info"
        #endif
    }

    var translateLanguage: String {
        defaults?.string(forKey: "draftright.translateLanguage") ?? "Vietnamese"
    }
}
