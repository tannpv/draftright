# Android Multi-Language Keyboard — Manual QA Matrix

**Feature:** Tier β multi-language keyboard (VI/FR/ES/DE/IT/PT in addition to EN)
**Branch:** `feature/android-multilang-keyboard-20260517`
**Spec:** `docs/superpowers/specs/2026-05-17-android-multilang-keyboard-design.md`
**Plan:** `docs/superpowers/plans/2026-05-17-android-multilang-keyboard.md`
**Test catalog:** `docs/test-cases.xlsx` sheet `KEYBOARD-MULTI` (rows 001-054)

## Devices

| Device | OS | Why this device |
|---|---|---|
| Samsung A52 | Android 13 (One UI 5) | Samsung strips Process Text — verify the keyboard + share intent paths still work without it |
| Xiaomi Redmi | Android 12 (MIUI 14) | MIUI throttles background services aggressively — verify keystroke responsiveness |
| Pixel emulator (or physical Pixel) | Android 14 | Baseline AOSP behavior, fastest iteration loop |

## Per-device, per-language matrix

For each device, exercise each of the seven languages in this sequence. Capture a screenshot or short screen recording per row.

| Step | Action | Expected |
|---|---|---|
| 1 | Enable both EN and the language under test in Settings → Keyboard languages | Both chips appear; FilterChip toggle visible |
| 2 | Open Messages / WhatsApp / any text field | DraftRight keyboard appears with the language strip above the tone toolbar |
| 3 | Tap the language chip for the language under test | Chip highlights blue; space-bar label changes to the native name |
| 4 | Type a real word with that language's diacritics (see word list below) | The word renders correctly with marks/tones |
| 5 | Tap the globe key 🌐 | Active language cycles to the next enabled pack |
| 6 | Long-press a vowel that has accents (e.g., `a` in ES) | Accent popup appears above the held key; tap an accent commits it |
| 7 | Force-stop the app, relaunch the keyboard | The previously-active language is still active |
| 8 | Run the tone rewrite (Polished) on a sentence in that language | Backend roundtrip succeeds; replaced text is still UTF-8 clean |

### Words to type per language

| Language | Word | Telex / typed sequence |
|---|---|---|
| Vietnamese | việt | v i e t j |
| Vietnamese | tiếng | t i e s n g |
| Vietnamese | người | n g u o w i f |
| Vietnamese | chương | c h u o w n g |
| French | café | c long-press a → á, OR direct ASCII fallback + accent |
| French | déjà vu | é long-press; à long-press |
| Spanish | mañana | m a long-press ñ a n a (or ñ key direct in ES) |
| Spanish | ¿qué? | long-press ? for ¿, type qué |
| German | schön | s c h o long-press → ö n (or ö dedicated key) |
| German | Straße | direct ß key in row 2 |
| Italian | però | p e r long-press o → ò |
| Italian | è | long-press e → è |
| Portuguese | açúcar | a long-press c → ç u long-press → ú c a r (or ç key direct) |
| Portuguese | coração | c o r a long-press → ç a long-press → ã o |

## Regression-proof EN

For each device, perform the EN-only baseline:

| Step | Action | Expected |
|---|---|---|
| 1 | Disable all non-EN languages in Settings | Only the EN chip remains; strip hidden |
| 2 | Open a text field, type `hello world quickly` | Identical keystroke output to the pre-Tier-β build |
| 3 | Type `?123` to enter symbols → digits → back to ABC | Layer cycling unchanged |
| 4 | Run the tone rewrite on the typed sentence | Backend roundtrip unchanged |
| 5 | Share text into DraftRight from another app | Share-intent rewrite flow unchanged |

## Pass criteria

- All seven languages produce their target diacritics on all three devices.
- EN-only flow has zero observable change from the pre-feature build.
- Tone rewrite + share intent both unaffected.
- Keystroke-to-screen latency feels indistinguishable from pre-feature (subjective; spec §9.5 budget is <30 ms on Samsung A52).
- No crashes, no ANRs in the logs.

## Sign-off

| Device | Tester | Date | Result | Notes |
|---|---|---|---|---|
| Samsung A52 | | | | |
| Xiaomi Redmi | | | | |
| Pixel emulator | | | | |

Apply `status: tested` label on the GitHub tracking issue once all three rows are green. Then promote to `status: deployed to production` after the Play Store roll-out.
