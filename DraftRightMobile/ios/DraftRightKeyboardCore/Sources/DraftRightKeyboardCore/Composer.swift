import Foundation

public enum ComposeResult: Equatable {
    case passThrough
    case commit(String)
    case composing(String)
    case consumed
}

public protocol Composer: AnyObject {
    func onKey(_ char: Character) -> ComposeResult
    func onBackspace() -> ComposeResult
    func reset()
    func currentComposingText() -> String
}
