# Japanese & Chinese Keyboards — Native-Feel Study & DraftRight Parity Map

Companion to `keyboard-samsung-study.md` / `keyboard-ios-study.md`, for CJK.
Goal: make DraftRight's JP/ZH input feel native — like the keyboards people
actually use: **Google Japanese Input / Gboard / iOS 日本語** for Japanese, and
**Sogou / Gboard Pinyin / iOS 拼音** for Chinese. Sourced from those products'
documented behavior + a scan of DraftRight's CJK stack.

## 0. DraftRight's CJK today (baseline, verified this session)

Both languages share the same thin pipeline:
`Composer (romaji→kana / pinyin) → DictionaryCandidateEngine (exact reading
lookup) → candidate bar (tap) `, plus:
- **Space converts** the reading to the top candidate, **repeat-space cycles**,
  enter/next-input confirms, backspace cancels (shipped + on-device verified).
- Seed dictionaries bundled; **real downloadable packs hosted** (`ja-v3` 2 MB,
  `zh-v1` 1.9 MB) installed from Settings.
- Feedback (haptic/sound) + magnified key preview apply (language-agnostic).

**The core limitation:** input is **QWERTY-romaji / QWERTY-pinyin only**, and
conversion is **single-reading, exact-match**. Real CJK keyboards do far more.

## 1. Japanese — what native keyboards do vs DraftRight

| Native JP feature | DraftRight | Gap |
|---|---|---|
| **Romaji (ローマ字) input** | ✅ has | — |
| **Flick input (フリック)** — 12-key kana, flick a kana row for い/う/え/お; **the dominant mobile JP input method** | ❌ none | 🔴 **biggest gap** — most JP mobile users expect flick |
| Toggle/multi-tap (トグル) 12-key | ❌ | 🟡 |
| Convert reading → kanji (space) + **cycle candidates** | ✅ has (this session) | — |
| **Bunsetsu / multi-segment conversion** — convert a whole phrase, adjust segment boundaries (←→) | ❌ single reading only | 🔴 big — real sentences need it |
| **Predictive / suggest (サジェスト)** — candidates before you finish, next-word | ⚠️ partial (prefix only, no learning) | 🟡 |
| **Katakana candidate** (カタカナ) + **half/full-width** (英数/全角) | ❌ | 🟡 |
| Okurigana / verb inflection (たべ→食べる) | ❌ (only exact readings in dict) | 🟡 |
| Number / counter / date conversion | ❌ | 🔵 |
| Emoji / kaomoji (顔文字) / symbol candidates | ❌ | 🟡 |
| **User dictionary + learning** (frequency adapts to you) | ❌ static freq | 🟡 |
| Voice / handwriting | voice ✅ · handwriting ❌ | 🔵 |

## 2. Chinese — what native keyboards do vs DraftRight

| Native ZH feature | DraftRight | Gap |
|---|---|---|
| **Full pinyin → hanzi** | ✅ (single syllable) | — |
| **Sentence-level pinyin** — type a whole sentence's pinyin, auto-segment into hanzi | ❌ single syllable only | 🔴 **biggest gap** — this is how people actually type ZH |
| **Abbreviation / initials** (`bj`→北京, `nh`→你好) | ❌ | 🔴 huge speed feature |
| **Fuzzy pinyin** (zh=z, ing=in, l=n…) | ❌ | 🟡 |
| **Number-key candidate select (1-9)** | ❌ tap-only | 🟡 expected muscle memory |
| Associational / predictive (联想) next-word | ❌ | 🟡 |
| **Simplified ⇄ Traditional** toggle | ❌ | 🟡 |
| Alt input methods: **Zhuyin/Bopomofo** (TW), Cangjie, Wubi, Stroke | ❌ pinyin only | 🔵 (Zhuyin matters for TW) |
| Cloud input (云输入) — server-side better candidates | ❌ | 🔵 |
| Emoji / symbols / user dictionary + learning | ❌ | 🟡 |

## 3. The headline gaps (what actually breaks "native feel")

1. **JP flick input (フリック)** — without it, most Japanese mobile users won't feel
   at home; romaji-QWERTY is the minority input on phones.
2. **ZH sentence-level pinyin + initials-abbreviation** — single-syllable lookup
   is unusably slow vs typing a whole sentence's pinyin and picking segmented hanzi.
3. **Number-key candidate selection (ZH) / richer candidate UX** — muscle memory.
4. **Learning / user dictionary** (both) — native keyboards adapt to you.
5. **Katakana + half/full-width (JP)**, **Simplified/Traditional (ZH)**.

## 4. Prioritized roadmap

**Tier 1 — the "feels native or not" features**
1. **ZH sentence-level pinyin segmentation** — extend `PinyinComposer` +
   `DictionaryCandidateEngine` to segment a multi-syllable pinyin string and
   offer segmented hanzi. Biggest ZH win, pure-logic (Mac/JVM testable).
2. **ZH pinyin abbreviation + fuzzy** — initials lookup + fuzzy-pinyin
   normalization in the dictionary query.
3. **JP flick input (フリック)** — a 12-key kana flick layout + gesture composer.
   Large (new layout + gesture engine) but the defining JP mobile feature.

**Tier 2 — expected polish**
4. **Number-key candidate selection (1-9)** + expandable full candidate list.
5. **JP bunsetsu multi-segment conversion** (phrase-level).
6. **Learning / user dictionary** — per-user frequency boost feeding the engine.
7. **Katakana / half-width (JP)** + **Simplified⇄Traditional (ZH)** toggles.

**Tier 3 — reach**
8. Emoji/kaomoji candidates · Zhuyin/Cangjie (ZH alt methods) · handwriting · cloud input.

## 5. Architecture notes (how these fit the existing seams — RULE #1)

- **Sentence pinyin / abbreviation / fuzzy** are engine/dictionary changes behind
  the existing `CandidateEngine` seam — no UI churn, and mirror-able Kotlin↔Swift
  with an agreement test (like the VI bigrams).
- **Number-key selection** is a candidate-bar + keystroke-routing change (the bar
  already commits on tap; add digit→index).
- **Flick input** is the one that needs a genuinely new surface (12-key layout +
  flick gesture composer) — scope it as its own epic.
- **Learning** rides `LanguageWordList` (a per-user overlay boosting frequencies).
- Keep JP/ZH parity between Android + iOS cores (both already mirror 1:1).

## 6. What NOT to chase first

Cloud input, handwriting, Cangjie/Wubi are niche. The native-feel unlock is
**ZH sentence pinyin + JP flick**; everything else is polish on top.
