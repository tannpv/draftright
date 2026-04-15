import AppKit
import SwiftUI
import Carbon.HIToolbox

/// Detects text selection via mouse/keyboard events and shows a floating trigger button.
/// Clicking the trigger grabs the selected text (AX or Cmd+C fallback) and calls onTextSelected.
@MainActor
final class SelectionMonitor {
    private let axService = AXTextService()
    private var triggerWindow: NSPanel?
    private var onTextSelected: ((String) -> Void)?
    private var mouseMonitor: Any?
    private var keyMonitor: Any?
    private var hotkeyRef: EventHotKeyRef?
    private var isDragging = false
    private var dragStartPoint: CGPoint = .zero
    /// Text captured at selection-detection time, before any click can disrupt it
    private var cachedSelectedText: String?

    /// The current hotkey config — set from AppModel.hotkeyString. Empty = hotkey off, pencil on.
    var hotkeyString: String = "" {
        didSet { reconfigureMode() }
    }

    func start(onTextSelected: @escaping (String) -> Void) {
        let hasPerm = axService.ensureAccessibilityPermission()
        let isTrusted = AXIsProcessTrusted()
        DRLogger.log("SelectionMonitor.start() — AX permission=\(hasPerm) isTrusted=\(isTrusted)", category: .monitor)
        self.onTextSelected = onTextSelected
        reconfigureMode()
    }

    /// Switch between pencil mode (mouse monitors) and hotkey mode
    private func reconfigureMode() {
        // Remove all existing monitors
        if let mouseMonitor { NSEvent.removeMonitor(mouseMonitor) }
        if let keyMonitor { NSEvent.removeMonitor(keyMonitor) }
        unregisterCarbonHotkey()
        mouseMonitor = nil
        keyMonitor = nil
        hideTrigger()

        guard onTextSelected != nil else { return }

        if hotkeyString.isEmpty {
            // Pencil mode: mouse/keyboard selection triggers pencil button
            DRLogger.log("Mode: PENCIL (highlight/double-click → pencil button)", category: .monitor)
            mouseMonitor = NSEvent.addGlobalMonitorForEvents(matching: [.leftMouseDown, .leftMouseUp, .leftMouseDragged]) { [weak self] event in
                Task { @MainActor in
                    self?.handleMouseEvent(event)
                }
            }
            keyMonitor = NSEvent.addGlobalMonitorForEvents(matching: [.keyUp]) { [weak self] event in
                Task { @MainActor in
                    self?.handleKeyEvent(event)
                }
            }
        } else {
            // Hotkey mode: only global hotkey triggers the panel
            DRLogger.log("Mode: HOTKEY (\(Self.hotkeyDisplayName(hotkeyString)))", category: .monitor)
            reinstallHotkeyMonitor()
        }
    }

    func stop() {
        if let mouseMonitor { NSEvent.removeMonitor(mouseMonitor) }
        if let keyMonitor { NSEvent.removeMonitor(keyMonitor) }
        unregisterCarbonHotkey()
        mouseMonitor = nil
        keyMonitor = nil
        hideTrigger()
    }

    // MARK: - Global Hotkey

    /// Parse "cmd+shift:15" into (modifierFlags, keyCode)
    static func parseHotkey(_ str: String) -> (NSEvent.ModifierFlags, UInt16)? {
        let parts = str.split(separator: ":")
        guard parts.count == 2, let keyCode = UInt16(parts[1]) else { return nil }

        var flags: NSEvent.ModifierFlags = []
        let modString = parts[0].lowercased()
        if modString.contains("cmd") || modString.contains("command") { flags.insert(.command) }
        if modString.contains("shift") { flags.insert(.shift) }
        if modString.contains("opt") || modString.contains("option") || modString.contains("alt") { flags.insert(.option) }
        if modString.contains("ctrl") || modString.contains("control") { flags.insert(.control) }

        return (flags, keyCode)
    }

