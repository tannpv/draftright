import Foundation

struct SharedSettings {
    private let defaults: UserDefaults?

    init() {
        defaults = UserDefaults(suiteName: "group.com.draftright.v2")
    }

    /// Long-lived dr_ext_* token (preferred). Stored in App Group keychain
    /// via SharedKeychain. Survives access-JWT expiry, so the keyboard
    /// works after the main app has been idle for hours/days.
    var extensionToken: String {
        SharedKeychain.get("draftright.extensionToken") ?? ""
    }

    /// Short-lived user JWT (legacy fallback). Will be removed in a
    /// follow-up release once everyone has launched the new main app
    /// version at least once and minted an extension token.
    var accessToken: String {
        defaults?.string(forKey: "draftright.accessToken") ?? ""
    }

    /// The token to actually present in Authorization headers. Prefer
    /// the long-lived extension token; fall back to the access JWT for
    /// users who haven't upgraded the main app yet.
    var bearerToken: String {
        let ext = extensionToken
        return ext.isEmpty ? accessToken : ext
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

    /// IDs of enabled keyboard languages, in user-chosen order. Flutter's
    /// shared_preferences writes to the app's standard UserDefaults — not
    /// the App Group — so AuthService bridges the list via a method
    /// channel as a JSON-encoded string. Defaults to ["en"] if never set.
    var enabledLanguageIds: [String] {
        guard let raw = defaults?.string(forKey: "draftright.enabledLanguageIds"),
              let data = raw.data(using: .utf8),
              let parsed = try? JSONSerialization.jsonObject(with: data) as? [String],
              !parsed.isEmpty
        else {
            return ["en"]
        }
        return parsed
    }

    /// Currently active keyboard language id. Defaults to "en".
    var activeLanguageId: String {
        defaults?.string(forKey: "draftright.activeLanguageId") ?? "en"
    }
}
