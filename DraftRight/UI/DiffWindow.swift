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
    @Published var grammarResult: GrammarResult? = nil

    func startLoading(tone: Tone) {
        selectedTone = tone
        isLoading = true
        rewritten = nil
        errorMessage = nil
        grammarResult = nil
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
        grammarResult = nil
    }

    func setGrammarResult(_ result: GrammarResult) {
        grammarResult = result
        isLoading = false
        errorMessage = nil
    }

    func handleRewriteResponse(_ text: String, tone: Tone) {
        if tone == .grammarCheck,
           let data = text.data(using: .utf8),
           let grammar = try? JSONDecoder().decode(GrammarResult.self, from: data) {
            setGrammarResult(grammar)
        } else {
            setResult(text)
        }
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
        visibleTones: [Tone]? = nil,
        autoRunTone: Tone? = nil,
        onToneSelected: @escaping (Tone) -> Void,
        onReplace: @escaping (String) -> Void,
        onCopy: @escaping (String) -> Void
    ) {
        DRLogger.log("presentPanel called, original length=\(original.count)", category: .panel)
        close()
        model.reset()

        previousApp = NSWorkspace.shared.frontmostApplication
        self.onToneSelected = onToneSelected

        let panel = RewritePanelContainer(
            original: original,
            model: model,
            visibleTones: visibleTones ?? Tone.allCases.map { $0 },
            autoRunTone: autoRunTone,
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
        let fittingSize = hosting.fittingSize
        let size = CGSize(
            width: max(fittingSize.width, 520),
            height: max(fittingSize.height, 420)
        )
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
        nsPanel.minSize = NSSize(width: 450, height: 350)
        nsPanel.makeKeyAndOrderFront(nil)
        self.window = nsPanel
        DRLogger.log("Panel shown at x=\(x) y=\(y) size=\(size)", category: .panel)

        keyMonitor = NSEvent.addLocalMonitorForEvents(matching: .keyDown) { [weak self] event in
            if event.keyCode == UInt16(kVK_Escape) {
                self?.close()
                return nil
            }
            return event
        }
    }

    func close() {
        DRLogger.log("Panel closing", category: .panel)
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
    let visibleTones: [Tone]
    let autoRunTone: Tone?
    let onToneSelected: (Tone) -> Void
    let onReplace: (String) -> Void
    let onCopy: (String) -> Void
    let onCancel: () -> Void

    var body: some View {
        VStack(alignment: .leading, spacing: 0) {
            // Header: title + close
            HStack {
                Image(systemName: "pencil.and.outline")
                    .foregroundColor(.accentColor)
                    .font(.system(size: 14))
                Text("DraftRight")
                    .font(.system(size: 13, weight: .semibold))
                Spacer()

                if model.rewritten != nil {
                    Button(action: { onReplace(model.rewritten!) }) {
                        Label("Replace", systemImage: "arrow.right.doc.on.clipboard")
                            .font(.system(size: 11, weight: .medium))
                    }
                    .keyboardShortcut(.defaultAction)
                    .buttonStyle(.borderedProminent)
                    .controlSize(.small)

                    Button(action: { onCopy(model.rewritten!) }) {
                        Label("Copy", systemImage: "doc.on.doc")
                            .font(.system(size: 11, weight: .medium))
                    }
                    .buttonStyle(.bordered)
                    .controlSize(.small)
                }

                Button(action: onCancel) {
                    Image(systemName: "xmark.circle.fill")
                        .font(.system(size: 14))
                        .foregroundColor(.secondary)
                }
                .buttonStyle(.borderless)
            }
            .padding(.horizontal, 12)
            .padding(.top, 10)
            .padding(.bottom, 6)

            // Tone picker row with labels
            ScrollView(.horizontal, showsIndicators: false) {
                HStack(spacing: 4) {
                    ForEach(visibleTones) { tone in
                        Button(action: { onToneSelected(tone) }) {
                            HStack(spacing: 4) {
                                toneIcon(for: tone)
                                Text(tone.displayName)
                                    .font(.system(size: 11))
                                    .lineLimit(1)
                            }
                            .padding(.horizontal, 8)
                            .padding(.vertical, 5)
                            .background(
                                model.selectedTone == tone
                                    ? Color.accentColor.opacity(0.2)
                                    : Color.primary.opacity(0.05)
                            )
                            .foregroundColor(model.selectedTone == tone ? .accentColor : .secondary)
                            .cornerRadius(6)
                        }
                        .buttonStyle(.plain)
                        .help(tone.displayName)
                    }
                }
                .padding(.horizontal, 12)
            }
            .padding(.bottom, 8)

            // Content fills the rest
            if model.isLoading {
                HStack {
                    Spacer()
                    VStack(spacing: 8) {
                        ProgressView().scaleEffect(0.8)
                        Text("Rewriting with \(model.selectedTone?.displayName ?? "AI")...")
                            .font(.system(size: 12))
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
                            .textSelection(.enabled)
                        Button("Copy Error") {
                            NSPasteboard.general.clearContents()
                            NSPasteboard.general.setString(error, forType: .string)
                        }
                        .font(.caption2)
                    }
                    Spacer()
                }
                .frame(maxHeight: .infinity)
                .padding(.horizontal, 10)
            } else if let grammarResult = model.grammarResult {
                GrammarCheckView(
                    originalText: original,
                    result: grammarResult,
                    onReplace: onReplace,
                    onCopy: onCopy
                )
            } else if let rewritten = model.rewritten {
                HStack(alignment: .top, spacing: 2) {
                    VStack(alignment: .leading, spacing: 4) {
                        HStack(spacing: 4) {
                            Circle().fill(Color.red.opacity(0.6)).frame(width: 6, height: 6)
                            Text("Original").font(.system(size: 11, weight: .semibold)).foregroundColor(.secondary)
                        }
                        ScrollView {
                            diffText(tokens: WordDiff.diff(old: original, new: rewritten).oldTokens, highlightKind: .deleted, color: .red)
                                .font(.system(size: 13))
                                .lineSpacing(3)
                                .frame(maxWidth: .infinity, alignment: .leading)
                                .textSelection(.enabled)
                        }
                    }
                    .frame(maxWidth: .infinity, maxHeight: .infinity)
                    .padding(10)
                    .background(Color.red.opacity(0.03))
                    .cornerRadius(6)

                    VStack(alignment: .leading, spacing: 4) {
                        HStack(spacing: 4) {
                            Circle().fill(Color.green.opacity(0.6)).frame(width: 6, height: 6)
                            Text("Rewritten").font(.system(size: 11, weight: .semibold)).foregroundColor(.secondary)
                        }
                        ScrollView {
                            diffText(tokens: WordDiff.diff(old: original, new: rewritten).newTokens, highlightKind: .inserted, color: .green)
                                .font(.system(size: 13))
                                .lineSpacing(3)
                                .frame(maxWidth: .infinity, alignment: .leading)
                                .textSelection(.enabled)
                        }
                    }
                    .frame(maxWidth: .infinity, maxHeight: .infinity)
                    .padding(10)
                    .background(Color.green.opacity(0.03))
                    .cornerRadius(6)
                }
                .padding(.horizontal, 10)
                .padding(.bottom, 10)
            } else {
                HStack {
                    Spacer()
                    VStack(spacing: 6) {
                        Image(systemName: "hand.tap")
                            .font(.system(size: 24))
                            .foregroundColor(.secondary.opacity(0.5))
                        Text("Pick a tone above to rewrite your text")
                            .font(.system(size: 12))
                            .foregroundColor(.secondary)
                    }
                    Spacer()
                }
                .frame(maxHeight: .infinity)
            }
        }
        .frame(maxWidth: .infinity, maxHeight: .infinity)
        .background(.ultraThickMaterial, in: RoundedRectangle(cornerRadius: 10, style: .continuous))
        .onAppear {
            // Auto-run the default tab when panel opens
            if let tone = autoRunTone, model.selectedTone == nil {
                onToneSelected(tone)
            }
        }
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

    @ViewBuilder
    private func toneIcon(for tone: Tone) -> some View {
        if tone == .claude, let img = NSImage.claudeIcon {
            Image(nsImage: img)
                .resizable()
                .aspectRatio(contentMode: .fit)
                .frame(width: 14, height: 14)
        } else {
            Image(systemName: tone.sfSymbol)
                .font(.system(size: 12))
        }
    }
}

// MARK: - Shared tone icon helper

extension NSImage {
    static let claudeIcon: NSImage? = {
        if let img = NSImage(named: "claude-icon") { return img }
        // SPM bundles resources in Bundle.module
        if let url = Bundle.module.url(forResource: "claude-icon", withExtension: "png") {
            return NSImage(contentsOf: url)
        }
        if let path = Bundle.main.path(forResource: "claude-icon", ofType: "png") {
            return NSImage(contentsOfFile: path)
        }
        return nil
    }()
}
