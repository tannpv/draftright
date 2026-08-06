using System.Collections.Generic;
using DraftRightWindows.Models;
using DraftRightWindows.Services;
using Xunit;

namespace DraftRightWindows.PureTests;

/// <summary>
/// Grammar fixes are resolved by CONTENT, never by the LLM's offset (issue
/// #107). Trusting offsets produced "showswincorrectlyectly" (BR#49) — these
/// tests pin the behaviour that prevents it.
/// </summary>
public class GrammarFixerTests
{
    private static GrammarIssue Issue(string original, string suggestion, int offset = 0,
        string type = "grammar") => new()
    {
        Original = original,
        Suggestion = suggestion,
        Offset = offset,
        Length = original.Length,
        Type = type,
        Reason = "test",
    };

    [Fact]
    public void ResolvesByContent_WhenTheOffsetIsWrong()
    {
        const string text = "This sentence shows incorrectly.";
        // Offset deliberately nonsense — content must still win.
        var issue = Issue("incorrectly", "correctly", offset: 9999);

        var fixedText = GrammarFixer.ApplyFix(text, issue);

        Assert.Equal("This sentence shows correctly.", fixedText);
        // The exact splice-into-the-middle-of-a-word failure from BR#49.
        Assert.DoesNotContain("showswincorrectlyectly", fixedText);
    }

    [Fact]
    public void ResolvesByContent_WhenTheOffsetIsZero()
    {
        const string text = "alpha beta gamma";
        var fixedText = GrammarFixer.ApplyFix(text, Issue("gamma", "delta", offset: 0));
        Assert.Equal("alpha beta delta", fixedText);
    }

    [Fact]
    public void OffsetDisambiguatesDuplicates_NearestWins()
    {
        //           0         1         2
        //           0123456789012345678901234
        const string text = "cat dog cat dog cat";
        // Three "cat"s at 0, 8, 16. Claimed offset 15 → the one at 16.
        var fixedText = GrammarFixer.ApplyFix(text, Issue("cat", "fox", offset: 15));
        Assert.Equal("cat dog cat dog fox", fixedText);
    }

    [Fact]
    public void OffsetDisambiguatesDuplicates_FirstOccurrence()
    {
        const string text = "cat dog cat dog cat";
        var fixedText = GrammarFixer.ApplyFix(text, Issue("cat", "fox", offset: 0));
        Assert.Equal("fox dog cat dog cat", fixedText);
    }

    [Fact]
    public void TieOnDistance_PrefersTheEarlierOccurrence()
    {
        // "ab" at 0 and 4; claimed 2 is equidistant. Stable choice = earliest,
        // matching the macOS `min`.
        const string text = "ab..ab";
        var fixedText = GrammarFixer.ApplyFix(text, Issue("ab", "XY", offset: 2));
        Assert.Equal("XY..ab", fixedText);
    }

    [Fact]
    public void StaleIssue_LeavesTextUnchanged()
    {
        const string text = "already fixed";
        var fixedText = GrammarFixer.ApplyFix(text, Issue("nonexistent", "whatever"));
        Assert.Equal(text, fixedText);
    }

    [Fact]
    public void EmptyOriginal_IsNeverResolved()
    {
        Assert.Null(GrammarFixer.ResolveRange(Issue("", "x"), "some text"));
    }

    [Fact]
    public void OffsetBeyondTextLength_IsClamped_NotCrashing()
    {
        const string text = "short";
        var fixedText = GrammarFixer.ApplyFix(text, Issue("short", "long", offset: 100000));
        Assert.Equal("long", fixedText);
    }

    [Fact]
    public void FixAll_AppliesEveryIssue_RegardlessOfOrder()
    {
        const string text = "teh quik brown fox";
        var issues = new List<GrammarIssue>
        {
            Issue("quik", "quick", offset: 4),
            Issue("teh", "the", offset: 0),
        };
        Assert.Equal("the quick brown fox", GrammarFixer.FixAll(text, issues));
    }

    [Fact]
    public void FixAll_IsOrderIndependent_EvenWhenEarlierFixesShiftLaterOnes()
    {
        // A lengthening fix early in the string invalidates every later offset.
        // Content resolution is what makes this safe.
        const string text = "i beleive teh answer";
        var forward = new List<GrammarIssue>
        {
            Issue("beleive", "believe strongly in", offset: 2),
            Issue("teh", "the", offset: 10),
        };
        var reversed = new List<GrammarIssue>
        {
            Issue("teh", "the", offset: 10),
            Issue("beleive", "believe strongly in", offset: 2),
        };

        Assert.Equal("i believe strongly in the answer", GrammarFixer.FixAll(text, forward));
        Assert.Equal(GrammarFixer.FixAll(text, forward), GrammarFixer.FixAll(text, reversed));
    }

    [Fact]
    public void FixAll_SkipsIssuesRemovedByAnOverlappingFix()
    {
        const string text = "very very bad";
        var issues = new List<GrammarIssue>
        {
            Issue("very very bad", "excellent"),
            Issue("bad", "good"),   // its anchor is gone after the first fix
        };
        Assert.Equal("excellent", GrammarFixer.FixAll(text, issues));
    }

    [Fact]
    public void RemainingIssues_DropsTheOnesNoLongerPresent()
    {
        const string text = "the quick fox";
        var issues = new List<GrammarIssue>
        {
            Issue("quick", "slow"),
            Issue("missing", "x"),
        };
        var remaining = GrammarFixer.RemainingIssues(text, issues);
        Assert.Single(remaining);
        Assert.Equal("quick", remaining[0].Original);
    }

    [Fact]
    public void UnicodeSuggestion_AppliesCorrectly()
    {
        const string text = "toi dang viet ma";
        var fixedText = GrammarFixer.ApplyFix(text, Issue("toi", "tôi"));
        Assert.Equal("tôi dang viet ma", fixedText);
    }

    // ── Issue type mapping ──

    [Theory]
    [InlineData("spelling", GrammarIssueType.Spelling)]
    [InlineData("SPELLING", GrammarIssueType.Spelling)]
    [InlineData("grammar", GrammarIssueType.Grammar)]
    [InlineData("style", GrammarIssueType.Style)]
    [InlineData("something-new", GrammarIssueType.Other)]
    [InlineData(null, GrammarIssueType.Other)]
    public void IssueType_MapsFromWire(string? wire, GrammarIssueType expected)
    {
        Assert.Equal(expected, GrammarIssueTypeExtensions.FromWire(wire));
    }

    [Fact]
    public void IssueType_RoundTripsThroughWireValue()
    {
        foreach (var t in new[] { GrammarIssueType.Spelling, GrammarIssueType.Grammar, GrammarIssueType.Style })
        {
            Assert.Equal(t, GrammarIssueTypeExtensions.FromWire(t.WireValue()));
        }
    }

    [Fact]
    public void IssueType_ColoursMatchMacOs()
    {
        Assert.Equal("#ef4444", GrammarIssueType.Spelling.HexColor());  // red
        Assert.Equal("#f59e0b", GrammarIssueType.Grammar.HexColor());   // orange
        Assert.Equal("#5d87ff", GrammarIssueType.Style.HexColor());     // blue
    }
}
