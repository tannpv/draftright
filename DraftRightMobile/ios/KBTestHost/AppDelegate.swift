import UIKit

@main
final class AppDelegate: UIResponder, UIApplicationDelegate {
    var window: UIWindow?

    func application(_ application: UIApplication,
                     didFinishLaunchingWithOptions launchOptions: [UIApplication.LaunchOptionsKey: Any]?) -> Bool {
        seedAppGroupLanguages()
        let window = UIWindow(frame: UIScreen.main.bounds)
        window.rootViewController = HostViewController()
        window.makeKeyAndVisible()
        self.window = window
        return true
    }

    /// Seed the App Group with the same keys the Flutter app would write,
    /// so the DraftRight keyboard extension exposes EN/VI/FR under test
    /// without the full app installed + logged in. The active language is
    /// driven by a launch argument (`-drLang vi`) so each UI test can pin
    /// DraftRight to a specific composer deterministically, instead of
    /// relying on the in-keyboard globe cycle (which is unit-tested
    /// separately).
    private func seedAppGroupLanguages() {
        guard let defaults = UserDefaults(suiteName: "group.com.draftright.v2") else { return }
        defaults.set("[\"en\",\"vi\",\"fr\"]", forKey: "draftright.enabledLanguageIds")
        let lang = UserDefaults.standard.string(forKey: "drLang") ?? "en"
        defaults.set(lang, forKey: "draftright.activeLanguageId")

        // Optional rewrite-flow seeds. When the UI test points the keyboard
        // at a local stub backend (`-drBackend`) + a dummy token
        // (`-drToken`), the tone toolbar can exercise the rewrite + diff +
        // replace path offline. Absent these, the keyboard keeps whatever
        // the real app wrote.
        if let backend = UserDefaults.standard.string(forKey: "drBackend") {
            defaults.set(backend, forKey: "draftright.backendUrl")
        }
        if let token = UserDefaults.standard.string(forKey: "drToken") {
            defaults.set(token, forKey: "draftright.accessToken")
        }
    }
}
