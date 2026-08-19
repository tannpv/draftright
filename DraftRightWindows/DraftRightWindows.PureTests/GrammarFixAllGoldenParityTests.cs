using System.Collections.Generic;
using System.IO;
using System.Runtime.CompilerServices;
using System.Text.Json;
using DraftRightWindows.Models;
using DraftRightWindows.Services;
using Xunit;

namespace DraftRightWindows.PureTests;

/// <summary>
/// Windows grammar fixAll must match the shared golden vectors (#107, RULE #1).
///
/// <c>shared/grammar_fixall_golden_vectors.json</c> at the repo root is the
/// single source of truth; the macOS (Swift) and Linux (Python) ports assert
/// against the same file, so the three copies of the apply-all logic cannot
/// drift.
/// </summary>
public class GrammarFixAllGoldenParityTests
{
    private static string GoldenPath([CallerFilePath] string thisFile = "")
        => Path.GetFullPath(Path.Combine(
            Path.GetDirectoryName(thisFile)!, "..", "..",
            "shared", "grammar_fixall_golden_vectors.json"));

    [Fact]
    public void EveryCaseMatchesTheSharedVectors()
    {
        using var doc = JsonDocument.Parse(File.ReadAllText(GoldenPath()));
        var cases = doc.RootElement.GetProperty("cases");
        Assert.True(cases.GetArrayLength() > 0, "golden file is empty");

        foreach (var c in cases.EnumerateArray())
        {
            var name = c.GetProperty("name").GetString();
            var issues = new List<GrammarIssue>();
            foreach (var i in c.GetProperty("issues").EnumerateArray())
            {
                issues.Add(new GrammarIssue
                {
                    Original = i.GetProperty("original").GetString()!,
                    Suggestion = i.GetProperty("suggestion").GetString()!,
                    Offset = i.GetProperty("offset").GetInt32(),
                });
            }

            var got = GrammarFixer.FixAll(c.GetProperty("text").GetString()!, issues);
            Assert.Equal(c.GetProperty("expected").GetString(), got);
        }
    }
}
