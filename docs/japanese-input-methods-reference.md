# Japanese Mobile Input Methods — Reference

> Reference for future DraftRight Japanese keyboard work (flick input, predictive
> conversion, homophone candidate UI). Current shipped JP IME = Romaji→Kana
> (`RomajiKanaComposer`) + word-level dict candidate bar. **Flick input is NOT
> yet built** — this doc captures the target behavior for that task.

## Two input styles on Japanese phones

- **Flick Input (フリック入力)** — 12-key pad, tap-and-swipe. Dominant among
  native users; fastest once learned.
- **Romaji Input (ローマ字入力)** — QWERTY, type Latin spelling → converts to kana.
  Familiar to anyone who touch-types English. **This is what DraftRight ships today.**

## 1. 12-key Flick — vowel → direction mapping

Each key = a consonant *row* (あ-column kana printed on it). The five vowels sit
in the SAME five positions on EVERY key — learn the geometry once.

| Direction | Vowel (段) | Example on か key |
|-----------|-----------|-------------------|
| Center (tap) | a (あ段) | か |
| Left (←) | i (い段) | き |
| Up (↑) | u (う段) | く |
| Right (→) | e (え段) | け |
| Down (↓) | o (お段) | こ |

12 keys: **あ か さ た な は ま や ら わ** + punctuation key + **゛゜小 key**
(voiced / half-voiced / small kana).

Examples:
- た key: た(tap) ・ ち(←) ・ つ(↑) ・ て(→) ・ と(↓)
- ら key: ら(tap) ・ り(←) ・ る(↑) ・ れ(→) ・ ろ(↓)

## 2. Worked example — typing「ありがとう」(a-ri-ga-to-u)

### Flick method

| # | Kana | Key | Action |
|---|------|-----|--------|
| 1 | あ | あ key | Tap (center = a) |
| 2 | り | ら key | Flick ← left (ra-row + i) |
| 3 | が | か key | Tap か, then tap ゛ (dakuten) → が |
| 4 | と | た key | Flick ↓ down (ta-row + o) |
| 5 | う | あ key | Flick ↑ up (a-row + u) |

Voiced sounds (が ざ だ ば): enter unvoiced kana first (か), then tap ゛.
Half-voiced (ぱ ぴ): tap ゜ on the same key.

### Romaji method

Type: `a → r → i → g → a → t → o → u` → IME shows ありがとう. Press Space/変換
for Kanji form (e.g. 有難う) or confirm to keep hiragana.

Romaji rules: small っ = double next consonant (`kitte`→きって); ん = `nn` or `n`;
long vowels spelled out (`tou`→とう).

## 3. Predictive text — Kanji homophone selection

Many words share a reading. Classic: **かみ (kami)** → 紙 (paper) / 髪 (hair) / 神 (god).

Flow:
1. Type reading in kana (か→み = `kami`).
2. Trigger conversion (Space/変換) or read the candidate strip above the keys.
3. IME shows a ranked candidate list (紙, 髪, 神, かみ, カミ…) → tap the right one.

Ranking signals:
- **Context** — surrounding words. 「髪を切る」pushes 髪; 「紙に書く」pushes 紙.
- **Frequency** — common/recent words weighted higher.
- **Personal learning** — repeated picks bubble up next time.

> Design takeaway for DraftRight: type whole phrase before converting; context-aware
> ranking should put the right Kanji first. Our current engine is word-level dict
> lookup, NOT phrase-level statistical conversion — that's the real-world gap vs
> native IMEs.

## 4. Beginner speed tips

1. **Internalize the vowel cross, not 50 keys.** left/up/right/down = i/u/e/o on
   every row → only 10 consonant positions + one universal flick pattern.
2. **Convert by phrase, not by syllable.** Type a chunk in kana, convert once —
   predictive engine far more accurate with context.
3. **Flick-only mode + no-look swipes.** Disable old multi-tap (か×4=こ) to kill
   accidental repeats; drill 5–10 everyday words to muscle memory.

## DraftRight implementation notes (gap analysis)

- Shipped JP composer = `RomajiKanaComposer` (Kotlin + Swift), romaji→kana, no flick.
- Candidate bar = `JapaneseDictionaryEngine` word-level dict lookup (no librime).
- **To add Flick input:** new 12-key layout + flick-direction gesture handler →
  emits kana directly (skips romaji stage). Vowel mapping table above is the spec.
  Voiced/small handled by ゛゜小 key post-modifier on the composing buffer.
- **Homophone UI** already partially exists (candidate bar); phrase-level context
  ranking is the upgrade path.
