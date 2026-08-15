using DraftRightWindows.Services;
using Xunit;

namespace DraftRightWindows.PureTests;

/// <summary>
/// The show-pencil decision — drag only, mirroring macOS (DraftRight #180).
/// </summary>
public class PencilTriggerDecisionTests
{
    [Fact]
    public void ShowsOnDrag() =>
        Assert.True(PencilTriggerDecision.ShouldShowPencil(wasDragging: true));

    [Fact]
    public void HiddenWithoutDrag() =>
        Assert.False(PencilTriggerDecision.ShouldShowPencil(wasDragging: false));
}
