using DraftRightWindows.Services;
using Xunit;

namespace DraftRightWindows.Tests;

/// <summary>
/// Covers <see cref="ClipboardService.DecideCapture"/> — the pure classifier that
/// turns (pre-copy clipboard, post-copy clipboard, SendInput event count) into a
/// <see cref="SelectionCapture"/>. This is the logic behind BR#50: when nothing is
/// captured, the caller must know *why* so it can tell the user instead of silently
/// swallowing the hotkey.
/// </summary>
public class ClipboardCaptureTests
{
    [Fact]
    public void CopyYieldedText_IsCaptured()
    {
        var r = ClipboardService.DecideCapture(original: "old", afterCopy: "selected text", sentEvents: 4);
        Assert.Equal(SelectionCaptureStatus.Captured, r.Status);
        Assert.Equal("selected text", r.Text);
    }

    [Fact]
    public void CopyYieldedText_PrefersFreshOverOriginal()
    {
        // Even when SendInput reported blocked, a non-empty post-copy read wins.
        var r = ClipboardService.DecideCapture(original: "old", afterCopy: "fresh", sentEvents: 0);
        Assert.Equal(SelectionCaptureStatus.Captured, r.Status);
        Assert.Equal("fresh", r.Text);
    }

    [Fact]
    public void Blocked_WithPreCopiedText_FallsBackToOriginal()
    {
        // SendInput rejected (sent==0) and the copy produced nothing, but the user
        // had manually copied text — use it so "copy then hotkey" works.
        var r = ClipboardService.DecideCapture(original: "manually copied", afterCopy: null, sentEvents: 0);
        Assert.Equal(SelectionCaptureStatus.Captured, r.Status);
        Assert.Equal("manually copied", r.Text);
    }

    [Fact]
    public void Blocked_WithNoClipboard_ReportsSendInputBlocked()
    {
        var r = ClipboardService.DecideCapture(original: null, afterCopy: null, sentEvents: 0);
        Assert.Equal(SelectionCaptureStatus.SendInputBlocked, r.Status);
        Assert.Null(r.Text);
    }

    [Fact]
    public void Blocked_WithWhitespaceClipboard_ReportsSendInputBlocked()
    {
        var r = ClipboardService.DecideCapture(original: "   ", afterCopy: "", sentEvents: 0);
        Assert.Equal(SelectionCaptureStatus.SendInputBlocked, r.Status);
        Assert.Null(r.Text);
    }

    [Fact]
    public void CopyFiredButEmpty_ReportsNoSelection()
    {
        // SendInput succeeded (sent>0) but the clipboard stayed empty — nothing selected.
        var r = ClipboardService.DecideCapture(original: "old", afterCopy: null, sentEvents: 4);
        Assert.Equal(SelectionCaptureStatus.NoSelection, r.Status);
        Assert.Null(r.Text);
    }

    [Fact]
    public void CopyFiredButWhitespace_ReportsNoSelection()
    {
        var r = ClipboardService.DecideCapture(original: "old", afterCopy: "  \r\n ", sentEvents: 4);
        Assert.Equal(SelectionCaptureStatus.NoSelection, r.Status);
        Assert.Null(r.Text);
    }
}
