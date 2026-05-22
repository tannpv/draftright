# CJK Input — Downloadable-Pack IME Framework (Phase 1: Japanese)

**Status:** Design / approved to spec
**Date:** 2026-05-22
**Owner:** Tan Nguyen

## Goal

Let users type **CJK languages inside the DraftRight keyboard** (not just rewrite
already-typed text), starting with **Japanese**. Each language's heavy
dictionary data is **downloaded on demand** so the base app stays small and
users only pay the data cost for languages they actually use.

Phase 1 ships the reusable framework + the Japanese language. Chinese (Pinyin)
and Korean (Hangul) are explicit follow-on phases that reuse it.

## Background / current state

- DraftRight keyboards today handle **Latin only** (EN, VI-Telex, FR, ES, DE,
  IT, PT) via a per-language `LanguagePack` + a lightweight `Composer`
  (diacritic composition, e.g. `TelexComposer`). No candidate selection.
- CJK is currently **out of the keyboard** — those users are routed to their
  own system keyboard + the Share / Process-Text flow for rewriting.
- Neither iOS nor Android exposes the **system** kana→kanji / pinyin→hanzi
  engine to a third-party keyboard, so a custom keyboard must bring its own
  conversion engine + dictionary. (Confirmed: iOS has no public conversion
  API; Android system IMEs aren't callable by other apps.)

## Why on-device (not server) conversion

Input conversion is the **hot path** — it fires on every phrase, unlike the
occasional rewrite call. A server in the typing loop makes the keyboard feel
broken on poor networks and can't match native-IME instant/offline behavior.
Therefore conversion is **on-device**; any server role is an optional later
enhancement, never blocking input.

## Engine: RIME (librime)

- **librime** — BSD-3 licensed, schema-driven C++ IME engine. One engine
  serves **Chinese Pinyin and Japanese** (and more); per-language differences
  live in the **schema + dictionary** = our downloadable packs.
- **Proven in mobile keyboards:** Trime (Android IME) and Hamster (iOS keyboard
  extension) both embed librime, including under the iOS extension memory cap —
  so this is a trodden integration path, not greenfield.
- *Rejected alternative:* Mozc (best-in-class Japanese, BSD) is JP-only and
  would require bridging a second engine for Chinese. RIME chosen for a unified
  framework; a JP pack could be swapped to a Mozc-backed converter later if
  quality demands.

## Architecture

### Bundled in the app (small, always present)
- **IME framework:**
  - **Composer** — romaji/pinyin input buffer (extends the existing `Composer`
    seam in `DraftRightKeyboardCore` / the Kotlin core).
  - **Candidate bar UI** — horizontal scrollable candidates above the keys,
    with page/expand; commit on tap, space = pick top, number keys = pick Nth.
  - **RIME engine binary** — bridged into iOS (Swift ↔ C++ via Obj-C++) and
    Android (Kotlin ↔ NDK/JNI). Engine code is a few MB; ships in the app.
- **Korean (later phase, no pack):** pure **Hangul composition automaton**
  (2-set 두벌식). No engine/dictionary — rides the existing composer model.

### Downloaded on demand (per-language packs)
- A **pack** = a RIME schema + compiled dictionary for one language
  (Japanese ≈ 15–30 MB; Chinese-Pinyin ≈ 15–30 MB).
- **Versioned + checksummed.** Loaded at runtime via **mmap** (only touched
  pages resident → low RAM, fits the iOS keyboard-extension memory budget).

### Pack delivery (self-hosted)
- Packs hosted on **draftright.info / CDN** (reuse existing downloads infra).
- **Manifest endpoint** (e.g. `GET /ime-packs/manifest`) returns a list of
  `{ id, language, version, size_bytes, sha256, url }`.
- **The main app downloads** a pack into the **App Group container (iOS)** /
  **app files dir (Android)**; the **keyboard extension reads + mmaps it** from
  that shared location. (The iOS extension cannot reliably download itself —
  memory + background limits — so the host app owns downloads.)
- Integrity: verify sha256 before activating; atomic install (download to temp,
  verify, move into place); resumable/retryable download.

### User flow
1. Settings → Languages → enable **日本語**.
2. App prompts **"Download Japanese (≈18 MB)?"** → downloads with progress.
3. On success, the keyboard offers Japanese (globe-cycle includes it).
4. Disable / "Remove data" → delete the pack, reclaim space; keyboard drops it.

### Data flow (typing a Japanese phrase)
```
key taps → Composer builds romaji → on each change, RIME (mmap'd JP pack)
produces ranked candidates → candidate bar renders them → user picks (tap /
space / number) → committed text inserted via the text proxy → composer resets
```
No network in this loop. Rewrite (tone buttons) remains a separate, occasional
backend call on the committed text — unchanged.

## Components & boundaries
| Unit | Responsibility | Depends on |
|---|---|---|
| `ImePackManager` (host app, per platform) | list/download/verify/install/remove packs; expose installed packs to the extension via shared storage | manifest endpoint, App Group / files dir |
| `RimeEngine` bridge | load a schema+dict (mmap), feed input, return candidates | librime (C++/NDK), a pack on disk |
| `CandidateBarView` | render + select candidates | engine output |
| `Composer` (existing seam) | accumulate romaji/pinyin; reset on commit/context change | — |
| Korean `HangulComposer` (later) | jamo→syllable automaton | — (no pack) |

## Phasing
- **Phase 1 (this spec):** IME framework + pack system + **Japanese** (RIME JP
  pack). Proves engine bridge, candidate UI, and the download/mmap pipeline.
- **Phase 2:** **Chinese (Pinyin)** — a new RIME schema+dict pack; reuses
  everything. Separate spec.
- **Phase 3:** **Korean (Hangul)** — composition automaton; independent of the
  pack system, can land anytime. Separate spec.

## Risks & mitigations
- **Engine bridging effort (highest risk):** embedding/bridging librime into
  both Swift and Kotlin keyboard targets is the hard part (weeks). Mitigate by
  validating a minimal "type romaji → get candidates" spike on a **real iOS
  device + Android device early**, before building UI/packs.
- **iOS extension memory:** mmap + librime's proven Hamster usage; must be
  verified on a physical iPhone (the keyboard extension OOMs are silent).
- **App Store / Play review:** a Full-Access keyboard that downloads data and
  sends **nothing** to a server for input is a *good* privacy story (input
  stays on-device); the pack download needs a clear disclosure. Note the
  keyboard is currently mid-review — this ships on a **separate track**.
- **Licensing:** librime BSD = fine; each **dictionary** pack's data license
  (pinyin/JP dictionaries) must be vetted before shipping that pack.
- **Pack hosting/versioning:** packs must stay compatible with the bundled
  engine version; manifest carries a min-engine-version field; app refuses
  incompatible packs.

## Out of scope (Phase 1)
- Chinese, Korean (later phases).
- 12-key flick / Zhuyin / Cangjie layouts (romaji/pinyin-on-QWERTY only).
- Server-side conversion or hybrid (on-device only).
- On-device personalization/learning beyond what RIME provides by default.

## Success criteria
- On a real iPhone and Android phone: enable Japanese → pack downloads →
  type romaji → correct kana→kanji candidates appear → selection commits →
  rewrite still works on the committed Japanese text.
- Base app size unchanged for users who don't enable a CJK language.
- iOS keyboard extension stays within its memory budget while Japanese is
  active (no jetsam kills).
