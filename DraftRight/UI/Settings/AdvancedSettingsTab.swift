import SwiftUI
import AppKit

struct AdvancedSettingsTab: View {
    @EnvironmentObject var appModel: AppModel
    @State private var loggingEnabled: Bool = DRLogger.isEnabled

    var body: some View {
        Form {
            Section(header: Text("Services")) {
                Text("After launching DraftRight, the rewrite options appear in the right-click → Services menu of any app.")
                    .font(.caption)
                    .foregroundColor(.secondary)
                    .fixedSize(horizontal: false, vertical: true)

                Button("Refresh Services") {
                    NSUpdateDynamicServices()
                }
                .help("Force macOS to re-scan available services")
            }

            Section(header: Text("Feedback")) {
                Text("Hit a bug? Send us a description (and a screenshot if you have one) and we'll take a look.")
                    .font(.caption)
                    .foregroundColor(.secondary)
                    .fixedSize(horizontal: false, vertical: true)

                Button("Report a Bug…") {
                    BugReportPresenter.present(appModel: appModel)
                }
                .help("Open the bug report form")

                Button("Suggest a Feature…") {
                    FeedbackPresenter.present(appModel: appModel)
                }
                .help("Open the feature request form")
            }

            Section(header: Text("Logs")) {
                Toggle("Enable Logging", isOn: $loggingEnabled)
                    .onChange(of: loggingEnabled) { DRLogger.isEnabled = $0 }
                if loggingEnabled {
                    HStack {
                        Text(DRLogger.logFilePath)
                            .font(.caption)
                            .foregroundColor(.secondary)
                            .lineLimit(1)
                            .truncationMode(.middle)
                        Spacer()
                        Button("Open") {
                            NSWorkspace.shared.selectFile(DRLogger.logFilePath, inFileViewerRootedAtPath: "")
                        }
                        .font(.caption)
                        Button("Clear") {
                            DRLogger.clear()
                        }
                        .font(.caption)
                        .foregroundColor(.red)
                    }
                }
            }
        }
        .formStyle(.grouped)
    }
}
