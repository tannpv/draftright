using System;
using System.Collections.Generic;
using System.IO;
using System.Linq;
using System.Runtime.CompilerServices;
using System.Text.Json;
using DraftRightWindows.Diff;
using Xunit;

namespace DraftRightWindows.PureTests;

/// <summary>
/// Windows WordDiff must match the shared golden vectors (#107, RULE #1).
///
/// <c>shared/diff_golden_vectors.json</c> at the repo root is the single source
/// of truth; the macOS (Swift) and Linux (Python) ports assert against the same
/// file, so the three LCS word-diff implementations cannot drift apart.
/// </summary>
public class WordDiffGoldenParityTests
{
    // The source file sits at <repo>/DraftRightWindows/DraftRightWindows.PureTests/,
    // so the golden is two directories up. CallerFilePath resolves against the
    // repo checkout the tests were compiled from.
    private static string GoldenPath([CallerFilePath] string thisFile = "")
        => Path.GetFullPath(Path.Combine(
            Path.GetDirectoryName(thisFile)!, "..", "..", "shared", "diff_golden_vectors.json"));

    private static List<string> Encode(IEnumerable<DiffToken> tokens)
        => tokens.Select(t => $"{t.Text}\t{t.Kind.ToString().ToLowerInvariant()}").ToList();

    private static List<string> Expected(JsonElement tokenPairs)
        => tokenPairs.EnumerateArray()
            .Select(p => $"{p[0].GetString()}\t{p[1].GetString()}")
            .ToList();

    [Fact]
    public void EveryCaseMatchesTheSharedVectors()
    {
        using var doc = JsonDocument.Parse(File.ReadAllText(GoldenPath()));
        var cases = doc.RootElement.GetProperty("cases");
        Assert.True(cases.GetArrayLength() > 0, "golden file is empty");

        foreach (var c in cases.EnumerateArray())
        {
            var name = c.GetProperty("name").GetString();
            var (oldT, newT) = WordDiff.Diff(
                c.GetProperty("old").GetString()!, c.GetProperty("new").GetString()!);

            Assert.Equal(Expected(c.GetProperty("old_tokens")), Encode(oldT));
            Assert.Equal(Expected(c.GetProperty("new_tokens")), Encode(newT));
        }
    }
}
