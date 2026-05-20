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
    /// so the DraftRight keyboard extension cycles EN/VI/FR under test
    /// without needing the full app installed + logged in.
    private func seedAppGroupLanguages() {
        guard let defaults = UserDefaults(suiteName: "group.com.draftright.v2") else { return }
        defaults.set("[\"en\",\"vi\",\"fr\"]", forKey: "draftright.enabledLanguageIds")
        defaults.set("en", forKey: "draftright.activeLanguageId")
    }
}
