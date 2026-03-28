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
                // Drag selection
                isDragging = false
                let endPoint = NSEvent.mouseLocation
                let leftX = min(dragStartPoint.x, endPoint.x)
                let bottomY = min(dragStartPoint.y, endPoint.y)
                let position = CGPoint(x: leftX, y: bottomY)
                DispatchQueue.main.asyncAfter(deadline: .now() + 0.15) { [weak self] in
                    self?.showTriggerAt(position)
                }
            } else if event.clickCount >= 2 {
                // Double-click (word) or triple-click (line/paragraph)
                let pos = NSEvent.mouseLocation
                DispatchQueue.main.asyncAfter(deadline: .now() + 0.2) { [weak self] in
                    self?.showTriggerAt(pos)
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

        // Use drag coordinates; try AX selection bounds for better X position
        var originX = point.x
        var originY = point.y
        if let selBounds = axService.selectedTextBounds(), selBounds.minX > 0 {
            originX = selBounds.minX
        }
        let origin = CGPoint(x: originX, y: originY)

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
