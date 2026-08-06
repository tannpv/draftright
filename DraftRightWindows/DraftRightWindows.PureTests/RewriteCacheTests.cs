using DraftRightWindows.Services;
using Xunit;

namespace DraftRightWindows.PureTests;

/// <summary>
/// Client-side rewrite cache (issue #108). Mirrors the macOS and Linux
/// implementations: key = (language, tone, text), bounded, FIFO eviction.
/// </summary>
public class RewriteCacheTests
{
    private const string Tone = "polished";

    [Fact]
    public void Get_ReturnsNull_OnMiss()
    {
        var c = new RewriteCache();
        Assert.Null(c.Get("hello", Tone));
    }

    [Fact]
    public void Set_ThenGet_RoundTrips()
    {
        var c = new RewriteCache();
        c.Set("hello", Tone, "Hello.");
        Assert.Equal("Hello.", c.Get("hello", Tone));
    }

    [Fact]
    public void DifferentTone_IsADifferentEntry()
    {
        var c = new RewriteCache();
        c.Set("hello", "polished", "Polished.");
        c.Set("hello", "concise", "Concise.");
        Assert.Equal("Polished.", c.Get("hello", "polished"));
        Assert.Equal("Concise.", c.Get("hello", "concise"));
    }

    [Fact]
    public void Language_IsPartOfTheKey()
    {
        // The bug this guards: storing without the language but reading with it
        // (or vice versa) means translations never hit — and worse, switching
        // language could serve the previous language's result.
        var c = new RewriteCache();
        c.Set("hello", "translate", "Hola.", "Spanish");

        Assert.Equal("Hola.", c.Get("hello", "translate", "Spanish"));
        Assert.Null(c.Get("hello", "translate", "French"));
        Assert.Null(c.Get("hello", "translate"));
    }

    [Fact]
    public void ReSettingSameKey_OverwritesWithoutGrowing()
    {
        var c = new RewriteCache();
        c.Set("hello", Tone, "first");
        c.Set("hello", Tone, "second");
        Assert.Equal("second", c.Get("hello", Tone));
        Assert.Equal(1, c.Count);
    }

    [Fact]
    public void Clear_DropsEverything()
    {
        var c = new RewriteCache();
        c.Set("a", Tone, "A");
        c.Set("b", Tone, "B");
        c.Clear();
        Assert.Equal(0, c.Count);
        Assert.Null(c.Get("a", Tone));
    }

    [Fact]
    public void EvictsOldestSlice_WhenFull()
    {
        // max 8, evict fraction 4 → drop the 2 oldest when the 9th arrives.
        var c = new RewriteCache(maxEntries: 8, evictFraction: 4);
        for (var i = 0; i < 8; i++) c.Set($"t{i}", Tone, $"r{i}");
        Assert.Equal(8, c.Count);

        c.Set("t8", Tone, "r8");

        Assert.Equal(7, c.Count);              // 8 - 2 evicted + 1 inserted
        Assert.Null(c.Get("t0", Tone));        // oldest two gone
        Assert.Null(c.Get("t1", Tone));
        Assert.Equal("r2", c.Get("t2", Tone)); // third-oldest survives
        Assert.Equal("r8", c.Get("t8", Tone)); // newcomer present
    }

    [Fact]
    public void EvictionOrder_IsInsertion_NotLastUse()
    {
        // FIFO, matching macOS and Linux — re-reading an entry does not make it
        // survive eviction. Documented so a future switch to LRU is a conscious
        // change rather than an accident.
        var c = new RewriteCache(maxEntries: 4, evictFraction: 4);
        for (var i = 0; i < 4; i++) c.Set($"t{i}", Tone, $"r{i}");
        c.Get("t0", Tone);          // touch the oldest
        c.Set("t4", Tone, "r4");    // triggers eviction of 1 entry

        Assert.Null(c.Get("t0", Tone));   // still evicted despite the read
    }

    [Fact]
    public void ReSettingExistingKey_DoesNotChangeItsEvictionPosition()
    {
        var c = new RewriteCache(maxEntries: 4, evictFraction: 4);
        for (var i = 0; i < 4; i++) c.Set($"t{i}", Tone, $"r{i}");
        c.Set("t0", Tone, "updated");   // overwrite oldest — must not requeue it
        c.Set("t4", Tone, "r4");        // evicts 1

        Assert.Null(c.Get("t0", Tone));         // t0 was still the oldest
        Assert.Equal("r1", c.Get("t1", Tone));
    }

    [Theory]
    [InlineData(0)]
    [InlineData(-5)]
    public void MaxEntries_IsClampedToAtLeastOne(int badMax)
    {
        // A zero/negative bound must not make Set throw or loop forever.
        var c = new RewriteCache(maxEntries: badMax);
        c.Set("a", Tone, "A");
        Assert.Equal("A", c.Get("a", Tone));
        c.Set("b", Tone, "B");
        Assert.Equal("B", c.Get("b", Tone));
    }

    [Fact]
    public void EvictFraction_IsClampedToAtLeastOne()
    {
        var c = new RewriteCache(maxEntries: 4, evictFraction: 0);
        for (var i = 0; i < 5; i++) c.Set($"t{i}", Tone, $"r{i}");
        Assert.Equal("r4", c.Get("t4", Tone));
    }

    [Fact]
    public void DefaultsMatchTheOtherClients()
    {
        // macOS: maxEntries = 200, evicts maxEntries/4.
        // Linux: REWRITE_CACHE_MAX_ENTRIES = 200, EVICT_FRACTION = 4.
        Assert.Equal(200, RewriteCache.DefaultMaxEntries);
        Assert.Equal(4, RewriteCache.DefaultEvictFraction);
    }

    [Fact]
    public void KeyShape_MatchesMacOsAndLinux()
    {
        Assert.Equal("polished::hello", RewriteCache.Key("hello", "polished"));
        Assert.Equal("Spanish::translate::hello", RewriteCache.Key("hello", "translate", "Spanish"));
        // Empty language is treated as absent, not as a distinct bucket.
        Assert.Equal("polished::hello", RewriteCache.Key("hello", "polished", ""));
    }
}