    /// Human-readable label for a hotkey string
    static func hotkeyDisplayName(_ str: String) -> String {
        guard let (flags, keyCode) = parseHotkey(str) else { return str }
        var parts: [String] = []
        if flags.contains(.control) { parts.append("⌃") }
        if flags.contains(.option) { parts.append("⌥") }
        if flags.contains(.shift) { parts.append("⇧") }
        if flags.contains(.command) { parts.append("⌘") }
        parts.append(keyCodeToString(keyCode))
        return parts.joined()
    }

    private static func keyCodeToString(_ code: UInt16) -> String {
        let map: [UInt16: String] = [
            0: "A", 1: "S", 2: "D", 3: "F", 4: "H", 5: "G", 6: "Z", 7: "X",
            8: "C", 9: "V", 11: "B", 12: "Q", 13: "W", 14: "E", 15: "R",
            16: "Y", 17: "T", 31: "O", 32: "U", 34: "I", 35: "P", 37: "L",
            38: "J", 40: "K", 45: "N", 46: "M",
        ]
        return map[code] ?? "Key\(code)"
    }

    /// Register a system-wide hotkey using Carbon RegisterEventHotKey.
    /// This works without Accessibility permission and doesn't produce the system bonk sound.
    private func reinstallHotkeyMonitor() {
        unregisterCarbonHotkey()

        guard let (nsFlags, keyCode) = Self.parseHotkey(hotkeyString) else {
            DRLogger.log("Invalid hotkey string: \(hotkeyString)", category: .monitor)
            return
        }

        // Convert NSEvent modifier flags to Carbon modifier flags
        var carbonMods: UInt32 = 0
        if nsFlags.contains(.command) { carbonMods |= UInt32(cmdKey) }
        if nsFlags.contains(.shift) { carbonMods |= UInt32(shiftKey) }
        if nsFlags.contains(.option) { carbonMods |= UInt32(optionKey) }
        if nsFlags.contains(.control) { carbonMods |= UInt32(controlKey) }

        DRLogger.log("Registering Carbon hotkey: \(Self.hotkeyDisplayName(hotkeyString)) (keyCode=\(keyCode))", category: .monitor)

        // Install Carbon event handler (once)
        Self.installCarbonHandler()

        // Store self so the C callback can reach us
        Self.activeInstance = self

        var hotkeyID = EventHotKeyID(signature: OSType(0x4452), id: 1) // "DR"
        let status = RegisterEventHotKey(
            UInt32(keyCode),
            carbonMods,
            hotkeyID,
            GetApplicationEventTarget(),
            0,
            &hotkeyRef
        )

        if status != noErr {
            DRLogger.log("Failed to register Carbon hotkey: \(status)", category: .monitor)
        }
    }

    private func unregisterCarbonHotkey() {
        if let ref = hotkeyRef {
            UnregisterEventHotKey(ref)
            hotkeyRef = nil
        }
    }

    // MARK: - Carbon Event Handler (static)

    private static weak var activeInstance: SelectionMonitor?
    private static var carbonHandlerInstalled = false

    private static func installCarbonHandler() {
        guard !carbonHandlerInstalled else { return }
        carbonHandlerInstalled = true

        var eventType = EventTypeSpec(eventClass: OSType(kEventClassKeyboard), eventKind: UInt32(kEventHotKeyPressed))

        InstallEventHandler(
            GetApplicationEventTarget(),
            { (_, event, _) -> OSStatus in
                Task { @MainActor in
                    SelectionMonitor.activeInstance?.hotkeyTriggered()
                }
                return noErr
            },
            1,
            &eventType,
            nil,
            nil
        )
    }

