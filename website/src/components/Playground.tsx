import { useState } from 'react';

const API_URL = (typeof import.meta !== 'undefined' && import.meta.env?.PUBLIC_API_URL) || 'https://api.draftright.info';

const TONES = [
  { value: 'simple', label: 'Simple', icon: '✎' },
  { value: 'natural', label: 'Natural', icon: '💬' },
  { value: 'polished', label: 'Polished', icon: '✨' },
  { value: 'concise', label: 'Concise', icon: '⊖' },
  { value: 'technical', label: 'Technical', icon: '🔧' },
  { value: 'translate', label: 'Translate', icon: '🌐' },
];

const LANGUAGES: { name: string; flag: string; code: string }[] = [
  { name: 'Arabic', flag: '🇸🇦', code: 'ar' },
  { name: 'Chinese (Simplified)', flag: '🇨🇳', code: 'zh' },
  { name: 'Chinese (Traditional)', flag: '🇹🇼', code: 'zh-TW' },
  { name: 'Czech', flag: '🇨🇿', code: 'cs' },
  { name: 'Danish', flag: '🇩🇰', code: 'da' },
  { name: 'Dutch', flag: '🇳🇱', code: 'nl' },
  { name: 'English', flag: '🇺🇸', code: 'en' },
  { name: 'Finnish', flag: '🇫🇮', code: 'fi' },
  { name: 'French', flag: '🇫🇷', code: 'fr' },
  { name: 'German', flag: '🇩🇪', code: 'de' },
  { name: 'Greek', flag: '🇬🇷', code: 'el' },
  { name: 'Hebrew', flag: '🇮🇱', code: 'he' },
  { name: 'Hindi', flag: '🇮🇳', code: 'hi' },
  { name: 'Hungarian', flag: '🇭🇺', code: 'hu' },
  { name: 'Indonesian', flag: '🇮🇩', code: 'id' },
  { name: 'Italian', flag: '🇮🇹', code: 'it' },
  { name: 'Japanese', flag: '🇯🇵', code: 'ja' },
  { name: 'Korean', flag: '🇰🇷', code: 'ko' },
  { name: 'Malay', flag: '🇲🇾', code: 'ms' },
  { name: 'Norwegian', flag: '🇳🇴', code: 'no' },
  { name: 'Polish', flag: '🇵🇱', code: 'pl' },
  { name: 'Portuguese', flag: '🇧🇷', code: 'pt' },
  { name: 'Romanian', flag: '🇷🇴', code: 'ro' },
  { name: 'Russian', flag: '🇷🇺', code: 'ru' },
  { name: 'Spanish', flag: '🇪🇸', code: 'es' },
  { name: 'Swedish', flag: '🇸🇪', code: 'sv' },
  { name: 'Thai', flag: '🇹🇭', code: 'th' },
  { name: 'Turkish', flag: '🇹🇷', code: 'tr' },
  { name: 'Ukrainian', flag: '🇺🇦', code: 'uk' },
  { name: 'Vietnamese', flag: '🇻🇳', code: 'vi' },
];

const LOCALE_TO_LANGUAGE: Record<string, string> = {};
LANGUAGES.forEach(l => { LOCALE_TO_LANGUAGE[l.code.split('-')[0]] = l.name; });

function detectLanguage(): string {
  if (typeof navigator === 'undefined') return 'Vietnamese';
  const locale = navigator.language?.split('-')[0]?.toLowerCase() || '';
  return LOCALE_TO_LANGUAGE[locale] || 'Vietnamese';
}

const MAX_CHARS = 500;

