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

/// Unicode scalar value used as the key code for a literal character key.
/// Shared by every LanguagePack so the layout files don't re-spell
/// `Int(Character("x").unicodeScalars.first!.value)` at each call site.
func keyCode(_ label: String) -> Int {
    Int(label.unicodeScalars.first!.value)
}

/// Builds character KeyDefs from single-character labels. Shared across
/// language packs so each one no longer redeclares its own `chars` helper.
func chars(_ labels: String...) -> [KeyDef] {
    labels.map { KeyDef($0, keyCode($0)) }
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
