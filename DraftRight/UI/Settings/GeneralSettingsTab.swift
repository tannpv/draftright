import SwiftUI

struct GeneralSettingsTab: View {
    @EnvironmentObject var appModel: AppModel

    var body: some View {
        Form {
            Section(header: Text("General")) {
                Toggle("Launch at Login", isOn: $appModel.launchAtLogin)
            }

            Section(header: Text("Backend Server")) {
                TextField("Backend URL", text: $appModel.backendUrl)
                    .help("Leave default unless self-hosting")
            }

            Section(header: Text("Updates")) {
                HStack {
                    Text("Version")
                    Spacer()
                    Text(Bundle.main.infoDictionary?["CFBundleShortVersionString"] as? String ?? "1.0.0")
                        .foregroundColor(.secondary)
                }
                Button("Check for Updates") {
                    Task {
                        await appModel.updateService?.checkNow()
                    }
                }
            }
        }
        .padding(12)
    }
}