    private func hotkeyTriggered() {
        DRLogger.log("Hotkey triggered: \(Self.hotkeyDisplayName(hotkeyString))", category: .monitor)
        hideTrigger()

        // Grab selected text via AX or Cmd+C fallback
        if let text = axService.readSelectedText(), !text.isEmpty {
            DRLogger.log("Hotkey AX got: '\(text.prefix(30))'", category: .monitor)
            DiffWindow.shared.anchorPoint = NSEvent.mouseLocation
            onTextSelected?(text)
            return
        }

        // Fallback: Cmd+C then read clipboard
        DRLogger.log("Hotkey AX failed, trying Cmd+C", category: .monitor)
        let pasteboard = NSPasteboard.general
        let savedChangeCount = pasteboard.changeCount

        let source = CGEventSource(stateID: .hidSystemState)
        let keyDown = CGEvent(keyboardEventSource: source, virtualKey: CGKeyCode(kVK_ANSI_C), keyDown: true)
        keyDown?.flags = .maskCommand
        let keyUp = CGEvent(keyboardEventSource: source, virtualKey: CGKeyCode(kVK_ANSI_C), keyDown: false)
        keyUp?.flags = .maskCommand
        keyDown?.post(tap: .cgSessionEventTap)
        keyUp?.post(tap: .cgSessionEventTap)

        DispatchQueue.main.asyncAfter(deadline: .now() + 0.3) { [weak self] in
            guard let self = self else { return }
            if pasteboard.changeCount != savedChangeCount {
                let text = (pasteboard.string(forType: .string) ?? "").trimmingCharacters(in: .whitespacesAndNewlines)
                if !text.isEmpty {
                    DRLogger.log("Hotkey clipboard got: '\(text.prefix(30))'", category: .monitor)
                    DiffWindow.shared.anchorPoint = NSEvent.mouseLocation
                    self.onTextSelected?(text)
                    return
                }
            }
            DRLogger.log("Hotkey: no text found", category: .monitor)
        }
    }

    private func handleMouseEvent(_ event: NSEvent) {
        switch event.type {
        case .leftMouseDown:
            isDragging = false
            dragStartPoint = NSEvent.mouseLocation
            // Don't hide trigger if clicking ON the trigger button itself
            if let tw = triggerWindow, tw.frame.contains(NSEvent.mouseLocation) {
                // Click is on the trigger — let the button handle it
                triggerButtonClicked()
                return
            }
            hideTrigger()

        case .leftMouseDragged:
            isDragging = true

        case .leftMouseUp:
            let pos = NSEvent.mouseLocation

            // After any mouseUp, check if there's actually selected text
            // This is more reliable than tracking drag distance
            DispatchQueue.main.asyncAfter(deadline: .now() + 0.2) { [weak self] in
                guard let self = self else { return }

                // Only show pencil when AX confirms selected text, or on drag
                let hasSelection: Bool
                if let text = self.axService.readSelectedText(), !text.isEmpty {
                    hasSelection = true
                } else {
                    hasSelection = self.isDragging
                }

                DRLogger.log("MouseUp hasSelection=\(hasSelection) clicks=\(event.clickCount) isDrag=\(self.isDragging)", category: .monitor)

                if hasSelection {
                    self.showTriggerAt(pos)
                }
            }
            isDragging = false

        default:
            break
        }
    }

    private func handleKeyEvent(_ event: NSEvent) {
        let isShift = event.modifierFlags.contains(.shift)
        let isCmd = event.modifierFlags.contains(.command)
        let key = Int(event.keyCode)
        let isArrowKey = [123, 124, 125, 126].contains(key)
        let isSelectAll = isCmd && key == kVK_ANSI_A

        if (isShift && isArrowKey) || isSelectAll {
            let pos = NSEvent.mouseLocation
            DispatchQueue.main.asyncAfter(deadline: .now() + 0.15) { [weak self] in
                self?.showTriggerAt(pos)
            }
        }
    }

