import Foundation

public final class LanguageRegistry {
    public let all: [LanguagePack]

    public init(packs: [LanguagePack]) {
        precondition(!packs.isEmpty, "LanguageRegistry needs at least one LanguagePack")
        self.all = packs
    }

    public func byId(_ id: String) -> LanguagePack {
        guard let p = all.first(where: { $0.id == id }) else {
            fatalError("Unknown LanguagePack id: \(id)")
        }
        return p
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
