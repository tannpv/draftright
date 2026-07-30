/// How the user produced the text. `.speech` adds a server-side cleanup
/// preamble on the backend. Mirrors the Android `InputKind` enum so both
/// clients send the same `input_kind` wire value.
public enum InputKind: String {
    case typed
    case speech

    /// Stable wire value sent as `input_kind` (omitted for `.typed`).
    public var apiValue: String { rawValue }
}
