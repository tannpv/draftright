import SwiftUI
import AppKit

@main
struct DraftRightApp: App {
    @NSApplicationDelegateAdaptor(AppDelegate.self) var appDelegate

    var body: some Scene {
        MenuBarExtra("DraftRight", systemImage: "pencil.and.outline") {
            MenuBarView()
                .environmentObject(appDelegate.appModel)
        }
        Settings {
            SettingsView()
                .environmentObject(appDelegate.appModel)
        }
    }
}

@MainActor
final class AppDelegate: NSObject, NSApplicationDelegate {
    let appModel = AppModel()
    private var serviceProvider: ServiceProvider?

    func applicationDidFinishLaunching(_ notification: Notification) {
        NSApplication.shared.setActivationPolicy(.accessory)

        // Register services provider
        serviceProvider = ServiceProvider(appModel: appModel)
        NSApp.servicesProvider = serviceProvider
        NSUpdateDynamicServices()
    }
}
