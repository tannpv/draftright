import SwiftUI
import AppKit
import UserNotifications

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

        let hostingController = NSHostingController(rootView: settingsView)
        let window = NSWindow(contentViewController: hostingController)
        window.title = "DraftRight V2 Settings"
        window.styleMask = [.titled, .closable]
        window.setContentSize(NSSize(width: 560, height: 560))
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

        // When user clicks pencil icon or presses hotkey, fork on appMode
        monitor.start { text in
            DRLogger.log("onTextSelected fired, isLoggedIn=\(appModel.isLoggedIn) mode=\(appModel.appMode.rawValue)", category: .app)
            guard appModel.isLoggedIn, !appModel.accessToken.isEmpty else {
                DRLogger.log("BLOCKED: not logged in — ignoring selection", category: .app)
                return
            }

            switch appModel.appMode {
            case .oneClick:
                Self.runOneClickRewrite(
                    text: text,
                    appModel: appModel,
                    aiClient: aiClient
                )
            case .advanced:
                Self.runAdvancedRewrite(
                    text: text,
                    appModel: appModel,
                    aiClient: aiClient,
                    diffWindow: diffWindow
                )
            }
        }

        objc_setAssociatedObject(appModel, "serviceProvider", serviceProvider, .OBJC_ASSOCIATION_RETAIN)
        objc_setAssociatedObject(appModel, "selectionMonitor", monitor, .OBJC_ASSOCIATION_RETAIN)
    }

    @MainActor
    static func runAdvancedRewrite(
        text: String,
        appModel: AppModel,
        aiClient: BackendClient,
        diffWindow: DiffWindow
    ) {
        DRLogger.log("Advanced mode: opening panel with text: '\(text.prefix(30))'", category: .app)
        diffWindow.presentPanel(
            original: text,
            visibleTones: appModel.visibleTones,
            autoRunTone: appModel.autoRunTone,
            onToneSelected: { tone in
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
                        diffWindow.model.handleRewriteResponse(rewritten, tone: tone)
                    } catch {
                        diffWindow.model.setError(error.localizedDescription)
                    }
                    appModel.isRewriting = false
                }
            },
            onReplace: { _ in },
            onCopy: { rewritten in
                ClipboardHelper.copy(text: rewritten)
            }
        )
    }

    @MainActor
    static func runOneClickRewrite(
        text: String,
        appModel: AppModel,
        aiClient: BackendClient
    ) {
        let tone = appModel.oneClickTone
        DRLogger.log("One-Click mode: instant rewrite with tone=\(tone.rawValue) text='\(text.prefix(30))'",
                     category: .app)
        appModel.isRewriting = true

        Task { @MainActor in
            defer { appModel.isRewriting = false }
            do {
                let rewritten = try await aiClient.rewrite(
                    text: text,
                    tone: tone,
                    accessToken: appModel.accessToken,
                    backendUrl: appModel.backendUrl,
                    targetLanguage: appModel.translateLanguage
                )
                DRLogger.log("One-Click rewrite OK, replacing selection", category: .app)
                if !AXTextService().replaceSelectedText(with: rewritten) {
                    ClipboardHelper.copy(text: rewritten)
                    DispatchQueue.main.asyncAfter(deadline: .now() + 0.1) {
                        ClipboardHelper.pasteFromClipboard()
                    }
                }
            } catch {
                DRLogger.log("One-Click rewrite FAILED: \(error.localizedDescription)", category: .app)
                Self.showOneClickError(error.localizedDescription)
            }
        }
    }

    @MainActor
    static func showOneClickError(_ message: String) {
        let center = UNUserNotificationCenter.current()
        center.requestAuthorization(options: [.alert]) { _, _ in }
        let content = UNMutableNotificationContent()
        content.title = "DraftRight — One-Click Rewrite Failed"
        content.body = message
        let request = UNNotificationRequest(identifier: UUID().uuidString, content: content, trigger: nil)
        center.add(request, withCompletionHandler: nil)
    }
}
