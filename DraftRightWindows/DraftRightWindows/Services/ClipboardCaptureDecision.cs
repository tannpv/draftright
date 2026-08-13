namespace DraftRightWindows.Services;

// The pure, Win32-free half of ClipboardService: the selection-capture decision
// and its result types. Split into its own file (partial class) so the unit
// tests can compile it in isolation — the headless CI test project source-
// includes THIS file, not the P/Invoke + DRLogger half, so it doesn't drag in
// the WinUI app assembly whose module init hangs test discovery (#80).

/// <summary>
/// Why a <see cref="ClipboardService.GetSelectedTextAsync"/> call did (or did not)
/// yield text. Lets the caller give the user a specific reason instead of silently
/// swallowing the hotkey when nothing comes back.
/// </summary>
public enum SelectionCaptureStatus
{
    /// <summary>Text was captured (either from the simulated copy or a manual-copy fallback).</summary>
    Captured,

    /// <summary>
    /// SendInput was rejected (returned 0) so the simulated Ctrl+C never reached the
    /// target window — UIPI/elevation mismatch, BlockInput, or a different desktop —
    /// and there was no pre-existing clipboard text to fall back to.
    /// </summary>
    SendInputBlocked,

    /// <summary>The copy fired but the clipboard stayed empty — nothing was selected.</summary>
    NoSelection,
}

/// <summary>Result of a selection-capture attempt: the text (if any) plus why.</summary>
public readonly record struct SelectionCapture(string? Text, SelectionCaptureStatus Status);

public sealed partial class ClipboardService
{
    /// <summary>
    /// Decides what a capture attempt yielded, from the clipboard before/after the
    /// simulated copy and how many input events actually fired.
    ///
    /// Pure and Win32-free so it can be unit-tested. The load-bearing rule: when
    /// SendInput was rejected (<paramref name="sentEvents"/> == 0) the simulated
    /// copy never happened, so we fall back to whatever the user had copied by hand.
    /// This restores the "copy the text yourself, then press the hotkey" workaround —
    /// which is otherwise impossible because <see cref="GetSelectedTextAsync"/> clears
    /// the clipboard before copying.
    /// </summary>
    public static SelectionCapture DecideCapture(string? original, string? afterCopy, uint sentEvents)
    {
        if (!string.IsNullOrWhiteSpace(afterCopy))
            return new SelectionCapture(afterCopy, SelectionCaptureStatus.Captured);

        if (sentEvents == 0)
        {
            return !string.IsNullOrWhiteSpace(original)
                ? new SelectionCapture(original, SelectionCaptureStatus.Captured)
                : new SelectionCapture(null, SelectionCaptureStatus.SendInputBlocked);
        }

        return new SelectionCapture(null, SelectionCaptureStatus.NoSelection);
    }
}
