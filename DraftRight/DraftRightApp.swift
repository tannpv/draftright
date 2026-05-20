import SwiftUI
import AppKit
import UserNotifications

@main
struct DraftRightApp: App {
    @StateObject private var appModel: AppModel

    init() {
        let model = AppModel()
        _appModel = StateObject(wrappedValue: model)

        // Auto-pop the sign-in alert the moment the backend rejects our
        // refresh token. Without this the user only learns the session
        // expired on the next hotkey press or by opening Settings, which
        // can leave the app silently dead for hours.
        model.onSessionExpired = { [weak model] in
            guard let model else { return }
            DispatchQueue.main.async {
                Self.showSignInRequiredAlert(appModel: model)
            }
        }

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
            Button("Report a Bug…") {
                BugReportPresenter.present(appModel: appModel)
            }
            Button("Suggest a Feature…") {
                FeedbackPresenter.present(appModel: appModel)
            }
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

    /// Surface a visible "you need to sign in" prompt when the hotkey fires
    /// while the user is logged out. Silently ignoring the hotkey leaves the
    /// user thinking the app is broken; this gives them a clear next action.
    /// Differentiates "session expired" (token rejected by backend) from
    /// "never signed in" so the message matches the actual state.
    @MainActor
    static func showSignInRequiredAlert(appModel: AppModel) {
        let alert = NSAlert()
        if appModel.sessionExpired {
            alert.messageText = "DraftRight session expired"
            alert.informativeText = "Your sign-in session expired. Please sign in again to keep using rewrite."
        } else {
            alert.messageText = "Sign in to use DraftRight"
            alert.informativeText = "DraftRight needs you to sign in before it can rewrite text."
        }
        alert.alertStyle = .warning
        alert.addButton(withTitle: "Open Settings")
        alert.addButton(withTitle: "Cancel")
        NSApp.activate(ignoringOtherApps: true)
        let response = alert.runModal()
        if response == .alertFirstButtonReturn {
            openSettingsWindow(appModel: appModel)
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
                DRLogger.warn("BLOCKED: not logged in — surfacing sign-in alert", category: .app)
                Self.showSignInRequiredAlert(appModel: appModel)
                return
            }

            switch appModel.appMode {
            case .oneClick:
                monitor.startLoadingAnimation()
                Self.runOneClickRewrite(
                    text: text,
                    appModel: appModel,
                    aiClient: aiClient,
                    monitor: monitor
                )
            case .advanced:
                monitor.hideTrigger()
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
                            targetLanguage: appModel.translateLanguage,
                            refreshToken: appModel.refreshToken,
                            onTokensRefreshed: { newAccess, newRefresh in
                                Task { @MainActor in
                                    appModel.storeTokens(access: newAccess, refresh: newRefresh)
                                }
                            }
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
        aiClient: BackendClient,
        monitor: SelectionMonitor
    ) {
        let tone = appModel.oneClickTone
        DRLogger.log("One-Click mode: instant rewrite with tone=\(tone.rawValue) text='\(text.prefix(30))'",
                     category: .app)
        appModel.isRewriting = true

        Task { @MainActor in
            defer {
                appModel.isRewriting = false
                monitor.stopLoadingAndHide()
            }
            do {
                let rewritten = try await aiClient.rewrite(
                    text: text,
                    tone: tone,
                    accessToken: appModel.accessToken,
                    backendUrl: appModel.backendUrl,
                    targetLanguage: appModel.translateLanguage
                )
                DRLogger.log("One-Click rewrite OK, replacing selection via clipboard paste", category: .app)
                // AX setSelectedText is unreliable across apps — many browsers
                // and Electron apps return AX success but don't actually mutate
                // the field. So we always go through the clipboard. Snapshot
                // and restore so we don't blow away whatever the user copied.
                let savedClipboard = ClipboardHelper.snapshot()
                ClipboardHelper.copy(text: rewritten)
                // 0.3s delay so the user's hotkey modifiers (Cmd+Shift)
                // have time to release before we synthesize Cmd+V — otherwise
                // the OS sees Cmd+Shift+V (paste-and-match) or nothing.
                DispatchQueue.main.asyncAfter(deadline: .now() + 0.3) {
                    ClipboardHelper.pasteFromClipboard()
                    DRLogger.log("Clipboard paste fired", category: .app)
                    // Give the paste a moment to land in the target app
                    // before we restore the user's original clipboard.
                    DispatchQueue.main.asyncAfter(deadline: .now() + 0.5) {
                        ClipboardHelper.restore(savedClipboard)
                        DRLogger.log("Clipboard restored", category: .app)
                    }
                }
            } catch {
                DRLogger.error("One-Click rewrite FAILED: \(error.localizedDescription)", category: .app)
                Self.showOneClickError(error.localizedDescription)
            }
        }
    }

    @MainActor
    static func showOneClickError(_ message: String) {
        let center = UNUserNotificationCenter.current()
        center.requestAuthorization(options: [.alert]) { _, _ in }
        let content = UNMutableNotificationContent()
        content.title = "DraftRight — Simple Mode Rewrite Failed"
        content.body = message
        let request = UNNotificationRequest(identifier: UUID().uuidString, content: content, trigger: nil)
        center.add(request, withCompletionHandler: nil)
    }
}
