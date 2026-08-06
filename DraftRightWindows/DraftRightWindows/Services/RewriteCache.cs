// In-memory client-side rewrite cache.
//
// Mirrors DraftRightLinux/draftright/services/rewrite_cache.py and the macOS
// RewriteCache: key = (language, tone, text), bounded size, evict the oldest
// slice when full. Saves a backend round-trip when the user retries the same
// text+tone combo — reopening the panel, or toggling tones back and forth.
//
// Pure logic: no WinUI, no WinForms, no HttpClient. That is deliberate — it
// lets DraftRightWindows.PureTests cover this headlessly (issue #80). Keep it
// dependency-free.
using System;
using System.Collections.Generic;

namespace DraftRightWindows.Services;

/// <summary>
/// Bounded, insertion-ordered (text, tone, language) → result cache.
/// Thread-safe: the rewrite runs on a background task and the UI reads on the
/// dispatcher thread.
/// </summary>
public class RewriteCache
{
    /// <summary>Entries held before eviction kicks in.</summary>
    public const int DefaultMaxEntries = 200;

    /// <summary>
    /// Evict roughly 1/N of the cache in one pass when it fills up (N = 4 →
    /// 25%), rather than trimming a single entry on every insert once full.
    /// Matches macOS (<c>maxEntries / 4</c>) and Linux
    /// (<c>REWRITE_CACHE_EVICT_FRACTION</c>).
    /// </summary>
    public const int DefaultEvictFraction = 4;

    private readonly int _max;
    private readonly int _evictFraction;
    private readonly Dictionary<string, string> _data;

    // Insertion order for FIFO eviction. A key is enqueued only when it is
    // first inserted, so re-setting an existing key updates the value without
    // changing its position — same semantics as Python's OrderedDict.
    private readonly Queue<string> _order;
    private readonly object _lock = new();

    public RewriteCache(int maxEntries = DefaultMaxEntries, int evictFraction = DefaultEvictFraction)
    {
        _max = Math.Max(1, maxEntries);
        _evictFraction = Math.Max(1, evictFraction);
        _data = new Dictionary<string, string>(StringComparer.Ordinal);
        _order = new Queue<string>();
    }

    /// <summary>
    /// Cache key. Same shape as macOS ("{tone}::{text}") so behaviour matches
    /// across clients, with the target language prepended when it affects the
    /// result — otherwise switching translation language would serve the
    /// previous language from cache.
    /// </summary>
    internal static string Key(string text, string tone, string? language = null)
    {
        var basis = $"{tone}::{text}";
        return string.IsNullOrEmpty(language) ? basis : $"{language}::{basis}";
    }

    /// <summary>Cached result, or null on a miss.</summary>
    public string? Get(string text, string tone, string? language = null)
    {
        lock (_lock)
        {
            return _data.TryGetValue(Key(text, tone, language), out var hit) ? hit : null;
        }
    }

    public void Set(string text, string tone, string result, string? language = null)
    {
        var key = Key(text, tone, language);
        lock (_lock)
        {
            if (!_data.ContainsKey(key) && _data.Count >= _max)
            {
                var remove = Math.Max(1, _max / _evictFraction);
                for (var i = 0; i < remove && _order.Count > 0; i++)
                {
                    _data.Remove(_order.Dequeue());
                }
            }
            if (!_data.ContainsKey(key)) _order.Enqueue(key);
            _data[key] = result;
        }
    }

    /// <summary>Drop everything — called on sign-out so one user's rewrites
    /// are never served to the next.</summary>
    public void Clear()
    {
        lock (_lock)
        {
            _data.Clear();
            _order.Clear();
        }
    }

    public int Count
    {
        get { lock (_lock) { return _data.Count; } }
    }
}
