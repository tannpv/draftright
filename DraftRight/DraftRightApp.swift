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

    private var statusColor: Color {
        switch appModel.backendStatus {
        case .connected: return .green
        case .notLoggedIn: return .yellow
        case .offline: return .red
        case .wrongServer: return .purple
        }
    }

    private var statusLabel: String {
        switch appModel.backendStatus {
        case .connected: return "Connected"
        case .notLoggedIn: return "Not Logged In"
        case .offline: return "Offline"
        case .wrongServer: return "Wrong Server"
        }
    }

    var body: some Scene {
        MenuBarExtra {
            Button(appModel.isRewriting ? "Rewriting..." : statusLabel) {}
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
        } label: {
            Image(systemName: "pencil.and.outline")
                .symbolRenderingMode(.palette)
                .foregroundStyle(statusColor)
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
        monitor.hotkeyString = appModel.hotkeyString
        let aiClient = BackendClient()
        let diffWindow = DiffWindow.shared

        // Sync hotkey changes from settings to monitor
        appModel.$hotkeyString.sink { newValue in
            Task { @MainActor in
                monitor.hotkeyString = newValue
            }
        }.store(in: &appModel.cancellables)

        // When user clicks pencil icon or presses hotkey, open the rewrite panel
        monitor.start { text in
            DRLogger.log("onTextSelected fired, isLoggedIn=\(appModel.isLoggedIn) hasToken=\(!appModel.accessToken.isEmpty)", category: .app)
            guard appModel.isLoggedIn, !appModel.accessToken.isEmpty else {
                DRLogger.log("BLOCKED: not logged in — panel will not show", category: .app)
                return
            }
            DRLogger.log("Opening panel with text: '\(text.prefix(30))'", category: .app)

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
