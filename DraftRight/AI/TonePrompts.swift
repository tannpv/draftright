import Foundation

enum Tone: String, CaseIterable, Identifiable {
    case professional
    case casual
    case grammar
    case shorter
    case longer

    var id: String { rawValue }

    var displayName: String {
        switch self {
        case .professional: return "Professional"
        case .casual: return "Casual"
        case .grammar: return "Fix Grammar"
        case .shorter: return "Shorter"
        case .longer: return "Longer"
        }
    }

    var systemPrompt: String {
        switch self {
        case .professional:
            return "Rewrite the following text to be professional, clear, and workplace-appropriate. Preserve the original meaning. Return only the rewritten text, no explanations."
        case .casual:
            return "Rewrite the following text to be friendly and conversational. Preserve the original meaning. Return only the rewritten text, no explanations."
        case .grammar:
            return "Fix grammar, spelling, and punctuation errors in the following text. Do not change the tone or style. Return only the corrected text, no explanations."
        case .shorter:
            return "Condense the following text while preserving the key meaning. Return only the shortened text, no explanations."
        case .longer:
            return "Expand the following text with more detail and context while keeping the same tone. Return only the expanded text, no explanations."
        }
    }
}