    private func showTriggerAt(_ point: CGPoint) {
        hideTrigger()
        DRLogger.log("showTriggerAt(\(point))", category: .monitor)

        // Pre-capture selected text NOW, before any click can disrupt the selection
        preCaptureSelectedText()

        let buttonSize = CGSize(width: 32, height: 32)

        // Use drag coordinates; try AX selection bounds for better X position
        var originX = point.x
        var originY = point.y
        if let selBounds = axService.selectedTextBounds(), selBounds.minX > 0 {
            originX = selBounds.minX
        }
        let origin = CGPoint(x: originX, y: originY)

        // Use NSButton — SwiftUI buttons don't receive clicks in nonactivatingPanel
        let nsButton = NSButton(frame: CGRect(origin: .zero, size: buttonSize))
        nsButton.bezelStyle = .circular
        nsButton.image = NSImage(systemSymbolName: "pencil.and.outline", accessibilityDescription: "Rewrite")
        nsButton.imageScaling = .scaleProportionallyDown
        nsButton.contentTintColor = .white
        nsButton.isBordered = false
        nsButton.wantsLayer = true
        nsButton.layer?.backgroundColor = NSColor.systemBlue.cgColor
        nsButton.layer?.cornerRadius = buttonSize.width / 2
        nsButton.target = self
        nsButton.action = #selector(triggerButtonClicked)

        let panel = ClickablePanel(
            contentRect: NSRect(origin: origin, size: buttonSize),
            styleMask: [.borderless, .nonactivatingPanel],
            backing: .buffered,
            defer: false
        )
        panel.isOpaque = false
        panel.backgroundColor = .clear
        panel.level = .floating
        panel.hasShadow = true
        panel.isReleasedWhenClosed = false
        panel.collectionBehavior = [.canJoinAllSpaces, .fullScreenAuxiliary]
        panel.contentView = nsButton
        panel.orderFrontRegardless()
        self.triggerWindow = panel
    }

    /// Pre-capture text when selection is detected (before any click can disrupt it)
    private func preCaptureSelectedText() {
        cachedSelectedText = nil

        // Strategy 1: AX API
        if let text = axService.readSelectedText(), !text.isEmpty {
            DRLogger.log("Pre-capture AX got: '\(text.prefix(30))'", category: .monitor)
            cachedSelectedText = text
            return
        }
        DRLogger.log("Pre-capture AX failed, trying Cmd+C", category: .monitor)

        // Strategy 2: Cmd+C to clipboard (selection is still active at this point)
        let pasteboard = NSPasteboard.general
        let savedChangeCount = pasteboard.changeCount

        let source = CGEventSource(stateID: .hidSystemState)
        let keyDown = CGEvent(keyboardEventSource: source, virtualKey: CGKeyCode(kVK_ANSI_C), keyDown: true)
        keyDown?.flags = .maskCommand
        let keyUp = CGEvent(keyboardEventSource: source, virtualKey: CGKeyCode(kVK_ANSI_C), keyDown: false)
        keyUp?.flags = .maskCommand
        keyDown?.post(tap: .cgSessionEventTap)
        keyUp?.post(tap: .cgSessionEventTap)

        DispatchQueue.main.asyncAfter(deadline: .now() + 0.3) { [weak self] in
            guard let self = self else { return }
            // Only read if clipboard actually changed (Cmd+C succeeded)
            if pasteboard.changeCount != savedChangeCount {
                let text = (pasteboard.string(forType: .string) ?? "").trimmingCharacters(in: .whitespacesAndNewlines)
                if !text.isEmpty {
                    DRLogger.log("Pre-capture clipboard got: '\(text.prefix(30))'", category: .monitor)
                    self.cachedSelectedText = text
                } else {
                    DRLogger.log("Pre-capture clipboard empty", category: .monitor)
                }
            } else {
                DRLogger.log("Pre-capture clipboard unchanged (Cmd+C may have failed)", category: .monitor)
            }
        }
    }

    /// Use pre-captured text, or try again as last resort
    private func grabTextAndOpen() {
        DRLogger.log("grabTextAndOpen called", category: .monitor)

        // Use pre-captured text (captured when pencil was shown, before click disrupted selection)
        if let text = cachedSelectedText, !text.isEmpty {
            DRLogger.log("Using pre-captured text: '\(text.prefix(30))'", category: .monitor)
            cachedSelectedText = nil
            onTextSelected?(text)
            return
        }

        // Last resort: try AX (unlikely to work after click)
        if let text = axService.readSelectedText(), !text.isEmpty {
            DRLogger.log("AX got text (fallback): \(text.prefix(30))", category: .monitor)
            onTextSelected?(text)
            return
        }

        DRLogger.log("NO TEXT FOUND (pre-capture missed and AX failed)", category: .monitor)
    }