export default function Playground() {
  const [text, setText] = useState('');
  const [tone, setTone] = useState('polished');
  const [sourceLanguage, setSourceLanguage] = useState('auto');
  const [targetLanguage, setTargetLanguage] = useState(detectLanguage);
  const [result, setResult] = useState('');
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');
  const [triesLeft, setTriesLeft] = useState(3);
  const [copied, setCopied] = useState(false);

  const handleRewrite = async () => {
    if (!text.trim() || loading) return;

    setLoading(true);
    setError('');
    setResult('');
    setCopied(false);

    try {
      const res = await fetch(`${API_URL}/rewrite/trial`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          text,
          tone,
          ...(tone === 'translate' ? {
            target_language: targetLanguage,
            ...(sourceLanguage !== 'auto' ? { source_language: sourceLanguage } : {}),
          } : {}),
        }),
      });

      if (res.status === 429) {
        setError('rate-limit');
        setTriesLeft(0);
        return;
      }

      if (!res.ok) {
        throw new Error('Something went wrong. Please try again.');
      }

      const data = await res.json();
      setResult(data.rewritten_text);
      setTriesLeft((prev) => Math.max(0, prev - 1));
    } catch (err: any) {
      if (error !== 'rate-limit') {
        setError(err.message || 'Something went wrong. Please try again.');
      }
    } finally {
      setLoading(false);
    }
  };

  const handleCopy = async () => {
    await navigator.clipboard.writeText(result);
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  };

  const exhausted = triesLeft === 0;

  return (
    <section id="playground" className="py-20 px-4 sm:px-6 lg:px-8 max-w-5xl mx-auto">
      <h2 className="text-3xl sm:text-4xl font-bold text-center text-white mb-3">
        Try It Now
      </h2>
      <p className="text-center text-slate-400 mb-10">
        Paste any text and pick a tone — see the magic in seconds.
      </p>

      {/* Input */}
      <div className="relative mb-4">
        <textarea
          value={text}
          onChange={(e) => setText(e.target.value.slice(0, MAX_CHARS))}
          placeholder="Paste or type your text here..."
          rows={5}
          className="w-full rounded-lg bg-dark-bg border border-dark-border text-white placeholder-slate-500 p-4 pr-16 resize-none focus:outline-none focus:ring-2 focus:ring-brand-400 focus:border-transparent"
          disabled={exhausted}
        />
        <span className="absolute bottom-3 right-3 text-xs text-slate-500">
          {text.length}/{MAX_CHARS}
        </span>
      </div>

      {/* Tone Buttons */}
      <div className="flex flex-wrap gap-2 mb-6">
        {TONES.map((t) => {
          const sourceFlag = LANGUAGES.find(l => l.name === sourceLanguage)?.flag || '🔍';
          const targetFlag = LANGUAGES.find(l => l.name === targetLanguage)?.flag || '🌐';
          const icon = t.value === 'translate' ? sourceFlag : t.icon;
          const label = t.value === 'translate'
            ? `${sourceLanguage === 'auto' ? 'Auto' : sourceLanguage} → ${targetLanguage}`
            : t.label;
          return (
            <button
              key={t.value}
              onClick={() => setTone(t.value)}
              disabled={exhausted}
              className={`px-4 py-2 rounded-lg border text-sm font-medium transition-colors ${
                tone === t.value
                  ? 'bg-brand-400 text-white border-brand-400'
                  : 'bg-dark-bg text-slate-300 border-dark-border hover:border-brand-400'
              } disabled:opacity-50 disabled:cursor-not-allowed`}
            >
              <span className="mr-1.5">{icon}</span>
              {t.value === 'translate' && <span className="mr-1">→ {targetFlag}</span>}
              {label}
            </button>
          );
        })}
      </div>

      {/* Language Selection (visible when Translate is selected) */}
      {tone === 'translate' && (
        <div className="mb-6 space-y-4">
          {/* From Language */}
          <div>
            <p className="text-sm text-slate-400 mb-3">From:</p>
            <div className="flex flex-wrap gap-2">
              <button
                onClick={() => setSourceLanguage('auto')}
                disabled={exhausted}
                className={`flex items-center gap-1.5 px-3 py-1.5 rounded-lg border text-sm transition-colors ${
                  sourceLanguage === 'auto'
                    ? 'bg-brand-400/15 border-brand-400 text-white'
                    : 'bg-dark-bg border-dark-border text-slate-400 hover:border-slate-500 hover:text-slate-300'
                } disabled:opacity-50 disabled:cursor-not-allowed`}
              >
                <span className="text-base leading-none">🔍</span>
                <span>Auto-detect</span>
              </button>
              {LANGUAGES.map((lang) => (
                <button
                  key={`from-${lang.name}`}
                  onClick={() => setSourceLanguage(lang.name)}
                  disabled={exhausted}
                  title={lang.name}
                  className={`flex items-center gap-1.5 px-3 py-1.5 rounded-lg border text-sm transition-colors ${
                    sourceLanguage === lang.name
                      ? 'bg-brand-400/15 border-brand-400 text-white'
                      : 'bg-dark-bg border-dark-border text-slate-400 hover:border-slate-500 hover:text-slate-300'
                  } disabled:opacity-50 disabled:cursor-not-allowed`}
                >
                  <span className="text-base leading-none">{lang.flag}</span>
                  <span className="hidden sm:inline">{lang.name}</span>
                </button>
              ))}
            </div>
          </div>

          {/* Arrow */}
          <div className="flex justify-center">
            <span className="text-slate-500 text-xl">↓</span>
          </div>

          {/* To Language */}
          <div>
            <p className="text-sm text-slate-400 mb-3">To:</p>
            <div className="flex flex-wrap gap-2">
              {LANGUAGES.map((lang) => (
                <button
                  key={`to-${lang.name}`}
                  onClick={() => setTargetLanguage(lang.name)}
                  disabled={exhausted}
                  title={lang.name}
                  className={`flex items-center gap-1.5 px-3 py-1.5 rounded-lg border text-sm transition-colors ${
                    targetLanguage === lang.name
                      ? 'bg-emerald-400/15 border-emerald-400 text-emerald-300'
                      : 'bg-dark-bg border-dark-border text-slate-400 hover:border-slate-500 hover:text-slate-300'
                  } disabled:opacity-50 disabled:cursor-not-allowed`}
                >
                  <span className="text-base leading-none">{lang.flag}</span>
                  <span className="hidden sm:inline">{lang.name}</span>
                </button>
              ))}
            </div>
          </div>
        </div>
      )}

      {/* Rewrite Button */}
      {!exhausted && (
        <button
          onClick={handleRewrite}
          disabled={loading || !text.trim()}
          className="w-full py-3 rounded-lg bg-brand-400 text-white font-semibold text-lg hover:bg-brand-500 transition-colors disabled:opacity-50 disabled:cursor-not-allowed mb-2"
        >
          {loading ? (
            <span className="inline-flex items-center gap-2">
              <svg className="animate-spin h-5 w-5" viewBox="0 0 24 24" fill="none">
                <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4" />
                <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z" />
              </svg>
              Rewriting...
            </span>
          ) : (
            'Rewrite'
          )}
        </button>
      )}

      {!exhausted && !error && (
        <p className="text-center text-sm text-slate-500 mb-6">
          {triesLeft} free {triesLeft === 1 ? 'try' : 'tries'} remaining
        </p>
      )}

      {/* Result */}
      {result && (
        <div className="mt-4 rounded-lg border border-emerald-700/50 bg-emerald-950/30 p-4">
          <p className="text-emerald-200 whitespace-pre-wrap leading-relaxed">{result}</p>
          <button
            onClick={handleCopy}
            className="mt-3 inline-flex items-center gap-1.5 text-sm text-emerald-400 hover:text-emerald-300 transition-colors"
          >
            {copied ? (
              <>
                <svg className="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                  <path strokeLinecap="round" strokeLinejoin="round" d="M5 13l4 4L19 7" />
                </svg>
                Copied!
              </>
            ) : (
              <>
                <svg className="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                  <path strokeLinecap="round" strokeLinejoin="round" d="M8 16H6a2 2 0 01-2-2V6a2 2 0 012-2h8a2 2 0 012 2v2m-6 12h8a2 2 0 002-2v-8a2 2 0 00-2-2h-8a2 2 0 00-2 2v8a2 2 0 002 2z" />
                </svg>
                Copy to clipboard
              </>
            )}
          </button>
        </div>
      )}

      {/* Error / Rate Limit */}
      {error === 'rate-limit' && (
        <div className="mt-6 text-center rounded-lg border border-amber-700/50 bg-amber-950/30 p-6">
          <p className="text-amber-300 font-medium mb-2">You've reached the daily free limit.</p>
          <a
            href="#download"
            className="inline-block mt-2 px-6 py-3 rounded-lg bg-brand-400 text-white font-semibold hover:bg-brand-500 transition-colors"
          >
            Download App for Unlimited Rewrites
          </a>
        </div>
      )}

      {error && error !== 'rate-limit' && (
        <p className="mt-4 text-center text-red-400 text-sm">{error}</p>
      )}

      {/* Exhausted CTA */}
      {exhausted && error !== 'rate-limit' && (
        <div className="mt-6 text-center rounded-lg border border-brand-400/30 bg-brand-400/10 p-6">
          <p className="text-slate-300 font-medium mb-2">You've used all 3 free tries.</p>
          <a
            href="#download"
            className="inline-block mt-2 px-6 py-3 rounded-lg bg-brand-400 text-white font-semibold hover:bg-brand-500 transition-colors"
          >
            Download App for Unlimited Rewrites
          </a>
        </div>
      )}
    </section>
  );
}
