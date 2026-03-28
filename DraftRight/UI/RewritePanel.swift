import SwiftUI

/// Single-window Grammarly-style rewrite panel.
/// Header has tone tabs; body shows loading → diff result; footer has Replace/Copy/Cancel.
struct RewritePanel: View {
    let original: String
    let onToneSelected: (Tone) -> Void
    let onReplace: (String) -> Void
    let onCopy: (String) -> Void
    let onCancel: () -> Void

    @State private var selectedTone: Tone? = nil
    @State private var rewritten: String? = nil
    @State private var isLoading = false
    @State private var errorMessage: String? = nil

    var body: some View {
        VStack(alignment: .leading, spacing: 0) {
            // Header: app name + close
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
                        selectTone(tone)
                    }) {
                        HStack(spacing: 4) {
                            Image(systemName: iconName(for: tone))
                                .font(.caption2)
                            Text(tone.displayName)
                                .font(.caption)
                        }
                        .padding(.horizontal, 10)
                        .padding(.vertical, 6)
                        .background(selectedTone == tone ? Color.accentColor.opacity(0.2) : Color.clear)
                        .foregroundColor(selectedTone == tone ? .accentColor : .secondary)
                        .cornerRadius(6)
                    }
                    .buttonStyle(.plain)
                }
            }
            .padding(.horizontal, 14)
            .padding(.bottom, 8)

            Divider()

            // Content area
            if isLoading {
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
                .frame(minHeight: 120)
            } else if let error = errorMessage {
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
                .frame(minHeight: 120)
                .padding(.horizontal, 14)
            } else if let rewritten = rewritten {
                // Side-by-side diff
                HStack(alignment: .top, spacing: 1) {
                    VStack(alignment: .leading, spacing: 4) {
                        Text("Original")
                            .font(.caption)
                            .foregroundColor(.secondary)
                            .fontWeight(.semibold)
                        ScrollView {
                            diffText(tokens: diffResult.oldTokens, highlightKind: .deleted, color: .red)
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
                            .font(.caption)
                            .foregroundColor(.secondary)
                            .fontWeight(.semibold)
                        ScrollView {
                            diffText(tokens: diffResult.newTokens, highlightKind: .inserted, color: .green)
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
                // Initial state: prompt user to pick a tone
                HStack {
                    Spacer()
                    Text("Select a tone above to rewrite your text")
                        .font(.caption)
                        .foregroundColor(.secondary)
                    Spacer()
                }
                .frame(minHeight: 80)
            }

            // Footer buttons (only show when we have a result)
            if rewritten != nil {
                Divider()
                HStack {
                    Spacer()
                    Button("Cancel", action: onCancel)
                        .keyboardShortcut(.cancelAction)
                    Button("Copy") { onCopy(rewritten!) }
                    Button("Replace") { onReplace(rewritten!) }
                        .keyboardShortcut(.defaultAction)
                }
                .padding(.horizontal, 14)
                .padding(.vertical, 10)
            }
        }
        .background(.ultraThickMaterial, in: RoundedRectangle(cornerRadius: 12, style: .continuous))
    }

    private func selectTone(_ tone: Tone) {
        selectedTone = tone
        rewritten = nil
        errorMessage = nil
        isLoading = true
        onToneSelected(tone)
    }

    /// Called from outside to deliver the rewrite result
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

    private var diffResult: (oldTokens: [DiffToken], newTokens: [DiffToken]) {
        guard let rewritten = rewritten else { return ([], []) }
        return WordDiff.diff(old: original, new: rewritten)
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
