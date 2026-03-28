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
    private var isDragging = false
    private var dragStartPoint: CGPoint = .zero

    func start(onTextSelected: @escaping (String) -> Void) {
        _ = axService.ensureAccessibilityPermission()
        self.onTextSelected = onTextSelected

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
    }

    func stop() {
        if let mouseMonitor { NSEvent.removeMonitor(mouseMonitor) }
        if let keyMonitor { NSEvent.removeMonitor(keyMonitor) }
        mouseMonitor = nil
        keyMonitor = nil
        hideTrigger()
    }

    private func handleMouseEvent(_ event: NSEvent) {
        switch event.type {
        case .leftMouseDown:
            isDragging = false
            dragStartPoint = NSEvent.mouseLocation
            hideTrigger()

        case .leftMouseDragged:
            let current = NSEvent.mouseLocation
            let distance = hypot(current.x - dragStartPoint.x, current.y - dragStartPoint.y)
            if distance > 5 {
                isDragging = true
            }

        case .leftMouseUp:
            if isDragging {
                isDragging = false
                let endPoint = NSEvent.mouseLocation
                // Bottom-left of the selection: leftmost X, lowest Y
                let leftX = min(dragStartPoint.x, endPoint.x)
                let bottomY = min(dragStartPoint.y, endPoint.y)
                let position = CGPoint(x: leftX, y: bottomY)
                DispatchQueue.main.asyncAfter(deadline: .now() + 0.15) { [weak self] in
                    self?.showTriggerAt(position)
                }
            }

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

        let buttonSize = CGSize(width: 32, height: 32)

        // Try to position at the bottom-left of the focused text field (like Grammarly)
        let origin: CGPoint
        if let frame = axService.focusedElementFrame() {
            origin = CGPoint(
                x: frame.minX + 6,
                y: frame.minY + 6
            )
        } else {
            // Fallback: at the selection point
            origin = CGPoint(
                x: point.x,
                y: point.y
            )
        }

        let button = TriggerButtonView { [weak self] in
            // Save the trigger window's actual screen position right before opening panel
            if let frame = self?.triggerWindow?.frame {
                DiffWindow.shared.anchorPoint = CGPoint(x: frame.minX, y: frame.minY)
            }
            self?.hideTrigger()
            self?.grabTextAndOpen()
        }

        let hosting = NSHostingView(rootView: button)
        hosting.autoresizingMask = [.width, .height]
        hosting.frame = CGRect(origin: .zero, size: buttonSize)

        let panel = NSPanel(
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
        panel.contentView = hosting
        panel.orderFrontRegardless()
        self.triggerWindow = panel
    }

    /// Try AX first to get selected text; fall back to Cmd+C clipboard grab
    private func grabTextAndOpen() {
        // Strategy 1: AX API
        if let text = axService.readSelectedText(), !text.isEmpty {
            onTextSelected?(text)
            return
        }

        // Strategy 2: Cmd+C to clipboard
        let pasteboard = NSPasteboard.general
        let savedItems = pasteboard.pasteboardItems?.compactMap { item -> (NSPasteboard.PasteboardType, Data)? in
            guard let type = item.types.first, let data = item.data(forType: type) else { return nil }
            return (type, data)
        } ?? []

        let source = CGEventSource(stateID: .hidSystemState)
        let keyDown = CGEvent(keyboardEventSource: source, virtualKey: CGKeyCode(kVK_ANSI_C), keyDown: true)
        keyDown?.flags = .maskCommand
        let keyUp = CGEvent(keyboardEventSource: source, virtualKey: CGKeyCode(kVK_ANSI_C), keyDown: false)
        keyUp?.flags = .maskCommand
        keyDown?.post(tap: .cgSessionEventTap)
        keyUp?.post(tap: .cgSessionEventTap)

        DispatchQueue.main.asyncAfter(deadline: .now() + 0.2) { [weak self] in
            let text = pasteboard.string(forType: .string) ?? ""

            pasteboard.clearContents()
            for (type, data) in savedItems {
                pasteboard.setData(data, forType: type)
            }

            let trimmed = text.trimmingCharacters(in: .whitespacesAndNewlines)
            if !trimmed.isEmpty {
                self?.onTextSelected?(trimmed)
            }
        }
    }

    func hideTrigger() {
        triggerWindow?.orderOut(nil)
        triggerWindow = nil
    }
}
