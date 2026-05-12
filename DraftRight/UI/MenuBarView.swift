import SwiftUI

struct MenuBarView: View {
    @EnvironmentObject var appModel: AppModel

    var body: some View {
        VStack(alignment: .leading, spacing: 8) {
            HStack {
                Circle()
                    .fill(appModel.isRewriting ? Color.orange : Color.green)
                    .frame(width: 8, height: 8)
                Text(appModel.isRewriting ? "Rewriting..." : "Ready")
                    .font(.subheadline)
                    .foregroundColor(.secondary)
            }

            Divider()

            Text("Right-click selected text and look")
                .font(.caption)
                .foregroundColor(.secondary)
            Text("under Services for DraftRight options.")
                .font(.caption)
                .foregroundColor(.secondary)

            if let update = appModel.availableUpdate {
                Divider()
                Button(appModel.updateStaged
                       ? "⬆ Update \(update.version) ready — restart & install"
                       : "⬆ Update \(update.version) available — install now") {
                    appModel.updateService?.startInstall(update)
                }
                .foregroundColor(.green)
            }

            Divider()

            Button("Settings...") {
                NSApp.activate(ignoringOtherApps: true)
                if #available(macOS 14.0, *) {
                    NSApp.sendAction(Selector(("showSettingsWindow:")), to: nil, from: nil)
                } else {
                    NSApp.sendAction(Selector(("showPreferencesWindow:")), to: nil, from: nil)
                }
            }
            Button("Quit DraftRight") {
                NSApp.terminate(nil)
            }
        }
        .padding(12)
        .frame(width: 280)
    }
}
