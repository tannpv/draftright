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

            Divider()

            if #available(macOS 14.0, *) {
                SettingsLink {
                    Text("Settings...")
                }
            } else {
                Button("Settings...") {
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
