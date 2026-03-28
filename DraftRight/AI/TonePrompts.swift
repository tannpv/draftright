import Foundation

enum Tone: String, CaseIterable, Identifiable {
    case simple
    case natural
    case polished
    case concise
    case technical

    var id: String { rawValue }

    var displayName: String {
        switch self {
        case .simple: return "Simple"
        case .natural: return "More Natural"
        case .polished: return "More Polished"
        case .concise: return "Concise"
        case .technical: return "Technical"
        }
    }

    var systemPrompt: String {
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
        }
    }
}
