import Foundation

public struct KeyDef: Equatable {
    public let label: String
    public let code: Int
    public let widthWeight: CGFloat

    public init(_ label: String, _ code: Int, widthWeight: CGFloat = 1.0) {
        self.label = label
        self.code = code
        self.widthWeight = widthWeight
    }
}

public protocol LanguagePack {
    var id: String { get }
    var displayName: String { get }
    var locale: Locale { get }
    var alphaRows: [[KeyDef]] { get }
    var symbols1Rows: [[KeyDef]] { get }
    var symbols2Rows: [[KeyDef]] { get }
    var longPressAccents: [Character: [Character]] { get }
    func makeComposer() -> Composer?
}

public extension LanguagePack {
    func makeComposer() -> Composer? { nil }
}
