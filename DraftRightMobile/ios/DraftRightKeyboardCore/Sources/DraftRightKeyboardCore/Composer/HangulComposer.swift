import Foundation

/// Composer for Korean: accumulates typed jamo and exposes the live Hangul
/// assembly as the composing buffer. Deterministic composition, no candidate
/// bar (the whole of it is `HangulAssembler`). Reuses `BufferingComposer`.
public final class HangulComposer: BufferingComposer {
    public override func transform(_ raw: String) -> String { HangulAssembler.assemble(raw) }
}
