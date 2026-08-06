using System.Linq;
using DraftRightWindows.Diff;
using Xunit;

namespace DraftRightWindows.PureTests;

/// <summary>
/// Word-level LCS diff (issue #107), ported from macOS WordDiff.swift.
/// </summary>
public class WordDiffTests
{
    private static string Rebuild(System.Collections.Generic.IReadOnlyList<DiffToken> tokens)
        => string.Concat(tokens.Select(t => t.Text));

    [Fact]
    public void IdenticalText_IsAllEqual()
    {
        var (oldT, newT) = WordDiff.Diff("the quick fox", "the quick fox");
        Assert.All(oldT, t => Assert.Equal(DiffKind.Equal, t.Kind));
        Assert.All(newT, t => Assert.Equal(DiffKind.Equal, t.Kind));
    }

    [Fact]
    public void ReplacedWord_IsDeletedOnLeft_InsertedOnRight()
    {
        var (oldT, newT) = WordDiff.Diff("the quick fox", "the slow fox");

        Assert.Contains(oldT, t => t.Text == "quick" && t.Kind == DiffKind.Deleted);
        Assert.Contains(newT, t => t.Text == "slow" && t.Kind == DiffKind.Inserted);
        Assert.Contains(oldT, t => t.Text == "the" && t.Kind == DiffKind.Equal);
        Assert.Contains(newT, t => t.Text == "fox" && t.Kind == DiffKind.Equal);
    }

    [Fact]
    public void PureInsertion_LeavesLeftUntouched()
    {
        var (oldT, newT) = WordDiff.Diff("hello world", "hello brave world");
        Assert.DoesNotContain(oldT, t => t.Kind == DiffKind.Deleted);
        Assert.Contains(newT, t => t.Text == "brave" && t.Kind == DiffKind.Inserted);
    }

    [Fact]
    public void PureDeletion_LeavesRightUntouched()
    {
        var (oldT, newT) = WordDiff.Diff("hello brave world", "hello world");
        Assert.Contains(oldT, t => t.Text == "brave" && t.Kind == DiffKind.Deleted);
        Assert.DoesNotContain(newT, t => t.Kind == DiffKind.Inserted);
    }

    [Fact]
    public void TokensRebuildTheOriginalStrings_Exactly()
    {
        // Whitespace is emitted as its own token precisely so this holds —
        // the diff view renders these runs directly, so any loss here shows up
        // as mangled spacing on screen.
        const string a = "the  quick\nbrown fox";
        const string b = "the slow\nbrown  fox";
        var (oldT, newT) = WordDiff.Diff(a, b);
        Assert.Equal(a, Rebuild(oldT));
        Assert.Equal(b, Rebuild(newT));
    }

    [Theory]
    [InlineData("", "hello")]
    [InlineData("hello", "")]
    [InlineData("", "")]
    public void EmptySide_DoesNotThrow(string oldText, string newText)
    {
        // The Swift original writes `for i in 1...m`, which TRAPS when either
        // side has zero tokens (verified: SIGTRAP). Diffing against an empty
        // rewrite therefore crashes on macOS. This port must not.
        var (oldT, newT) = WordDiff.Diff(oldText, newText);
        Assert.Equal(oldText, Rebuild(oldT));
        Assert.Equal(newText, Rebuild(newT));
    }

    [Fact]
    public void EmptyOldSide_MarksEverythingInserted()
    {
        var (oldT, newT) = WordDiff.Diff("", "brand new");
        Assert.Empty(oldT);
        Assert.All(newT, t => Assert.Equal(DiffKind.Inserted, t.Kind));
    }

    [Fact]
    public void CompletelyDifferentText_SharesNothing()
    {
        var (oldT, newT) = WordDiff.Diff("aaa", "bbb");
        Assert.All(oldT, t => Assert.Equal(DiffKind.Deleted, t.Kind));
        Assert.All(newT, t => Assert.Equal(DiffKind.Inserted, t.Kind));
    }

    [Fact]
    public void RepeatedWords_AreHandledWithoutLosingContent()
    {
        const string a = "a a a b";
        const string b = "a b b";
        var (oldT, newT) = WordDiff.Diff(a, b);
        Assert.Equal(a, Rebuild(oldT));
        Assert.Equal(b, Rebuild(newT));
    }

    [Fact]
    public void UnicodeText_RoundTrips()
    {
        // Vietnamese is a first-class case for this product.
        const string a = "tôi đang viết mã";
        const string b = "tôi đang viết code";
        var (oldT, newT) = WordDiff.Diff(a, b);
        Assert.Equal(a, Rebuild(oldT));
        Assert.Equal(b, Rebuild(newT));
        Assert.Contains(oldT, t => t.Text == "mã" && t.Kind == DiffKind.Deleted);
        Assert.Contains(newT, t => t.Text == "code" && t.Kind == DiffKind.Inserted);
    }
}
