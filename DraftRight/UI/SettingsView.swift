import SwiftUI

struct SettingsView: View {
    @EnvironmentObject var appModel: AppModel
    @State private var tempApiKey: String = ""

    var body: some View {
        Form {
            Section(header: Text("OpenAI API")) {
                SecureField("API Key", text: Binding(
                    get: { tempApiKey },
                    set: {
                        tempApiKey = $0
                        appModel.apiKey = $0
                    }
                ))
                .help("Stored securely in macOS Keychain")

                TextField("Endpoint", text: $appModel.endpoint)

                TextField("Model", text: $appModel.model)

                HStack {
                    Text("Temperature")
                    Slider(value: $appModel.temperature, in: 0...1)
                    Text(String(format: "%.2f", appModel.temperature))
                        .font(.footnote)
                        .foregroundColor(.secondary)
                        .frame(width: 30)
                }
            }

            Section(header: Text("General")) {
                Toggle("Launch at Login", isOn: $appModel.launchAtLogin)
            }

            Section(header: Text("Services")) {
                Text("After launching DraftRight, the rewrite options appear in the right-click → Services menu of any app.")
                    .font(.caption)
                    .foregroundColor(.secondary)

                Button("Refresh Services") {
                    NSUpdateDynamicServices()
                }
                .help("Force macOS to re-scan available services")
            }
        }
        .padding(12)
        .frame(width: 480)
        .onAppear {
            tempApiKey = appModel.apiKey
        }
    }
}
