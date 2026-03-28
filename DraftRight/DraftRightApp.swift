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
        MenuBarExtra("DraftRight", systemImage: "pencil.and.outline") {
            MenuBarView()
                .environmentObject(appModel)
        }
        Settings {
            SettingsView()
                .environmentObject(appModel)
        }
    }

    @MainActor
    static func startServices(appModel: AppModel) {
        NSApplication.shared.setActivationPolicy(.accessory)

        let serviceProvider = ServiceProvider(appModel: appModel)
        NSApp.servicesProvider = serviceProvider
        NSUpdateDynamicServices()

        let monitor = SelectionMonitor()
        let aiClient = OpenAIClient()
        let diffWindow = DiffWindow.shared
        // When user clicks pencil icon, open the rewrite panel with the selected text
        monitor.start { text in
            guard !appModel.apiKey.isEmpty else { return }

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
                                apiKey: appModel.apiKey,
                                endpoint: appModel.endpoint,
                                model: appModel.model,
                                temperature: appModel.temperature,
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
