import AppKit
import Carbon.HIToolbox

enum ClipboardHelper {
    static func copy(text: String) {
        let pasteboard = NSPasteboard.general
        pasteboard.clearContents()
        pasteboard.setString(text, forType: .string)
    }

    /// Snapshot the current clipboard string so we can restore it
    /// after we hijack the pasteboard for a synthetic Cmd+V.
    static func snapshot() -> String? {
        return NSPasteboard.general.string(forType: .string)
    }

    /// Restore a previously-snapshotted clipboard contents.
    static func restore(_ saved: String?) {
        let pasteboard = NSPasteboard.general
        pasteboard.clearContents()
        if let s = saved { pasteboard.setString(s, forType: .string) }
    }

    static func selectAll() {
        let source = CGEventSource(stateID: .hidSystemState)
        let keyDown = CGEvent(keyboardEventSource: source, virtualKey: CGKeyCode(kVK_ANSI_A), keyDown: true)
        keyDown?.flags = .maskCommand
        let keyUp = CGEvent(keyboardEventSource: source, virtualKey: CGKeyCode(kVK_ANSI_A), keyDown: false)
        keyUp?.flags = .maskCommand
        keyDown?.post(tap: .cgSessionEventTap)
        keyUp?.post(tap: .cgSessionEventTap)
    }

    static func pasteFromClipboard() {
        let source = CGEventSource(stateID: .hidSystemState)
        let keyDown = CGEvent(keyboardEventSource: source, virtualKey: CGKeyCode(kVK_ANSI_V), keyDown: true)
        keyDown?.flags = .maskCommand
        let keyUp = CGEvent(keyboardEventSource: source, virtualKey: CGKeyCode(kVK_ANSI_V), keyDown: false)
        keyUp?.flags = .maskCommand
        keyDown?.post(tap: .cgSessionEventTap)
        keyUp?.post(tap: .cgSessionEventTap)
    }
}
