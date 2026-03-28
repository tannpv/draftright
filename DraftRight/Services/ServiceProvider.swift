import AppKit
import UserNotifications

@MainActor
final class ServiceProvider: NSObject {
    let appModel: AppModel

    private let aiClient = OpenAIClient()
    private let diffWindow = DiffWindow.shared

    init(appModel: AppModel) {
        self.appModel = appModel
        super.init()
    }

    // MARK: - Service selectors (one per tone, matching Info.plist NSMessage values)

    @objc func rewriteProfessional(_ pasteboard: NSPasteboard, userData: String?, error: AutoreleasingUnsafeMutablePointer<NSString>) {
        handleRewrite(pasteboard: pasteboard, tone: .professional, error: error)
    }

    @objc func rewriteCasual(_ pasteboard: NSPasteboard, userData: String?, error: AutoreleasingUnsafeMutablePointer<NSString>) {
        handleRewrite(pasteboard: pasteboard, tone: .casual, error: error)
    }

    @objc func rewriteGrammar(_ pasteboard: NSPasteboard, userData: String?, error: AutoreleasingUnsafeMutablePointer<NSString>) {
        handleRewrite(pasteboard: pasteboard, tone: .grammar, error: error)
    }

    @objc func rewriteShorter(_ pasteboard: NSPasteboard, userData: String?, error: AutoreleasingUnsafeMutablePointer<NSString>) {
        handleRewrite(pasteboard: pasteboard, tone: .shorter, error: error)
    }

    @objc func rewriteLonger(_ pasteboard: NSPasteboard, userData: String?, error: AutoreleasingUnsafeMutablePointer<NSString>) {
        handleRewrite(pasteboard: pasteboard, tone: .longer, error: error)
    }

    // MARK: - Core rewrite logic

    private func handleRewrite(pasteboard: NSPasteboard, tone: Tone, error: AutoreleasingUnsafeMutablePointer<NSString>) {
        guard let text = pasteboard.string(forType: .string), !text.isEmpty else {
            error.pointee = "No text provided." as NSString
            return
        }

        guard !appModel.apiKey.isEmpty else {
            showNotification("API key not set. Open DraftRight settings to configure.")
            return
        }

        appModel.isRewriting = true
        diffWindow.showLoading(tone: tone)

        Task {
            do {
                let rewritten = try await aiClient.rewrite(
                    text: text,
                    tone: tone,
                    apiKey: appModel.apiKey,
                    endpoint: appModel.endpoint,
                    model: appModel.model,
                    temperature: appModel.temperature
                )

                diffWindow.present(
                    tone: tone,
                    original: text,
                    rewritten: rewritten,
                    replaceHandler: {
                        ClipboardHelper.copy(text: rewritten)
                        DispatchQueue.main.asyncAfter(deadline: .now() + 0.1) {
                            ClipboardHelper.pasteFromClipboard()
                        }
                    },
                    copyHandler: {
                        ClipboardHelper.copy(text: rewritten)
                    }
                )
            } catch {
                diffWindow.close()
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
