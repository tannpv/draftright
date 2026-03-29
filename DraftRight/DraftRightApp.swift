import SwiftUI
import AppKit

@main
struct DraftRightApp: App {
    @StateObject private var appModel: AppModel

    init() {
        let model = AppModel()
        _appModel = StateObject(wrappedValue: model)

        DispatchQueue.main.async {
            Self.startServices(appModel: model)
        }
    }

    var body: some Scene {
        MenuBarExtra("DraftRight V2", systemImage: "pencil.and.outline") {
            Button(appModel.isRewriting ? "Rewriting..." : "Ready") {}
                .disabled(true)
            Divider()
            Button("Settings...") {
                Self.openSettingsWindow(appModel: appModel)
            }
            .keyboardShortcut(",", modifiers: .command)
            Divider()
            Button("Quit DraftRight V2") {
                NSApp.terminate(nil)
            }
            .keyboardShortcut("q", modifiers: .command)
        }
    }

    @MainActor
    static func openSettingsWindow(appModel: AppModel) {
        // Reuse existing window if open
        for window in NSApp.windows where window.title == "DraftRight V2 Settings" {
            window.makeKeyAndOrderFront(nil)
            NSApp.activate(ignoringOtherApps: true)
            return
        }

        let settingsView = SettingsView()
            .environmentObject(appModel)
            .frame(minWidth: 450, minHeight: 350)

        let hostingController = NSHostingController(rootView: settingsView)
        let window = NSWindow(contentViewController: hostingController)
        window.title = "DraftRight V2 Settings"
        window.styleMask = [.titled, .closable, .resizable]
        window.setContentSize(NSSize(width: 500, height: 400))
        window.center()
        window.makeKeyAndOrderFront(nil)
        NSApp.activate(ignoringOtherApps: true)
    }

    @MainActor
    static func startServices(appModel: AppModel) {
        NSApplication.shared.setActivationPolicy(.accessory)

        let serviceProvider = ServiceProvider(appModel: appModel)
        NSApp.servicesProvider = serviceProvider
        NSUpdateDynamicServices()

        let monitor = SelectionMonitor()
        let aiClient = BackendClient()
        let diffWindow = DiffWindow.shared

        // When user clicks pencil icon, open the rewrite panel with the selected text
        monitor.start { text in
            guard appModel.isLoggedIn, !appModel.accessToken.isEmpty else { return }

            diffWindow.presentPanel(
                original: text,
                onToneSelected: { tone in
                    // User picked a tone inside the panel → call API
                    diffWindow.model.startLoading(tone: tone)
                    appModel.isRewriting = true

                    Task {
                        do {
                            let rewritten = try await aiClient.rewrite(
                                text: text,
                                tone: tone,
                                accessToken: appModel.accessToken,
                                backendUrl: appModel.backendUrl,
                                targetLanguage: appModel.translateLanguage
                            )
                            diffWindow.model.setResult(rewritten)
                        } catch {
                            diffWindow.model.setError(error.localizedDescription)
                        }
                        appModel.isRewriting = false
                    }
                },
                onReplace: { _ in
                    // Handled by DiffWindow (copy + refocus + paste)
                },
                onCopy: { rewritten in
                    ClipboardHelper.copy(text: rewritten)
                }
            )
        }

        objc_setAssociatedObject(appModel, "serviceProvider", serviceProvider, .OBJC_ASSOCIATION_RETAIN)
        objc_setAssociatedObject(appModel, "selectionMonitor", monitor, .OBJC_ASSOCIATION_RETAIN)
    }
}
