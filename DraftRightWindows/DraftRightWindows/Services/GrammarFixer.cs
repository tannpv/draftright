// Applying grammar suggestions to text — port of the macOS resolveRange /
// applyFix / fixAll logic in DraftRight/UI/GrammarCheckView.swift (issue #107).
//
// Pure logic: no UI, no network. Covered headlessly by
// DraftRightWindows.PureTests (issue #80).
using System;
using System.Collections.Generic;
using DraftRightWindows.Models;

namespace DraftRightWindows.Services;

/// <summary>
/// Resolves and applies grammar fixes.
///
/// LLM-reported offsets are UNRELIABLE — models count tokens or bytes and
/// drift by several characters. Trusting them spliced suggestions into the
/// middle of words ("showswincorrectlyectly", BR#49). Every range is therefore
/// re-resolved from the issue's <c>Original</c> CONTENT against the current
/// text at the moment of use; the numeric offset only disambiguates between
/// duplicate occurrences, where the nearest one wins.
///
/// See memory feedback_llm_offsets_unreliable and the macOS fix history.
/// </summary>
public static class GrammarFixer
{
    /// <summary>
    /// Locates <paramref name="issue"/>'s original text inside
    /// <paramref name="text"/>, preferring the occurrence nearest the
    /// LLM-claimed offset. Null when the original no longer exists — a stale
    /// issue, e.g. already removed by an overlapping fix.
    /// </summary>
    public static (int Start, int Length)? ResolveRange(GrammarIssue issue, string text)
    {
        if (issue == null || string.IsNullOrEmpty(issue.Original) || text == null) return null;

        var candidates = new List<int>();
        var from = 0;
        while (from <= text.Length - issue.Original.Length)
        {
            var at = text.IndexOf(issue.Original, from, StringComparison.Ordinal);
            if (at < 0) break;
            candidates.Add(at);
            // Advance past this match — overlapping matches would produce
            // ranges that cannot both be applied.
            from = at + issue.Original.Length;
        }
        if (candidates.Count == 0) return null;

        var claimed = Math.Max(0, Math.Min(issue.Offset, text.Length));
        var best = candidates[0];
        foreach (var c in candidates)
        {
            // Strictly-less keeps the earliest occurrence on a tie, matching
            // the macOS `min` which is stable.
            if (Math.Abs(c - claimed) < Math.Abs(best - claimed)) best = c;
        }
        return (best, issue.Original.Length);
    }

    /// <summary>
    /// Applies one issue's suggestion. Returns the text unchanged when the
    /// issue is stale (its original is no longer present).
    /// </summary>
    public static string ApplyFix(string text, GrammarIssue issue)
    {
        var range = ResolveRange(issue, text);
        if (range == null) return text;
        var (start, length) = range.Value;
        return string.Concat(text.AsSpan(0, start), issue.Suggestion, text.AsSpan(start + length));
    }

    /// <summary>
    /// Applies every issue, re-resolving against the evolving text after each
    /// one. Order does not matter, because ranges come from content rather than
    /// offsets — which is exactly what the offset-based version got wrong.
    /// </summary>
    public static string FixAll(string text, IEnumerable<GrammarIssue> issues)
    {
        if (issues == null) return text;
        var result = text;
        foreach (var issue in issues)
        {
            result = ApplyFix(result, issue);
        }
        return result;
    }

    /// <summary>
    /// Issues still applicable to <paramref name="text"/>. Used to drop stale
    /// entries from the UI after a fix without any offset bookkeeping.
    /// </summary>
    public static List<GrammarIssue> RemainingIssues(string text, IEnumerable<GrammarIssue> issues)
    {
        var remaining = new List<GrammarIssue>();
        if (issues == null) return remaining;
        foreach (var issue in issues)
        {
            if (ResolveRange(issue, text) != null) remaining.Add(issue);
        }
        return remaining;
    }
}
