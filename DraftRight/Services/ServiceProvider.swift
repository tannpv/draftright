import AppKit
import UserNotifications

@MainActor
final class ServiceProvider: NSObject {
    let appModel: AppModel

    private let aiClient = BackendClient()
    private let diffWindow = DiffWindow.shared

    init(appModel: AppModel) {
        self.appModel = appModel
        super.init()
    }

    // MARK: - Service selectors (one per tone, matching Info.plist NSMessage values)

    @objc func rewriteSimple(_ pasteboard: NSPasteboard, userData: String?, error: AutoreleasingUnsafeMutablePointer<NSString>) {
        handleRewrite(pasteboard: pasteboard, tone: .simple, error: error)
    }

    @objc func rewriteNatural(_ pasteboard: NSPasteboard, userData: String?, error: AutoreleasingUnsafeMutablePointer<NSString>) {
        handleRewrite(pasteboard: pasteboard, tone: .natural, error: error)
    }

    @objc func rewritePolished(_ pasteboard: NSPasteboard, userData: String?, error: AutoreleasingUnsafeMutablePointer<NSString>) {
        handleRewrite(pasteboard: pasteboard, tone: .polished, error: error)
    }

    @objc func rewriteConcise(_ pasteboard: NSPasteboard, userData: String?, error: AutoreleasingUnsafeMutablePointer<NSString>) {
        handleRewrite(pasteboard: pasteboard, tone: .concise, error: error)
    }

    @objc func rewriteTechnical(_ pasteboard: NSPasteboard, userData: String?, error: AutoreleasingUnsafeMutablePointer<NSString>) {
        handleRewrite(pasteboard: pasteboard, tone: .technical, error: error)
    }

    // MARK: - Core rewrite logic

    private func handleRewrite(pasteboard: NSPasteboard, tone: Tone, error: AutoreleasingUnsafeMutablePointer<NSString>) {
        guard appModel.appMode == .advanced else {
            DRLogger.log("NSService ignored: app is in One-Click mode", category: .app)
            return
        }
        guard let text = pasteboard.string(forType: .string), !text.isEmpty else {
            error.pointee = "No text provided." as NSString
            return
        }

        guard appModel.isLoggedIn, !appModel.accessToken.isEmpty else {
            showNotification("Not signed in. Open DraftRight settings to sign in.")
            return
        }

        appModel.isRewriting = true

        // Open the panel pre-set to this tone
        diffWindow.presentPanel(
            original: text,
            visibleTones: appModel.visibleTones,
            onToneSelected: { [weak self] newTone in
                guard let self = self else { return }
                self.diffWindow.model.startLoading(tone: newTone)
                Task {
                    do {
                        let result = try await self.aiClient.rewrite(
                            text: text, tone: newTone,
                            accessToken: self.appModel.accessToken,
                            backendUrl: self.appModel.backendUrl,
                            targetLanguage: self.appModel.translateLanguage
                        )
                        self.diffWindow.model.handleRewriteResponse(result, tone: newTone)
                    } catch {
                        self.diffWindow.model.setError(error.localizedDescription)
                    }
                    self.appModel.isRewriting = false
                }
            },
            onReplace: { rewritten in
                if !AXTextService().replaceSelectedText(with: rewritten) {
                    ClipboardHelper.copy(text: rewritten)
                    DispatchQueue.main.asyncAfter(deadline: .now() + 0.1) {
                        ClipboardHelper.pasteFromClipboard()
                    }
                }
            },
            onCopy: { rewritten in
                ClipboardHelper.copy(text: rewritten)
            }
        )

        // Auto-select the tone from the service
        diffWindow.model.startLoading(tone: tone)
        Task {
            do {
                let rewritten = try await aiClient.rewrite(
                    text: text, tone: tone,
                    accessToken: appModel.accessToken,
                    backendUrl: appModel.backendUrl,
                    targetLanguage: appModel.translateLanguage
                )
                diffWindow.model.handleRewriteResponse(rewritten, tone: tone)
            } catch {
                diffWindow.model.setError(error.localizedDescription)
                showNotification("Rewrite failed: \(error.localizedDescription)")
            }
            appModel.isRewriting = false
        }
    }

    private func showNotification(_ message: String) {
        let center = UNUserNotificationCenter.current()
        center.requestAuthorization(options: [.alert, .sound]) { _, _ in }
        let content = UNMutableNotificationContent()
        content.title = "DraftRight"
        content.body = message
        let request = UNNotificationRequest(identifier: UUID().uuidString, content: content, trigger: nil)
        center.add(request, withCompletionHandler: nil)
    }
}
