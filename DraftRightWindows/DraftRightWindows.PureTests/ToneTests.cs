using DraftRightWindows.Models;
using Xunit;

namespace DraftRightWindows.PureTests;

/// <summary>
/// Tone enum mapping. <see cref="ToneExtensions.UsesTargetLanguage"/> drives
/// both the request payload and the rewrite cache key (issue #108), so it is
/// worth pinning rather than leaving as a `tone == Tone.Translate` literal in
/// the view model.
/// </summary>
public class ToneTests
{
    [Fact]
    public void OnlyTranslate_UsesTargetLanguage()
    {
        Assert.True(Tone.Translate.UsesTargetLanguage());
        foreach (var tone in (Tone[])System.Enum.GetValues(typeof(Tone)))
        {
            if (tone == Tone.Translate) continue;
            Assert.False(tone.UsesTargetLanguage(), $"{tone} should not use a target language");
        }
    }

    [Fact]
    public void OnlyGrammarCheck_ProducesUncacheableOutput()
    {
        // Grammar Check returns a structured issues object; caching it as a
        // string would drop the structure. Every other tone returns plain text.
        Assert.False(Tone.GrammarCheck.ProducesCacheableText());
        foreach (var tone in (Tone[])System.Enum.GetValues(typeof(Tone)))
        {
            if (tone == Tone.GrammarCheck) continue;
            Assert.True(tone.ProducesCacheableText(), $"{tone} should be cacheable");
        }
    }

    [Fact]
    public void ApiValue_RoundTrips_ForEveryTone()
    {
        foreach (var tone in (Tone[])System.Enum.GetValues(typeof(Tone)))
        {
            Assert.Equal(tone, ToneExtensions.FromApiValue(tone.ApiValue()));
        }
    }

    [Fact]
    public void FromApiValue_ReturnsNull_ForUnknown()
    {
        Assert.Null(ToneExtensions.FromApiValue("nope"));
        Assert.Null(ToneExtensions.FromApiValue(null));
    }
}
