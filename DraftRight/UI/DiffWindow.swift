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
        let size = CGSize(width: 500, height: 300)
        hosting.frame = CGRect(origin: .zero, size: size)

        let screen = NSScreen.main?.visibleFrame ?? NSRect(x: 0, y: 0, width: 1440, height: 900)
        let anchor = anchorPoint ?? NSEvent.mouseLocation

        // anchor = pencil icon's screen position
        // Place panel TOP at the icon, extending DOWNWARD
        // y in macOS = bottom edge, so y = iconTop - panelHeight to get top at icon
        // But that goes down. We want top at icon: y = anchor.y + 32 - size.height
        // Actually simpler: just put the panel origin (bottom-left) below the icon
        var x = anchor.x
        var y = anchor.y - size.height / 2 + 32

        // Clamp to screen edges
        if y + size.height > screen.maxY { y = screen.maxY - size.height }
        if y < screen.minY { y = screen.minY }
        x = max(screen.minX, min(x, screen.maxX - size.width))

        let nsPanel = NSPanel(
            contentRect: NSRect(origin: CGPoint(x: x, y: y), size: size),
            styleMask: [.borderless, .nonactivatingPanel],
            backing: .buffered,
            defer: false
        )
        nsPanel.isOpaque = false
        nsPanel.backgroundColor = .clear
        nsPanel.level = .floating
        nsPanel.hasShadow = true
        nsPanel.isReleasedWhenClosed = false
        nsPanel.collectionBehavior = [.canJoinAllSpaces, .fullScreenAuxiliary]
        nsPanel.contentView = hosting
        nsPanel.isMovableByWindowBackground = true
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

/// SwiftUI wrapper that observes the model and renders the panel
struct RewritePanelContainer: View {
    let original: String
    @ObservedObject var model: RewritePanelModel
    let onToneSelected: (Tone) -> Void
    let onReplace: (String) -> Void
    let onCopy: (String) -> Void
    let onCancel: () -> Void

    var body: some View {
        VStack(alignment: .leading, spacing: 0) {
            // Header
            HStack {
                Image(systemName: "pencil.and.outline")
                    .foregroundColor(.accentColor)
                Text("DraftRight")
                    .font(.headline)
                Spacer()
                Button(action: onCancel) {
                    Image(systemName: "xmark")
                        .foregroundColor(.secondary)
                }
                .buttonStyle(.borderless)
            }
            .padding(.horizontal, 14)
            .padding(.top, 12)
            .padding(.bottom, 8)

            // Tone tabs
            HStack(spacing: 4) {
                ForEach(Tone.allCases) { tone in
                    Button(action: {
                        onToneSelected(tone)
                    }) {
                        HStack(spacing: 4) {
                            Image(systemName: iconName(for: tone))
                                .font(.caption2)
                            Text(tone.displayName)
                                .font(.caption)
                        }
                        .padding(.horizontal, 10)
                        .padding(.vertical, 6)
                        .background(model.selectedTone == tone ? Color.accentColor.opacity(0.2) : Color.clear)
                        .foregroundColor(model.selectedTone == tone ? .accentColor : .secondary)
                        .cornerRadius(6)
                    }
                    .buttonStyle(.plain)
                }
            }
            .padding(.horizontal, 14)
            .padding(.bottom, 8)

            Divider()

            // Content
            if model.isLoading {
                HStack {
                    Spacer()
                    VStack(spacing: 8) {
                        ProgressView()
                            .scaleEffect(0.8)
                        Text("Rewriting...")
                            .font(.caption)
                            .foregroundColor(.secondary)
                    }
                    Spacer()
                }
                .frame(minHeight: 140)
            } else if let error = model.errorMessage {
                HStack {
                    Spacer()
                    VStack(spacing: 8) {
                        Image(systemName: "exclamationmark.triangle")
                            .foregroundColor(.orange)
                        Text(error)
                            .font(.caption)
                            .foregroundColor(.secondary)
                            .multilineTextAlignment(.center)
                    }
                    Spacer()
                }
                .frame(minHeight: 140)
                .padding(.horizontal, 14)
            } else if let rewritten = model.rewritten {
                // Side-by-side diff
                HStack(alignment: .top, spacing: 1) {
                    VStack(alignment: .leading, spacing: 4) {
                        Text("Original")
                            .font(.caption).foregroundColor(.secondary).fontWeight(.semibold)
                        ScrollView {
                            diffText(tokens: WordDiff.diff(old: original, new: rewritten).oldTokens, highlightKind: .deleted, color: .red)
                                .frame(maxWidth: .infinity, alignment: .leading)
                                .textSelection(.enabled)
                        }
                    }
                    .frame(maxWidth: .infinity)
                    .padding(8)
                    .background(Color.red.opacity(0.03))
                    .cornerRadius(6)

                    VStack(alignment: .leading, spacing: 4) {
                        Text("Rewritten")
                            .font(.caption).foregroundColor(.secondary).fontWeight(.semibold)
                        ScrollView {
                            diffText(tokens: WordDiff.diff(old: original, new: rewritten).newTokens, highlightKind: .inserted, color: .green)
                                .frame(maxWidth: .infinity, alignment: .leading)
                                .textSelection(.enabled)
                        }
                    }
                    .frame(maxWidth: .infinity)
                    .padding(8)
                    .background(Color.green.opacity(0.03))
                    .cornerRadius(6)
                }
                .padding(.horizontal, 14)
                .padding(.vertical, 8)
            } else {
                HStack {
                    Spacer()
                    Text("Select a tone above to rewrite your text")
                        .font(.caption)
                        .foregroundColor(.secondary)
                    Spacer()
                }
                .frame(minHeight: 100)
            }

            // Footer buttons
            if model.rewritten != nil {
                Divider()
                HStack {
                    Spacer()
                    Button("Cancel", action: onCancel)
                        .keyboardShortcut(.cancelAction)
                    Button("Copy") { onCopy(model.rewritten!) }
                    Button("Replace") { onReplace(model.rewritten!) }
                        .keyboardShortcut(.defaultAction)
                }
                .padding(.horizontal, 14)
                .padding(.vertical, 10)
            }
        }
        .background(.ultraThickMaterial, in: RoundedRectangle(cornerRadius: 12, style: .continuous))
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
        }
    }
}
