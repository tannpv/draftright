import SwiftUI
import AppKit

struct TriggerSettingsTab: View {
    @EnvironmentObject var appModel: AppModel
    @State private var isRecordingHotkey: Bool = false
    @State private var hotkeyMonitor: Any? = nil

    var body: some View {
        Form {
            Section(header: Text("Trigger")) {
                Picker("Rewrite Trigger", selection: $appModel.triggerMode) {
                    ForEach(TriggerMode.allCases) { mode in
                        Text(mode.displayName).tag(mode)
                    }
                }
                .pickerStyle(.segmented)

                // The hotkey config only matters when the mode uses the hotkey.
                if appModel.triggerMode.usesHotkey {
                    HStack {
                        Text("Keyboard Shortcut")
                        Spacer()
                        if appModel.hotkeyString.isEmpty {
                            Text("Not set")
                                .foregroundColor(.secondary)
                        } else {
                            Text(SelectionMonitor.hotkeyDisplayName(appModel.hotkeyString))
                                .foregroundColor(.accentColor)
                        }
                    }
                    HStack {
                        Button(appModel.hotkeyString.isEmpty ? "Set Hotkey" : "Change Hotkey") {
                            startRecordingHotkey()
                        }
                        if isRecordingHotkey {
                            Text("Press key combo…")
                                .foregroundColor(.orange)
                            Spacer()
                            Button("Cancel") { stopRecordingHotkey() }
                        }
                    }
                }

                Text(triggerHint)
                    .font(.caption)
                    .foregroundColor(.secondary)
            }
        }
        .formStyle(.grouped)
    }

    private var triggerHint: String {
        switch appModel.triggerMode {
        case .pencil:
            return "Highlight or double-click text to show the pencil button."
        case .hotkey:
            return appModel.hotkeyString.isEmpty
                ? "Set a shortcut, then select text and press it to open the rewrite panel."
                : "Select text, then press the shortcut to open the rewrite panel."
        case .both:
            return appModel.hotkeyString.isEmpty
                ? "Highlight text to show the pencil. Set a shortcut to also trigger by keyboard."
                : "Highlight text to show the pencil — the shortcut works too, whichever is faster."
        }
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
}
