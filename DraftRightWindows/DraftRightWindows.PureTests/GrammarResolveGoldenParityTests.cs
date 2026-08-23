using System.IO;
using System.Runtime.CompilerServices;
using System.Text.Json;
using DraftRightWindows.Models;
using DraftRightWindows.Services;
using Xunit;

namespace DraftRightWindows.PureTests;

/// <summary>
/// Windows grammar resolveRange must match the shared golden vectors (#107, RULE #1).
///
/// <c>parity/grammar-resolve-vectors.json</c> at the repo root is the
/// single source of truth; the macOS (Swift) and Linux (Python) ports assert
/// against the same file, so the three copies of the content-first resolve logic
/// (LLM-offset gotcha, BR#49) cannot drift.
/// </summary>
public class GrammarResolveGoldenParityTests
{
    private static string GoldenPath([CallerFilePath] string thisFile = "")
        => Path.GetFullPath(Path.Combine(
            Path.GetDirectoryName(thisFile)!, "..", "..",
            "parity", "grammar-resolve-vectors.json"));

    [Fact]
    public void EveryCaseMatchesTheSharedVectors()
    {
        using var doc = JsonDocument.Parse(File.ReadAllText(GoldenPath()));
        var cases = doc.RootElement.GetProperty("cases");
        Assert.True(cases.GetArrayLength() > 0, "golden file is empty");

        foreach (var c in cases.EnumerateArray())
        {
            var name = c.GetProperty("name").GetString();
            var issue = new GrammarIssue
            {
                Original = c.GetProperty("original").GetString()!,
                Suggestion = "X",
                Offset = c.GetProperty("offset").GetInt32(),
            };

            var got = GrammarFixer.ResolveRange(issue, c.GetProperty("text").GetString()!);
            var expected = c.GetProperty("expected");

            if (expected.ValueKind == JsonValueKind.Null)
            {
                Assert.False(got.HasValue, $"{name}: expected no match");
            }
            else
            {
                Assert.True(got.HasValue, $"{name}: expected a match");
                Assert.Equal(expected.GetProperty("start").GetInt32(), got!.Value.Start);
                Assert.Equal(expected.GetProperty("length").GetInt32(), got.Value.Length);
            }
        }
    }
}
