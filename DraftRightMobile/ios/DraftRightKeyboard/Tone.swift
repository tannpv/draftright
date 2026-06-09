import UIKit

enum Tone: String, CaseIterable {
    case simple
    case natural
    case polished
    case concise
    case technical
    case claude
    // Explicit raw value so apiValue stays rawValue — backend expects
    // "grammar_check", not the camelCase case name.
    case grammarCheck = "grammar_check"
    case translate

    var displayName: String {
        switch self {
        case .simple: return "Simple"
        case .natural: return "Natural"
        case .polished: return "Polished"
        case .concise: return "Concise"
        case .technical: return "Technical"
        case .claude: return "Claude Style"
        case .grammarCheck: return "Grammar Check"
        case .translate: return "Translate"
        }
    }

    var iconName: String {
        switch self {
        case .simple: return "textformat.size"
        case .natural: return "bubble.left"
        case .polished: return "sparkles"
        case .concise: return "arrow.down.right.and.arrow.up.left"
        case .technical: return "wrench.and.screwdriver"
        case .claude: return "brain"
        case .grammarCheck: return "text.badge.checkmark"
        case .translate: return "globe"
        }
    }

    var apiValue: String { rawValue }

    func systemPrompt(targetLanguage: String = "English") -> String {
        switch self {
        case .simple:
            return "Rewrite the following text using simple, easy-to-understand language. Use short sentences and common words. Preserve the original meaning. Return only the rewritten text, no explanations."
        case .natural:
            return "Rewrite the following text to sound more natural and conversational, as if spoken by a real person. Remove awkward phrasing and make it flow smoothly. Preserve the original meaning. Return only the rewritten text, no explanations."
        case .polished:
            return "Rewrite the following text to be more polished and professional. Improve grammar, word choice, and sentence structure for a refined, workplace-appropriate tone. Preserve the original meaning. Return only the rewritten text, no explanations."
        case .concise:
            return "Rewrite the following text to be as concise as possible. Remove unnecessary words, redundancy, and filler while preserving the key meaning. Return only the rewritten text, no explanations."
        case .technical:
            return "Rewrite the following text in a technical specification style. Use precise, unambiguous language suitable for documentation, specs, or technical communication. Preserve the original meaning. Return only the rewritten text, no explanations."
        case .claude:
            return "Rewrite the following text in a clear, thoughtful, and well-structured style. Be direct but warm — every sentence should carry weight. Use good paragraph breaks and logical flow. Acknowledge nuance where relevant without over-hedging. Sound naturally confident and approachable, not formal or stiff. Preserve the original meaning. Return only the rewritten text, no explanations."
        case .grammarCheck:
            return "Analyze the given text for grammar, spelling, and style issues. Return a JSON object with a \"score\" (0-100) and an \"issues\" array. Each issue has \"type\", \"offset\", \"length\", \"original\", \"suggestion\", and \"reason\". Return ONLY JSON."
        case .translate:
            return "Translate the following text into \(targetLanguage). If the text is already in \(targetLanguage), translate it into English instead. Preserve the original meaning and tone. Return only the translated text, no explanations."
        }
    }
}
