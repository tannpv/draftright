namespace DraftRightWindows.Services;

/// <summary>
/// Pure decision for whether a mouse-up should surface the pencil button.
/// Mirrors the macOS <c>SelectionMonitor.shouldShowPencil</c> exactly: only a
/// <b>drag</b> (highlighting text by dragging) triggers it. A click — including
/// a double/triple click that selects a word or line — is deliberately not a
/// trigger; it fires while merely reading (DraftRight #180). Kept pure and
/// static so it is unit-testable without any Win32 / mouse-hook plumbing.
/// </summary>
public static class PencilTriggerDecision
{
    public static bool ShouldShowPencil(bool wasDragging) => wasDragging;
}
