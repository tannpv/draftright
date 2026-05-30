import SwiftUI

struct GeneralSettingsTab: View {
    @EnvironmentObject var appModel: AppModel

    var body: some View {
        Form {
            Section(header: Text("General")) {
                Toggle("Launch at Login", isOn: $appModel.launchAtLogin)
            }

            Section(header: Text("Updates")) {
                HStack {
                    Text("Version")
                    Spacer()
                    Text(Bundle.main.infoDictionary?["CFBundleShortVersionString"] as? String ?? "1.0.0")
                        .foregroundColor(.secondary)
                }
                if let update = appModel.availableUpdate {
                    Button(appModel.updateStaged
                           ? "Update \(update.version) downloaded — click here to restart and install"
                           : "Update \(update.version) available — click here to download and install") {
                        appModel.updateService?.startInstall(update)
                    }
                    .foregroundColor(.accentColor)
                }
                Button("Check for Updates") {
                    Task {
                        await appModel.updateService?.checkNow()
                    }
                }
            }
        }
        .formStyle(.grouped)
        // Refresh in the background so a freshly-published release shows here
        // without waiting for the once-a-day check or pressing the button.
        .task {
            await appModel.updateService?.refreshAvailableUpdate()
        }
    }
}
