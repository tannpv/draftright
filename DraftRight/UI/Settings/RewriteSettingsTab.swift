import SwiftUI
import AppKit

struct RewriteSettingsTab: View {
    @EnvironmentObject var appModel: AppModel
    @State private var isRecordingHotkey: Bool = false
    @State private var hotkeyMonitor: Any? = nil

    var body: some View {
        Form {
            Section(header: Text("Mode")) {
                Picker("Interaction Mode", selection: $appModel.appMode) {
                    ForEach(AppMode.allCases) { mode in
                        Text(mode.displayName).tag(mode)
                    }
                }
                .pickerStyle(.segmented)

                if appModel.appMode == .oneClick {
                    Picker("One-Click Tone", selection: $appModel.oneClickTone) {
                        ForEach(Tone.allCases) { tone in
                            Text(tone.displayName).tag(tone)
                        }
                    }
                    Text("A pencil appears when you select text. Click it to rewrite with the selected tone — no preview, no confirmation.")
                        .font(.caption)
                        .foregroundColor(.secondary)
                } else {
                    Text("Select text, then click the pencil (or use your hotkey) to open the rewrite panel with all tones.")
                        .font(.caption)
                        .foregroundColor(.secondary)
                }
            }

            Section(header: Text("Panel Tones")) {
                ForEach(Tone.allCases) { tone in
                    Toggle(tone.displayName, isOn: toneBinding(tone))
                }

                Picker("Auto-run on open", selection: Binding(
                    get: { appModel.defaultTone },
                    set: { appModel.defaultTone = $0 }
                )) {
                    ForEach(appModel.visibleTones) { tone in
                        Text(tone.displayName).tag(Optional(tone))
                    }
                    Text("None").tag(Tone?.none)
                }
                .help("Automatically run this tone when the panel opens")
            }

            Section(header: Text("Translation")) {
                Picker("Target Language", selection: $appModel.translateLanguage) {
                    ForEach(Self.languages, id: \.self) { lang in
                        Text(lang).tag(lang)
                    }
                }
                .help("Language used by the Translate tone option")
            }

            Section(header: Text("Trigger")) {
                HStack {
                    Text("Rewrite Trigger")
                    Spacer()
                    if appModel.hotkeyEnabled {
                        Text(SelectionMonitor.hotkeyDisplayName(appModel.hotkeyString))
                            .foregroundColor(.accentColor)
                    } else {
                        Text("Pencil Button")
                            .foregroundColor(.secondary)
                    }
                }
                HStack {
                    if appModel.hotkeyEnabled {
                        Button("Change Hotkey") { startRecordingHotkey() }
                        Button("Use Pencil Instead") {
                            appModel.hotkeyString = ""
                        }
                        .foregroundColor(.red)
                    } else {
                        Button("Set Hotkey") { startRecordingHotkey() }
                    }
                }
                if isRecordingHotkey {
                    HStack {
                        Text("Press key combo…")
                            .foregroundColor(.orange)
                        Spacer()
                        Button("Cancel") { stopRecordingHotkey() }
                    }
                }
                Text(appModel.hotkeyEnabled
                    ? "Select text, then press the hotkey to open the rewrite panel."
                    : "Highlight or double-click text to show the pencil button.")
                    .font(.caption)
                    .foregroundColor(.secondary)
            }
        }
        .padding(12)
    }

    private func toneBinding(_ tone: Tone) -> Binding<Bool> {
        Binding(
            get: { appModel.enabledTones.contains(tone) },
            set: { enabled in
                if enabled {
                    appModel.enabledTones.insert(tone)
                } else if appModel.enabledTones.count > 1 {
                    appModel.enabledTones.remove(tone)
                    if appModel.defaultTone == tone {
                        appModel.defaultTone = appModel.enabledTones.first
                    }
                }
            }
        )
    }

    private func startRecordingHotkey() {
        isRecordingHotkey = true
        hotkeyMonitor = NSEvent.addLocalMonitorForEvents(matching: .keyDown) { [self] event in
            let mask: NSEvent.ModifierFlags = [.command, .shift, .option, .control]
            let mods = event.modifierFlags.intersection(mask)
            // Require at least one modifier
            guard !mods.isEmpty else { return event }

            var parts: [String] = []
            if mods.contains(.command) { parts.append("cmd") }
            if mods.contains(.shift) { parts.append("shift") }
            if mods.contains(.option) { parts.append("opt") }
            if mods.contains(.control) { parts.append("ctrl") }
            let newHotkey = "\(parts.joined(separator: "+")):\(event.keyCode)"

            appModel.hotkeyString = newHotkey
            DRLogger.log("Hotkey changed to: \(SelectionMonitor.hotkeyDisplayName(newHotkey))", category: .settings)
            stopRecordingHotkey()
            return nil // consume the event
        }
    }

    private func stopRecordingHotkey() {
        isRecordingHotkey = false
        if let monitor = hotkeyMonitor {
            NSEvent.removeMonitor(monitor)
            hotkeyMonitor = nil
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
