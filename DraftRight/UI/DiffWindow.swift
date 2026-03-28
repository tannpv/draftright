import SwiftUI
import AppKit
import Carbon.HIToolbox

/// Observable model that drives the RewritePanel content.
@MainActor
final class RewritePanelModel: ObservableObject {
    @Published var rewritten: String? = nil
    @Published var isLoading: Bool = false
    @Published var errorMessage: String? = nil
    @Published var selectedTone: Tone? = nil

    func startLoading(tone: Tone) {
        selectedTone = tone
        isLoading = true
        rewritten = nil
        errorMessage = nil
    }

    func setResult(_ text: String) {
        rewritten = text
        isLoading = false
        errorMessage = nil
    }

    func setError(_ message: String) {
        errorMessage = message
        isLoading = false
        rewritten = nil
    }

    func reset() {
        rewritten = nil
        isLoading = false
        errorMessage = nil
        selectedTone = nil
    }
}

@MainActor
final class DiffWindow {
    static let shared = DiffWindow()

    private var window: NSPanel?
    private var keyMonitor: Any?
    private var previousApp: NSRunningApplication?
    let model = RewritePanelModel()

    /// Called when user picks a tone inside the panel
    var onToneSelected: ((Tone) -> Void)?

    /// anchorPoint: where the pencil icon was (bottom-left of selection area)
    var anchorPoint: CGPoint?

    func presentPanel(
        original: String,
        onToneSelected: @escaping (Tone) -> Void,
        onReplace: @escaping (String) -> Void,
        onCopy: @escaping (String) -> Void
    ) {
        close()
        model.reset()

        previousApp = NSWorkspace.shared.frontmostApplication
        self.onToneSelected = onToneSelected

        let panel = RewritePanelContainer(
            original: original,
            model: model,
            onToneSelected: { tone in
                onToneSelected(tone)
            },
            onReplace: { [weak self] text in
                // Copy to clipboard first, then close panel and re-focus original app
                ClipboardHelper.copy(text: text)
                self?.close()
                self?.reactivatePreviousApp {
                    // App is re-focused, selection should be restored — paste over it
                    ClipboardHelper.pasteFromClipboard()
                }
            },
            onCopy: { text in
                onCopy(text)
            },
            onCancel: { [weak self] in
                self?.close()
            }
        )

        let hosting = NSHostingView(rootView: panel)
        let size = CGSize(width: 500, height: 420)
        hosting.frame = CGRect(origin: .zero, size: size)

        let screen = NSScreen.main?.visibleFrame ?? NSRect(x: 0, y: 0, width: 1440, height: 900)
        let anchor = anchorPoint ?? NSEvent.mouseLocation

        // Panel bottom edge just above the pencil icon (icon top = anchor.y + 32)
        var x = anchor.x
        var y = anchor.y + 32

        // Clamp to screen edges
        if y + size.height > screen.maxY { y = screen.maxY - size.height }
        if y < screen.minY { y = screen.minY }
        x = max(screen.minX, min(x, screen.maxX - size.width))

        let nsPanel = NSPanel(
            contentRect: NSRect(origin: CGPoint(x: x, y: y), size: size),
            styleMask: [.titled, .resizable, .nonactivatingPanel, .fullSizeContentView],
            backing: .buffered,
            defer: false
        )
        nsPanel.titlebarAppearsTransparent = true
        nsPanel.titleVisibility = .hidden
        nsPanel.standardWindowButton(.closeButton)?.isHidden = true
        nsPanel.standardWindowButton(.miniaturizeButton)?.isHidden = true
        nsPanel.standardWindowButton(.zoomButton)?.isHidden = true
        nsPanel.isOpaque = false
        nsPanel.backgroundColor = .clear
        nsPanel.level = .floating
        nsPanel.hasShadow = true
        nsPanel.isReleasedWhenClosed = false
        nsPanel.collectionBehavior = [.canJoinAllSpaces, .fullScreenAuxiliary]
        nsPanel.contentView = hosting
        nsPanel.isMovableByWindowBackground = true
        nsPanel.minSize = NSSize(width: 400, height: 250)
        nsPanel.makeKeyAndOrderFront(nil)
        self.window = nsPanel

        keyMonitor = NSEvent.addLocalMonitorForEvents(matching: .keyDown) { [weak self] event in
            if event.keyCode == UInt16(kVK_Escape) {
                self?.close()
                return nil
            }
            return event
        }
    }

    func close() {
        if let keyMonitor {
            NSEvent.removeMonitor(keyMonitor)
            self.keyMonitor = nil
        }
        window?.orderOut(nil)
        window = nil
    }

    private func reactivatePreviousApp(then action: @escaping () -> Void) {
        if let app = previousApp {
            app.activate(options: .activateIgnoringOtherApps)
            // Wait for app to regain focus, then select all text in the field, then act
            DispatchQueue.main.asyncAfter(deadline: .now() + 0.4) {
                ClipboardHelper.selectAll()
                DispatchQueue.main.asyncAfter(deadline: .now() + 0.15) {
                    action()
                }
            }
        } else {
            action()
        }
    }
}

