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

            Section(header: Text("Translation")) {
                Picker("Target Language", selection: $appModel.translateLanguage) {
                    ForEach(Self.languages, id: \.self) { lang in
                        Text(lang).tag(lang)
                    }
                }
                .help("Language used by the Translate tone option")
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

    private static let languages = [
        "Arabic", "Chinese (Simplified)", "Chinese (Traditional)",
        "Czech", "Danish", "Dutch", "English", "Finnish", "French",
        "German", "Greek", "Hebrew", "Hindi", "Hungarian",
        "Indonesian", "Italian", "Japanese", "Korean", "Malay",
        "Norwegian", "Polish", "Portuguese", "Romanian", "Russian",
        "Spanish", "Swedish", "Thai", "Turkish", "Ukrainian", "Vietnamese"
    ]
}
