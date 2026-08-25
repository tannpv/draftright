# Samsung Keyboard (Honeyboard) — Feature Study & DraftRight Parity Map

Reference for the keyboard-parity initiative (#207). Goal: match Samsung's
Vietnamese-Telex / Japanese / Chinese typing experience. Sourced from the
on-device Honeyboard settings tree (`dumpsys package com.samsung.android.honeyboard`)
on a Galaxy A52s (Android 14), the observed keyboard UI, and Honeyboard behaviour.

## 1. Samsung keyboard settings tree (the feature map)

| Category | Features |
|---|---|
| **Languages and types** | Manage input languages; per-language **input mode** (Vietnamese → **Telex** / VNI / VIQR); layout switcher (`LoSwitcher`). |
| **Smart typing** | **Predictive text** (word suggestion bar), **Auto spell check** (auto-correct), **Text shortcuts** (abbreviation → phrase), **Sticker/emoji suggestions**, auto-capitalize, auto-spacing, auto-punctuate ("." on double-space). |
| **Style and layout** | Themes, **High-contrast keyboard**, `KeyboardLayoutSettings` (number row, alternative characters, extra symbol row), keyboard **size/height/position**, one-handed / floating / split modes. |
| **Swipe, touch and feedback** | **Swipe-to-type** (glide typing), **key-tap sound**, **key-tap vibration (haptic)**, **character preview popup**, touch-and-hold delay, **Speak keyboard input aloud** (a11y). |
| **Voice input** | Samsung/Google voice dictation from the keyboard. |
| **AI writer** (`aiwriter`) | Samsung's on-keyboard rewrite/writing-assist (their analogue of DraftRight's tone rewrite). |
| **Direct writing** | S-Pen handwriting straight into any text field. |
| **Chinese input** | Cell dictionary + pinyin/stroke options. |

## 2. Keyboard UI / toolbar (observed)

- **Top toolbar** row: emoji/GIF/sticker, text-editing modes, clipboard, settings (⚙), overflow (…).
- **Suggestion bar**: word candidates while typing (predictive text) — tap to insert.
- Number row (optional), long-press keys → secondary chars/numbers, globe key to cycle languages, spacebar shows current language, resize/one-handed handles.

## 3. Vietnamese Telex — where Samsung feels convenient (the owner's focus)

Owner feedback: Samsung VI-Telex is convenient "especially for strokes and
character manipulation"; DraftRight "feels harder / different". Since DraftRight's
Telex *mechanics* are already mature (see #207 — iê/uô/yê + uôi/iêu/uyê promotion,
w-horn/breve incl. uo→ươ, dd→đ, tone cancel-by-retype, offglide-aware cluster
targeting, #152 base-first), the convenience gap is almost certainly:

1. **Word suggestion / prediction bar for Vietnamese** — you type approximately
   and tap the fully-diacriticised word. This removes the need to place every
   stroke precisely and is the single biggest "convenient" difference. DraftRight
   composes exact Telex with **no VI word suggestions** (its `CandidateBarView`
   is used only for CJK packs + the voice partial transcript).
2. **Auto-correct of mistyped/misplaced tones** ("hoaf"→hòa regardless of style;
   fixes a wrong tone-vowel from context).
3. **Tactile/audio key feedback** — Samsung vibrates + clicks on every keypress;
   DraftRight has **none** (grep: no haptic/sound anywhere in the keyboard),
   which reads as "keys feel dead / harder".

## 4. DraftRight keyboard today — has vs lacks

**Has:** mature Telex/JP/ZH/KO composers; magnified key-press **popup preview**;
**long-press accent popup** (diacritic strokes per key); `CandidateBarView` (CJK
packs + voice); tone/rewrite toolbar; multi-language cycle; hold-to-talk voice.

**Lacks (vs Samsung):**
| Gap | Impact | Effort |
|---|---|---|
| Key-tap **haptic + sound** feedback | HIGH — the "feel" | **Small** (one keypress chokepoint, gated on system settings) |
| **VI/Latin word prediction + candidate bar** | HIGH — the "harder/strokes" convenience | Large (VI dictionary + prediction feeding the existing CandidateBarView) |
| **Auto-correct / typo tolerance** | HIGH | Large (rides on the dictionary) |
| **Swipe-to-type** (glide) | MED | Large |
| Themes / resize / one-handed / floating | MED | Med |
| Clipboard, emoji/GIF/sticker, text shortcuts | LOW–MED | Med |
| Handwriting (Direct writing) | LOW | Large |

## 5. Proposed parity roadmap (VI first, per #207)

1. **Key-tap haptic + sound feedback** — smallest, biggest immediate "feel" win;
   respect `Settings.System` haptic/sound-enabled. Universal (all languages).
2. **Vietnamese word suggestion bar** — feed VI candidates into the existing
   `CandidateBarView` from a bundled VI frequency dictionary + the Telex buffer;
   tap-to-commit. This is the core "convenient strokes/manipulation" gap.
3. **Auto-correct / typo tolerance** on top of the same dictionary.
4. Then JP (kana-kanji candidates) and ZH (pinyin candidates) — thin composers,
   widest gap (#207 phases 2–3).

## 6. To make this empirical (next step)

Samsung on the test device is **English-only** — add *Vietnamese (Telex)* to
Samsung's input languages, then run the shared battery (ươ, uyên, diphthong
tones, đ, glides) on both keyboards and capture Samsung's suggestion-bar +
auto-correct behaviour as the concrete spec.

**Also verify:** committing `dayaj` in DraftRight rendered as `dậý` (y-acute?) in
the field — confirm no acute leaks onto the final `y` (should be `dậy`).
