import SwiftUI

struct DiffView: View {
    let tone: Tone
    let original: String
    let rewritten: String
    let onReplace: () -> Void
    let onCopy: () -> Void
    let onCancel: () -> Void
    let onRetry: (() -> Void)?

    @State private var errorMessage: String?

    var body: some View {
        VStack(alignment: .leading, spacing: 10) {
            // Header
            HStack {
                Image(systemName: "pencil.and.outline")
                    .foregroundColor(.accentColor)
                Text("DraftRight")
                    .font(.headline)
                Text("— \(tone.displayName)")
                    .font(.subheadline)
                    .foregroundColor(.secondary)
                Spacer()
                Button(action: onCancel) {
                    Image(systemName: "xmark")
                        .foregroundColor(.secondary)
                }
                .buttonStyle(.borderless)
            }

            // Side-by-side diff
            HStack(alignment: .top, spacing: 1) {
                // Original (left)
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

                // Rewritten (right)
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

            // Buttons
            HStack {
                Spacer()
                Button("Cancel", action: onCancel)
                    .keyboardShortcut(.cancelAction)
                Button("Copy", action: onCopy)
                Button("Replace", action: onReplace)
                    .keyboardShortcut(.defaultAction)
            }
        }
        .padding(14)
        .background(.ultraThickMaterial, in: RoundedRectangle(cornerRadius: 12, style: .continuous))
    }

    private var diffResult: (oldTokens: [DiffToken], newTokens: [DiffToken]) {
        WordDiff.diff(old: original, new: rewritten)
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
}
