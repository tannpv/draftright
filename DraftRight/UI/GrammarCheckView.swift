import SwiftUI
import AppKit

// MARK: - Data models

struct GrammarIssue: Codable, Identifiable {
    let type: String
    let offset: Int
    let length: Int
    let original: String
    let suggestion: String
    let reason: String

    var id: String { "\(offset)-\(length)-\(original)" }

    var color: NSColor {
        switch type {
        case "spelling": return .systemRed
        case "grammar": return .systemOrange
        case "style": return .systemBlue
        default: return .systemGray
        }
    }

    var swiftUIColor: Color {
        switch type {
        case "spelling": return .red
        case "grammar": return .orange
        case "style": return .blue
        default: return .gray
        }
    }

    var label: String {
        switch type {
        case "spelling": return "Spelling"
        case "grammar": return "Grammar"
        case "style": return "Style"
        default: return "Issue"
        }
    }

    var icon: String {
        switch type {
        case "spelling": return "textformat.abc"
        case "grammar": return "text.badge.xmark"
        case "style": return "paintbrush.pointed"
        default: return "exclamationmark.circle"
        }
    }
}

struct GrammarResult: Codable {
    let score: Int
    let issues: [GrammarIssue]
}

// MARK: - Main view

struct GrammarCheckView: View {
    let originalText: String
    let result: GrammarResult
    let onReplace: (String) -> Void
    let onCopy: (String) -> Void

    @State private var currentText: String
    @State private var remainingIssues: [GrammarIssue]

    init(originalText: String, result: GrammarResult, onReplace: @escaping (String) -> Void, onCopy: @escaping (String) -> Void) {
        self.originalText = originalText
        self.result = result
        self.onReplace = onReplace
        self.onCopy = onCopy
        self._currentText = State(initialValue: originalText)
        self._remainingIssues = State(initialValue: result.issues)
    }

    var body: some View {
        VStack(alignment: .leading, spacing: 0) {
            // Score bar
            HStack {
                scoreView
                Spacer()
                Text("\(remainingIssues.count) issue\(remainingIssues.count == 1 ? "" : "s")")
                    .font(.system(size: 11))
                    .foregroundColor(.secondary)
                if !remainingIssues.isEmpty {
                    Button("Fix All") { fixAll() }
                        .font(.system(size: 11))
                        .buttonStyle(.bordered)
                        .controlSize(.small)
                }
            }
            .padding(.horizontal, 12)
            .padding(.vertical, 8)

            Divider().padding(.horizontal, 12)

            if remainingIssues.isEmpty {
                // All fixed — show clean result
                ScrollView {
                    VStack(alignment: .leading, spacing: 8) {
                        HStack(spacing: 4) {
                            Image(systemName: "checkmark.circle.fill")
                                .foregroundColor(.green)
                                .font(.system(size: 12))
                            Text(currentText == originalText ? "Your writing looks great!" : "All issues fixed!")
                                .font(.system(size: 12, weight: .medium))
                                .foregroundColor(.green)
                        }
                        Text(currentText)
                            .font(.system(size: 13))
                            .lineSpacing(3)
                            .textSelection(.enabled)
                    }
                    .padding(12)
                    .frame(maxWidth: .infinity, alignment: .leading)
                }
            } else {
                // Two-section layout
                ScrollView {
                    VStack(alignment: .leading, spacing: 12) {
                        // Section 1: Highlighted text
                        highlightedTextSection

                        Divider()

                        // Section 2: Issue cards
                        issueCardsSection
                    }
                    .padding(12)
                }
            }
        }
    }

    // MARK: - Score badge

    @ViewBuilder
    private var scoreView: some View {
        let color: Color = result.score >= 90 ? .green : result.score >= 70 ? .orange : .red
        HStack(spacing: 4) {
            Image(systemName: result.score >= 90 ? "checkmark.circle.fill" : "exclamationmark.circle.fill")
                .foregroundColor(color)
            Text("\(result.score)/100")
                .font(.system(size: 13, weight: .semibold))
                .foregroundColor(color)
        }
    }

    // MARK: - Section 1: Text with highlighted issues

    private var highlightedTextSection: some View {
        VStack(alignment: .leading, spacing: 4) {
            Text("Your text")
                .font(.system(size: 10, weight: .semibold))
                .foregroundColor(.secondary)
                .textCase(.uppercase)

            Text(buildHighlightedText())
                .font(.system(size: 13))
                .lineSpacing(4)
                .textSelection(.enabled)
                .frame(maxWidth: .infinity, alignment: .leading)
                .padding(10)
                .background(Color.primary.opacity(0.03))
                .cornerRadius(6)
        }
    }

    private func buildHighlightedText() -> AttributedString {
        var attr = AttributedString(currentText)

        for issue in remainingIssues {
            // Content-based resolution (see Fix logic below) — LLM offsets
            // drift, so highlighting by raw offset underlined the wrong words.
            guard let range = Self.resolveRange(of: issue, in: currentText),
                  let attrRange = Range(range, in: attr) else { continue }

            attr[attrRange].backgroundColor = issue.color.withAlphaComponent(0.15)
            attr[attrRange].underlineStyle = .thick
            attr[attrRange].underlineColor = issue.color
        }

        return attr
    }

