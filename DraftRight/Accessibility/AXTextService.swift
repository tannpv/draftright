import Foundation
import ApplicationServices
import AppKit

final class AXTextService {
    private let systemWideElement = AXUIElementCreateSystemWide()

    func ensureAccessibilityPermission() -> Bool {
        let options = [kAXTrustedCheckOptionPrompt.takeUnretainedValue() as String: true] as CFDictionary
        return AXIsProcessTrustedWithOptions(options)
    }

    func readSelectedText() -> String? {
        var focused: CFTypeRef?
        let errorFocused = AXUIElementCopyAttributeValue(systemWideElement, kAXFocusedUIElementAttribute as CFString, &focused)

        if errorFocused != .success {
            DRLogger.log("focusedElement err=\(errorFocused.rawValue)", category: .ax)
            return nil
        }

        guard let element = focused else { return nil }

        var selected: CFTypeRef?
        let errorSelected = AXUIElementCopyAttributeValue(element as! AXUIElement, kAXSelectedTextAttribute as CFString, &selected)
        if errorSelected != .success {
            DRLogger.log("selectedText err=\(errorSelected.rawValue)", category: .ax)
            return nil
        }
        guard let value = selected as? String else { return nil }
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

    /// Returns the screen rect of the selected text (beginning of selection), or nil.
    /// Uses kAXBoundsForRangeParameterizedAttribute to get the exact position.
    func selectedTextBounds() -> NSRect? {
        var focused: CFTypeRef?
        let err = AXUIElementCopyAttributeValue(systemWideElement, kAXFocusedUIElementAttribute as CFString, &focused)
        guard err == .success, let element = focused else { return nil }

        // Get the selected text range
        var rangeValue: CFTypeRef?
        let rangeErr = AXUIElementCopyAttributeValue(element as! AXUIElement, kAXSelectedTextRangeAttribute as CFString, &rangeValue)
        guard rangeErr == .success, let range = rangeValue else { return nil }

        // Get the bounds for that range
        var boundsValue: CFTypeRef?
        let boundsErr = AXUIElementCopyParameterizedAttributeValue(
            element as! AXUIElement,
            kAXBoundsForRangeParameterizedAttribute as CFString,
            range,
            &boundsValue
        )
        guard boundsErr == .success, let bounds = boundsValue else { return nil }

        var rect = CGRect.zero
        guard AXValueGetValue(bounds as! AXValue, .cgRect, &rect) else { return nil }

        // Convert from AX coords (top-left origin) to macOS screen coords (bottom-left origin)
        let screenHeight = NSScreen.main?.frame.height ?? 900
        let flippedY = screenHeight - rect.origin.y - rect.size.height

        return NSRect(x: rect.origin.x, y: flippedY, width: rect.size.width, height: rect.size.height)
    }

    /// Returns true if the focused element is an editable text field (not static text).
    func isFocusedElementEditable() -> Bool {
        var focused: CFTypeRef?
        let err = AXUIElementCopyAttributeValue(systemWideElement, kAXFocusedUIElementAttribute as CFString, &focused)
        guard err == .success, let element = focused else { return false }

        // Check the role
        var roleValue: CFTypeRef?
        AXUIElementCopyAttributeValue(element as! AXUIElement, kAXRoleAttribute as CFString, &roleValue)
        let role = roleValue as? String ?? ""

        let editableRoles = ["AXTextField", "AXTextArea", "AXComboBox", "AXSearchField"]
        if editableRoles.contains(role) { return true }

        // For web content (Electron apps), check if the element is editable
        // via the AXRole "AXWebArea" or subrole, or check AXEditable attribute
        var editableValue: CFTypeRef?
        let editErr = AXUIElementCopyAttributeValue(element as! AXUIElement, "AXEditable" as CFString, &editableValue)
        if editErr == .success, let editable = editableValue as? Bool, editable {
            return true
        }

        // Check subrole for "AXTextArea" in web content
        var subroleValue: CFTypeRef?
        AXUIElementCopyAttributeValue(element as! AXUIElement, kAXSubroleAttribute as CFString, &subroleValue)
        let subrole = subroleValue as? String ?? ""
        if subrole == "AXTextArea" { return true }

        return false
    }

    func replaceSelectedText(with text: String) -> Bool {
        var focused: CFTypeRef?
        let errorFocused = AXUIElementCopyAttributeValue(systemWideElement, kAXFocusedUIElementAttribute as CFString, &focused)
        guard errorFocused == .success, let element = focused else { return false }

        let errorSet = AXUIElementSetAttributeValue(element as! AXUIElement, kAXSelectedTextAttribute as CFString, text as CFTypeRef)
        return errorSet == .success
    }
}