    @objc private func triggerButtonClicked() {
        DRLogger.log("Pencil CLICKED", category: .monitor)
        if let frame = triggerWindow?.frame {
            DiffWindow.shared.anchorPoint = CGPoint(x: frame.minX, y: frame.minY)
        }
        // Don't hide the trigger here — let the consumer decide.
        // Advanced mode calls hideTrigger() immediately; One-Click mode
        // calls startLoadingAnimation() and then stopLoadingAndHide()
        // when the rewrite finishes.
        grabTextAndOpen()
    }

    /// Show a pulsing pencil indicator to signal that a One-Click rewrite
    /// is in flight. If the trigger window is already visible (mouse-click
    /// path), animate it in place. Otherwise (hotkey path) create a new
    /// indicator at the current mouse location.
    func startLoadingAnimation() {
        if triggerWindow == nil {
            showLoadingIndicatorAt(NSEvent.mouseLocation)
        }
        guard let panel = triggerWindow,
              let button = panel.contentView as? NSButton,
              let layer = button.layer else { return }
        button.isEnabled = false
        let pulse = CABasicAnimation(keyPath: "opacity")
        pulse.fromValue = 1.0
        pulse.toValue = 0.25
        pulse.duration = 0.55
        pulse.autoreverses = true
        pulse.repeatCount = .infinity
        pulse.timingFunction = CAMediaTimingFunction(name: .easeInEaseOut)
        layer.add(pulse, forKey: "oneClickPulse")
    }

    /// Create a pencil-shaped loading indicator at the given screen point
    /// (used by the hotkey path where no pencil button was ever shown).
    private func showLoadingIndicatorAt(_ point: CGPoint) {
        let buttonSize = CGSize(width: 32, height: 32)
        let origin = CGPoint(x: point.x + 12, y: point.y + 12)

        let nsButton = NSButton(frame: CGRect(origin: .zero, size: buttonSize))
        nsButton.bezelStyle = .circular
        nsButton.image = NSImage(systemSymbolName: "pencil.and.outline", accessibilityDescription: "Rewriting")
        nsButton.imageScaling = .scaleProportionallyDown
        nsButton.contentTintColor = .white
        nsButton.isBordered = false
        nsButton.wantsLayer = true
        nsButton.layer?.backgroundColor = NSColor.systemBlue.cgColor
        nsButton.layer?.cornerRadius = buttonSize.width / 2
        nsButton.isEnabled = false

        let panel = ClickablePanel(
            contentRect: NSRect(origin: origin, size: buttonSize),
            styleMask: [.borderless, .nonactivatingPanel],
            backing: .buffered,
            defer: false
        )
        panel.isOpaque = false
        panel.backgroundColor = .clear
        panel.level = .floating
        panel.hasShadow = true
        panel.isReleasedWhenClosed = false
        panel.collectionBehavior = [.canJoinAllSpaces, .fullScreenAuxiliary]
        panel.contentView = nsButton
        panel.orderFrontRegardless()
        self.triggerWindow = panel
    }

    /// Stop the loading animation (if any) and hide the trigger window.
    func stopLoadingAndHide() {
        if let button = triggerWindow?.contentView as? NSButton {
            button.layer?.removeAnimation(forKey: "oneClickPulse")
            button.isEnabled = true
        }
        hideTrigger()
    }

    func hideTrigger() {
        triggerWindow?.orderOut(nil)
        triggerWindow = nil
    }
}

/// NSPanel subclass that accepts first mouse click without requiring activation
private class ClickablePanel: NSPanel {
    override var canBecomeKey: Bool { true }
}