    // MARK: - Section 2: Issue cards

    private var issueCardsSection: some View {
        VStack(alignment: .leading, spacing: 4) {
            Text("Issues found")
                .font(.system(size: 10, weight: .semibold))
                .foregroundColor(.secondary)
                .textCase(.uppercase)

            ForEach(remainingIssues) { issue in
                issueCard(issue)
            }
        }
    }

    @ViewBuilder
    private func issueCard(_ issue: GrammarIssue) -> some View {
        HStack(alignment: .top, spacing: 10) {
            // Type icon
            Image(systemName: issue.icon)
                .font(.system(size: 12))
                .foregroundColor(issue.swiftUIColor)
                .frame(width: 20, height: 20)
                .background(issue.swiftUIColor.opacity(0.1))
                .cornerRadius(4)

            // Issue details
            VStack(alignment: .leading, spacing: 4) {
                // Type + reason
                HStack(spacing: 4) {
                    Text(issue.label)
                        .font(.system(size: 10, weight: .semibold))
                        .foregroundColor(issue.swiftUIColor)
                    Text("·")
                        .foregroundColor(.secondary)
                    Text(issue.reason)
                        .font(.system(size: 10))
                        .foregroundColor(.secondary)
                        .lineLimit(1)
                }

                // Original → Suggestion
                HStack(spacing: 6) {
                    Text(issue.original)
                        .font(.system(size: 12))
                        .strikethrough(color: issue.swiftUIColor.opacity(0.5))
                        .foregroundColor(.secondary)
                        .padding(.horizontal, 4)
                        .padding(.vertical, 2)
                        .background(issue.swiftUIColor.opacity(0.08))
                        .cornerRadius(3)

                    Image(systemName: "arrow.right")
                        .font(.system(size: 9))
                        .foregroundColor(.secondary)

                    Text(issue.suggestion)
                        .font(.system(size: 12, weight: .medium))
                        .foregroundColor(issue.swiftUIColor)
                        .padding(.horizontal, 4)
                        .padding(.vertical, 2)
                        .background(issue.swiftUIColor.opacity(0.08))
                        .cornerRadius(3)
                }
            }

            Spacer()

            // Fix button
            Button(action: { applyFix(issue) }) {
                Text("Fix")
                    .font(.system(size: 11, weight: .medium))
            }
            .buttonStyle(.borderedProminent)
            .controlSize(.small)
        }
        .padding(8)
        .background(Color.primary.opacity(0.03))
        .cornerRadius(6)
    }

    // MARK: - Fix logic
    //
    // LLM-reported offsets are UNRELIABLE (models count tokens/bytes, drift by
    // ±N) — trusting them spliced suggestions into the middle of words
    // ("showswincorrectlyectly", BR#49). Every range is therefore re-resolved
    // from the issue's `original` CONTENT in the current text at the moment of
    // use; the numeric offset only disambiguates between duplicate occurrences
    // (nearest wins). Remaining issues need no offset bookkeeping after an
    // apply — they re-resolve on their own next use.

    /// Locates `issue.original` in `text`, preferring the occurrence nearest
    /// the LLM-claimed offset. Returns nil when the original string no longer
    /// exists (stale issue — e.g. already fixed by an overlapping apply).
    static func resolveRange(of issue: GrammarIssue, in text: String) -> Range<String.Index>? {
        guard !issue.original.isEmpty else { return nil }
        var candidates: [Range<String.Index>] = []
        var searchFrom = text.startIndex
        while let r = text.range(of: issue.original, range: searchFrom..<text.endIndex) {
            candidates.append(r)
            searchFrom = r.upperBound
        }
        guard !candidates.isEmpty else { return nil }
        let claimed = max(0, min(issue.offset, text.count))
        return candidates.min { a, b in
            abs(text.distance(from: text.startIndex, to: a.lowerBound) - claimed)
                < abs(text.distance(from: text.startIndex, to: b.lowerBound) - claimed)
        }
    }

    private func applyFix(_ issue: GrammarIssue) {
        if let range = Self.resolveRange(of: issue, in: currentText) {
            currentText.replaceSubrange(range, with: issue.suggestion)
        }
        // Applied or stale either way — drop it. Survivors re-resolve from
        // content on their next render/click, so no offset shifting.
        remainingIssues.removeAll { $0.id == issue.id }
    }

    private func fixAll() {
        // Apply one at a time, re-resolving against the evolving text — order
        // no longer matters because ranges come from content, not offsets.
        for issue in remainingIssues {
            if let range = Self.resolveRange(of: issue, in: currentText) {
                currentText.replaceSubrange(range, with: issue.suggestion)
            }
        }
        remainingIssues = []
    }
}
