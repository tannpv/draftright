import Foundation
import ApplicationServices
import AppKit

final class AXTextService {
    private let systemWideElement = AXUIElementCreateSystemWide()

    func ensureAccessibilityPermission() -> Bool {
        let options = [kAXTrustedCheckOptionPrompt.takeUnretainedValue() as String: true] as CFDictionary
        return AXIsProcessTrustedWithOptions(options)
    }

    private var diagCount = 0

    func readSelectedText() -> String? {
        var focused: CFTypeRef?
        let errorFocused = AXUIElementCopyAttributeValue(systemWideElement, kAXFocusedUIElementAttribute as CFString, &focused)

        diagCount += 1
        if diagCount % 20 == 0 {
            let msg = "\(Date()): [AX] err=\(errorFocused.rawValue) focused=\(focused != nil)\n"
            if let h = FileHandle(forWritingAtPath: "/tmp/draftright.log") {
                h.seekToEndOfFile(); h.write(msg.data(using: .utf8)!); h.closeFile()
            }
        }

        guard errorFocused == .success, let element = focused else { return nil }

        var selected: CFTypeRef?
        let errorSelected = AXUIElementCopyAttributeValue(element as! AXUIElement, kAXSelectedTextAttribute as CFString, &selected)
        guard errorSelected == .success, let value = selected as? String else { return nil }
        let trimmed = value.trimmingCharacters(in: .whitespacesAndNewlines)
        return trimmed.isEmpty ? nil : trimmed
    }

    /// Returns the screen frame of the currently focused text field, or nil.
    func focusedElementFrame() -> NSRect? {
        var focused: CFTypeRef?
        let err = AXUIElementCopyAttributeValue(systemWideElement, kAXFocusedUIElementAttribute as CFString, &focused)
        guard err == .success, let element = focused else { return nil }

        var posValue: CFTypeRef?
        var sizeValue: CFTypeRef?
        let posErr = AXUIElementCopyAttributeValue(element as! AXUIElement, kAXPositionAttribute as CFString, &posValue)
        let sizeErr = AXUIElementCopyAttributeValue(element as! AXUIElement, kAXSizeAttribute as CFString, &sizeValue)

        guard posErr == .success, sizeErr == .success else { return nil }

        var point = CGPoint.zero
        var size = CGSize.zero
        guard AXValueGetValue(posValue as! AXValue, .cgPoint, &point),
              AXValueGetValue(sizeValue as! AXValue, .cgSize, &size) else { return nil }

        // AX coordinates are top-left origin; convert to macOS screen coords (bottom-left origin)
        let screenHeight = NSScreen.main?.frame.height ?? 900
        let flippedY = screenHeight - point.y - size.height

        return NSRect(x: point.x, y: flippedY, width: size.width, height: size.height)
    }

    func replaceSelectedText(with text: String) -> Bool {
        var focused: CFTypeRef?
        let errorFocused = AXUIElementCopyAttributeValue(systemWideElement, kAXFocusedUIElementAttribute as CFString, &focused)
        guard errorFocused == .success, let element = focused else { return false }

        let errorSet = AXUIElementSetAttributeValue(element as! AXUIElement, kAXSelectedTextAttribute as CFString, text as CFTypeRef)
        return errorSet == .success
    }
}
