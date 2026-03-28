import SwiftUI
import AppKit
import Carbon.HIToolbox

@MainActor
final class DiffWindow {
    static let shared = DiffWindow()

    private var window: NSPanel?
    private var clickMonitor: Any?
    private var keyMonitor: Any?

    func present(
        tone: Tone,
        original: String,
        rewritten: String,
        replaceHandler: @escaping () -> Void,
        copyHandler: @escaping () -> Void
    ) {
        close()

        let content = DiffView(
            tone: tone,
            original: original,
            rewritten: rewritten,
            onReplace: {
                replaceHandler()
                self.close()
            },
            onCopy: {
                copyHandler()
                self.close()
            },
            onCancel: { self.close() },
            onRetry: nil
        )

        let hosting = NSHostingView(rootView: content)
        let size = CGSize(width: 600, height: 400)
        hosting.frame = CGRect(origin: .zero, size: size)

        let cursor = NSEvent.mouseLocation
        let origin = CGPoint(
            x: max(0, cursor.x - 10),
            y: max(0, cursor.y - size.height - 20)
        )

        let panel = NSPanel(
            contentRect: NSRect(origin: origin, size: size),
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
        panel.isMovableByWindowBackground = true
        panel.makeKeyAndOrderFront(nil)
        self.window = panel

        clickMonitor = NSEvent.addGlobalMonitorForEvents(matching: [.leftMouseDown, .rightMouseDown]) { [weak self] _ in
            self?.close()
        }
        keyMonitor = NSEvent.addLocalMonitorForEvents(matching: .keyDown) { [weak self] event in
            if event.keyCode == UInt16(kVK_Escape) {
                self?.close()
                return nil
            }
            return event
        }
    }

    func showLoading(tone: Tone) {
        close()

        let content = VStack(spacing: 12) {
            ProgressView()
                .scaleEffect(0.8)
            Text("Rewriting as \(tone.displayName)...")
                .font(.subheadline)
                .foregroundColor(.secondary)
        }
        .padding(24)
        .background(.ultraThickMaterial, in: RoundedRectangle(cornerRadius: 12, style: .continuous))

        let hosting = NSHostingView(rootView: content)
        let size = CGSize(width: 240, height: 80)
        hosting.frame = CGRect(origin: .zero, size: size)

        let cursor = NSEvent.mouseLocation
        let origin = CGPoint(x: max(0, cursor.x - 10), y: max(0, cursor.y - size.height - 20))

        let panel = NSPanel(
            contentRect: NSRect(origin: origin, size: size),
            styleMask: [.borderless, .nonactivatingPanel],
            backing: .buffered,
            defer: false
        )
        panel.isOpaque = false
        panel.backgroundColor = .clear
        panel.level = .floating
        panel.hasShadow = true
        panel.isReleasedWhenClosed = false
        panel.contentView = hosting
        panel.makeKeyAndOrderFront(nil)
        self.window = panel
    }

    func close() {
        if let clickMonitor {
            NSEvent.removeMonitor(clickMonitor)
            self.clickMonitor = nil
        }
        if let keyMonitor {
            NSEvent.removeMonitor(keyMonitor)
            self.keyMonitor = nil
        }
        window?.orderOut(nil)
        window = nil
    }
}
