import SwiftUI
import AppKit

struct TriggerSettingsTab: View {
    @EnvironmentObject var appModel: AppModel
    @State private var isRecordingHotkey: Bool = false
    @State private var hotkeyMonitor: Any? = nil

    var body: some View {
        Form {
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
        .formStyle(.grouped)
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