/// Compact panel: tone icons + action buttons in one toolbar row, content fills the rest
struct RewritePanelContainer: View {
    let original: String
    @ObservedObject var model: RewritePanelModel
    let onToneSelected: (Tone) -> Void
    let onReplace: (String) -> Void
    let onCopy: (String) -> Void
    let onCancel: () -> Void

    var body: some View {
        VStack(alignment: .leading, spacing: 0) {
            // Single toolbar row: tone icons | spacer | action buttons + close
            HStack(spacing: 2) {
                ForEach(Tone.allCases) { tone in
                    Button(action: { onToneSelected(tone) }) {
                        Image(systemName: iconName(for: tone))
                            .font(.system(size: 12))
                            .frame(width: 28, height: 24)
                            .background(model.selectedTone == tone ? Color.accentColor.opacity(0.2) : Color.clear)
                            .foregroundColor(model.selectedTone == tone ? .accentColor : .secondary)
                            .cornerRadius(4)
                    }
                    .buttonStyle(.plain)
                    .help(tone.displayName)
                }

                Spacer()

                if model.rewritten != nil {
                    Button("Replace") { onReplace(model.rewritten!) }
                        .font(.caption)
                        .keyboardShortcut(.defaultAction)
                    Button("Copy") { onCopy(model.rewritten!) }
                        .font(.caption)
                }

                Button(action: onCancel) {
                    Image(systemName: "xmark")
                        .font(.system(size: 10))
                        .foregroundColor(.secondary)
                        .frame(width: 20, height: 20)
                }
                .buttonStyle(.borderless)
            }
            .padding(.horizontal, 10)
            .padding(.vertical, 6)

            // Content fills the rest
            if model.isLoading {
                HStack {
                    Spacer()
                    VStack(spacing: 6) {
                        ProgressView().scaleEffect(0.7)
                        Text("Rewriting...")
                            .font(.caption2)
                            .foregroundColor(.secondary)
                    }
                    Spacer()
                }
                .frame(maxHeight: .infinity)
            } else if let error = model.errorMessage {
                HStack {
                    Spacer()
                    VStack(spacing: 6) {
                        Image(systemName: "exclamationmark.triangle")
                            .foregroundColor(.orange)
                        Text(error)
                            .font(.caption2)
                            .foregroundColor(.secondary)
                            .multilineTextAlignment(.center)
                    }
                    Spacer()
                }
                .frame(maxHeight: .infinity)
                .padding(.horizontal, 10)
            } else if let rewritten = model.rewritten {
                HStack(alignment: .top, spacing: 1) {
                    VStack(alignment: .leading, spacing: 2) {
                        Text("Original").font(.caption2).foregroundColor(.secondary).fontWeight(.semibold)
                        ScrollView {
                            diffText(tokens: WordDiff.diff(old: original, new: rewritten).oldTokens, highlightKind: .deleted, color: .red)
                                .font(.system(size: 12))
                                .frame(maxWidth: .infinity, alignment: .leading)
                                .textSelection(.enabled)
                        }
                    }
                    .frame(maxWidth: .infinity, maxHeight: .infinity)
                    .padding(6)
                    .background(Color.red.opacity(0.03))
                    .cornerRadius(4)

                    VStack(alignment: .leading, spacing: 2) {
                        Text("Rewritten").font(.caption2).foregroundColor(.secondary).fontWeight(.semibold)
                        ScrollView {
                            diffText(tokens: WordDiff.diff(old: original, new: rewritten).newTokens, highlightKind: .inserted, color: .green)
                                .font(.system(size: 12))
                                .frame(maxWidth: .infinity, alignment: .leading)
                                .textSelection(.enabled)
                        }
                    }
                    .frame(maxWidth: .infinity, maxHeight: .infinity)
                    .padding(6)
                    .background(Color.green.opacity(0.03))
                    .cornerRadius(4)
                }
                .padding(.horizontal, 8)
                .padding(.bottom, 8)
            } else {
                HStack {
                    Spacer()
                    Text("Pick a tone to rewrite")
                        .font(.caption2)
                        .foregroundColor(.secondary)
                    Spacer()
                }
                .frame(maxHeight: .infinity)
            }
        }
        .frame(maxWidth: .infinity, maxHeight: .infinity)
        .background(.ultraThickMaterial, in: RoundedRectangle(cornerRadius: 10, style: .continuous))
    }

    private func diffText(tokens: [DiffToken], highlightKind: DiffKind, color: Color) -> Text {
        var result = Text("")
        for token in tokens {
            if token.kind == highlightKind {
                var attributed = AttributedString(token.text)
                attributed.foregroundColor = .white
                attributed.backgroundColor = NSColor(color.opacity(0.7))
                attributed.inlinePresentationIntent = .stronglyEmphasized
                result = result + Text(attributed)
            } else if token.kind == .equal {
                result = result + Text(token.text)
            }
        }
        return result
    }

    private func iconName(for tone: Tone) -> String {
        switch tone {
        case .simple: return "text.word.spacing"
        case .natural: return "bubble.left"
        case .polished: return "sparkles"
        case .concise: return "arrow.down.right.and.arrow.up.left"
        case .technical: return "wrench.and.screwdriver"
        case .translate: return "globe"
        }
    }
}
