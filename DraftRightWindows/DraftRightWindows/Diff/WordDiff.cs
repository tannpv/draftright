// Word-level LCS diff — port of DraftRight/Diff/WordDiff.swift (issue #107).
//
// Pure logic: no WinUI, no WinForms. Covered headlessly by
// DraftRightWindows.PureTests (issue #80).
using System;
using System.Collections.Generic;

namespace DraftRightWindows.Diff;

public enum DiffKind
{
    Equal,
    Deleted,
    Inserted,
}

/// <summary>One run of text plus how it changed between the two sides.</summary>
public readonly record struct DiffToken(string Text, DiffKind Kind);

/// <summary>Before/after word diff driving the side-by-side view.</summary>
public static class WordDiff
{
    /// <summary>
    /// Tokenizes both sides on whitespace (keeping the whitespace as its own
    /// token so rebuilt text is byte-identical), then walks the longest common
    /// subsequence to mark each token equal / deleted / inserted.
    /// </summary>
    public static (IReadOnlyList<DiffToken> OldTokens, IReadOnlyList<DiffToken> NewTokens) Diff(
        string oldText, string newText)
    {
        var oldWords = Tokenize(oldText);
        var newWords = Tokenize(newText);
        var lcs = LongestCommonSubsequence(oldWords, newWords);

        var oldTokens = new List<DiffToken>();
        var newTokens = new List<DiffToken>();

        int oi = 0, ni = 0, li = 0;
        while (oi < oldWords.Count || ni < newWords.Count)
        {
            if (li < lcs.Count)
            {
                while (oi < oldWords.Count && oldWords[oi] != lcs[li])
                {
                    oldTokens.Add(new DiffToken(oldWords[oi], DiffKind.Deleted));
                    oi++;
                }
                while (ni < newWords.Count && newWords[ni] != lcs[li])
                {
                    newTokens.Add(new DiffToken(newWords[ni], DiffKind.Inserted));
                    ni++;
                }
                oldTokens.Add(new DiffToken(lcs[li], DiffKind.Equal));
                newTokens.Add(new DiffToken(lcs[li], DiffKind.Equal));
                oi++;
                ni++;
                li++;
            }
            else
            {
                while (oi < oldWords.Count)
                {
                    oldTokens.Add(new DiffToken(oldWords[oi], DiffKind.Deleted));
                    oi++;
                }
                while (ni < newWords.Count)
                {
                    newTokens.Add(new DiffToken(newWords[ni], DiffKind.Inserted));
                    ni++;
                }
            }
        }

        return (oldTokens, newTokens);
    }

    /// <summary>
    /// Splits into word tokens, emitting each whitespace character as its own
    /// token so the original spacing survives a round trip.
    /// </summary>
    private static List<string> Tokenize(string text)
    {
        var tokens = new List<string>();
        var current = new System.Text.StringBuilder();
        foreach (var ch in text)
        {
            if (char.IsWhiteSpace(ch))
            {
                if (current.Length > 0)
                {
                    tokens.Add(current.ToString());
                    current.Clear();
                }
                tokens.Add(ch.ToString());
            }
            else
            {
                current.Append(ch);
            }
        }
        if (current.Length > 0) tokens.Add(current.ToString());
        return tokens;
    }

    private static List<string> LongestCommonSubsequence(List<string> a, List<string> b)
    {
        int m = a.Count, n = b.Count;

        // Guard the empty case explicitly. The Swift original writes
        // `for i in 1...m`, which traps at runtime when m or n is 0 (verified:
        // SIGTRAP) — so diffing against an empty rewrite crashes on macOS.
        // Returning early here is both correct and crash-free.
        if (m == 0 || n == 0) return new List<string>();

        var dp = new int[m + 1, n + 1];
        for (var i = 1; i <= m; i++)
        {
            for (var j = 1; j <= n; j++)
            {
                dp[i, j] = a[i - 1] == b[j - 1]
                    ? dp[i - 1, j - 1] + 1
                    : Math.Max(dp[i - 1, j], dp[i, j - 1]);
            }
        }

        var result = new List<string>();
        int x = m, y = n;
        while (x > 0 && y > 0)
        {
            if (a[x - 1] == b[y - 1])
            {
                result.Add(a[x - 1]);
                x--;
                y--;
            }
            else if (dp[x - 1, y] > dp[x, y - 1]) x--;
            else y--;
        }
        result.Reverse();
        return result;
    }
}
