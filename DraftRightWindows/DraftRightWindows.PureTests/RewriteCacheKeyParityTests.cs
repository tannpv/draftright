using System.Text.Json;
using DraftRightWindows.Services;
using Xunit;

namespace DraftRightWindows.PureTests;

/// <summary>
/// Cross-platform parity for the RewriteCache key. The key algorithm is
/// duplicated in three languages (macOS Swift, Windows C#, Linux Python);
/// this test asserts the C# <see cref="RewriteCache.Key"/> reproduces every
/// vector in the shared <c>parity/rewrite-cache-key-vectors.json</c> fixture,
/// which is the single source of truth all three platforms are checked against.
/// See issue #174 (and #147/#108). If this fails, the C# key format drifted
/// from the other platforms — fix the code, not the fixture (unless the format
/// was changed on purpose in all three).
/// </summary>
public class RewriteCacheKeyParityTests
{
    public sealed record Vector(string Text, string Tone, string? Language, string ExpectedKey);

    private sealed record Fixture(Vector[] Vectors);

    /// <summary>
    /// Locate the repo-root <c>parity/</c> fixture by walking up from the test
    /// assembly's output directory — robust to the bin/Debug/... nesting and to
    /// running from CI, a dev box, or the IDE.
    /// </summary>
    private static string FixturePath()
    {
        var dir = new DirectoryInfo(AppContext.BaseDirectory);
        while (dir is not null)
        {
            var candidate = Path.Combine(dir.FullName, "parity", "rewrite-cache-key-vectors.json");
            if (File.Exists(candidate))
            {
                return candidate;
            }
            dir = dir.Parent;
        }
        throw new FileNotFoundException(
            "parity/rewrite-cache-key-vectors.json not found walking up from " + AppContext.BaseDirectory);
    }

    public static IEnumerable<object[]> Vectors()
    {
        var json = File.ReadAllText(FixturePath());
        var options = new JsonSerializerOptions { PropertyNameCaseInsensitive = true };
        var fixture = JsonSerializer.Deserialize<Fixture>(json, options)!;
        foreach (var v in fixture.Vectors)
        {
            yield return new object[] { v };
        }
    }

    [Theory]
    [MemberData(nameof(Vectors))]
    public void Key_MatchesSharedGoldenVector(Vector v)
    {
        Assert.Equal(v.ExpectedKey, RewriteCache.Key(v.Text, v.Tone, v.Language));
    }
}
