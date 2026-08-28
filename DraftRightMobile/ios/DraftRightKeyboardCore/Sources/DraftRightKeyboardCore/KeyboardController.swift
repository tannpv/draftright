import Foundation

public enum KeystrokeOutcome: Equatable {
    case commit(String)
    case composing(String)
    case deleteOne
    case noChange
}

public final class KeyboardController {
    private let registry: LanguageRegistry
    /// When true, Japanese uses the flick kana composer instead of rōmaji (#212).
    private let jpFlick: Bool
    public private(set) var enabled: [LanguagePack]
    public private(set) var current: LanguagePack
    public private(set) var composer: Composer?

    public init(registry: LanguageRegistry, enabledIds: [String], activeId: String, jpFlick: Bool = false) {
        self.registry = registry
        self.jpFlick = jpFlick
        let filtered = registry.all.filter { enabledIds.contains($0.id) }
        self.enabled = filtered.isEmpty ? [registry.byIdOrDefault("en")] : filtered
        self.current = self.enabled.first(where: { $0.id == activeId }) ?? self.enabled[0]
        self.composer = Self.makeComposer(self.current, jpFlick: jpFlick)
    }

    /// Japanese + flick mode gets the kana passthrough composer; every other case
    /// keeps the pack's own composer, so non-flick behaviour is unchanged (#212).
    private static func makeComposer(_ pack: LanguagePack, jpFlick: Bool) -> Composer? {
        (pack.id == "ja" && jpFlick) ? KanaComposer() : pack.makeComposer()
    }

    public func cycleLanguage(reverse: Bool = false) {
        guard enabled.count > 1 else { return }
        let idx = enabled.firstIndex(where: { $0.id == current.id }) ?? 0
        let step = reverse ? -1 : 1
        let rawIdx = idx + step
        let nextIdx = ((rawIdx % enabled.count) + enabled.count) % enabled.count
        composer?.reset()
        current = enabled[nextIdx]
        composer = Self.makeComposer(current, jpFlick: jpFlick)
    }

    public func setActive(id: String) {
        guard let target = enabled.first(where: { $0.id == id }) else { return }
        guard target.id != current.id else { return }
        composer?.reset()
        current = target
        composer = Self.makeComposer(current, jpFlick: jpFlick)
    }

    public func onKey(_ char: Character) -> KeystrokeOutcome {
        guard let c = composer else { return .commit(String(char)) }
        switch c.onKey(char) {
        case .commit(let text):    return .commit(text)
        case .composing(let text): return .composing(text)
        case .passThrough:         return .commit(String(char))
        case .consumed:            return .noChange
        }
    }

    public func onBackspace() -> KeystrokeOutcome {
        guard let c = composer else { return .deleteOne }
        switch c.onBackspace() {
        case .composing(let text): return .composing(text)
        case .consumed:            return .noChange
        case .passThrough:         return .deleteOne
        case .commit(let text):    return .commit(text)
        }
    }
}
