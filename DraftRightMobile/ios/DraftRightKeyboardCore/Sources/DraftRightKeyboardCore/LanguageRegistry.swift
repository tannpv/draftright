import Foundation

public final class LanguageRegistry {
    public let all: [LanguagePack]

    public init(packs: [LanguagePack]) {
        precondition(!packs.isEmpty, "LanguageRegistry needs at least one LanguagePack")
        self.all = packs
    }

    /// Look up by id, falling back to the default pack. Never crashes — a
    /// stale/unknown id (e.g. left in shared settings by an older build)
    /// must degrade to the default language, not take down the keyboard.
    public func byId(_ id: String) -> LanguagePack {
        return byIdOrDefault(id)
    }

    public func byIdOrDefault(_ id: String) -> LanguagePack {
        return all.first(where: { $0.id == id }) ?? all[0]
    }

    public func next(currentId: String) -> LanguagePack {
        guard let idx = all.firstIndex(where: { $0.id == currentId }) else {
            return all[0]
        }
        return all[(idx + 1) % all.count]
    }
}
