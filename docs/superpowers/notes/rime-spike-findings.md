# RIME bridge spike — findings + engine decision (Task 1 gate)

Date: 2026-06-07 · Issue #2 · Branch `feature/japanese-ime-20260524`

## Goal
Prove the Japanese kana→kanji engine is viable inside the iOS keyboard
extension (the hard gate) before building UI/packs on it.

## Verdict: **NO-GO on librime · GO on a dictionary-based kana→kanji engine**

### Why NO-GO on librime (as originally specced)
1. **iOS extension memory cap is brutal.** Custom-keyboard extensions are killed
   (jetsam) around **30–48 MB** RSS. Full librime + a Japanese dict + its runtime
   is a poor fit; staying under the cap would be a constant fight. (Hamster ships
   librime on iOS, but as a heavy full-keyboard app, and still battles memory.)
2. **Native toolchain cost.** librime is C++: an xcframework (arm64 device + sim)
   + Obj-C++ shim for iOS, and an NDK/CMake JNI bridge for Android. Large,
   ongoing build/maintenance burden.
3. **No signed real-iPhone path in this environment** to validate on-device RSS
   per the gate's own requirement (Step 3). Can't honestly clear the gate as
   written here.

### Why GO on a dictionary engine
The whole framework around the engine is **already built** (Tasks 3/5/7/11):
`RomajiComposer` (rōmaji→kana), the engine-agnostic `CandidateEngine` seam, the
candidate bar (iOS+Android), the downloadable-pack format + resolver, and the
backend manifest. A Japanese engine only has to implement `CandidateEngine`.

**Proven this session (headless, no devices, no native code):**
`JapaneseDictionaryEngine` (Swift) = `RomajiComposer` + a reading→kanji
dictionary. 4 unit tests green:
- `nihongo` → 日本語 (dict hit) + にほんご (plain-kana fallback)
- `kanji` → 漢字 / 幹事 (rank preserved)
- unknown reading → kana only
- empty → no candidates

Pure Swift, reuses every existing piece, mmap-friendly dict → fits the memory
budget librime can't. Kotlin mirror is a straight port (same as the other IME
classes already mirrored).

## Licensing constraint (important)
- **SKK-JISYO dictionaries are GPL** → cannot bundle in the closed-source app.
- Use a permissively-licensed reading→kanji source instead: **Mozc's dictionary**
  (BSD-style) or `jawiki-kana-kanji-dict` (verify its license), delivered as a
  **downloadable pack** (user-fetched data, mmap'd), not bundled in the binary.

## Production notes (for the GO path)
- Store the dict as an mmap'd/CDB-style or trie pack, not an in-memory Swift
  `Dictionary`, to keep extension RSS low (POC used in-memory for the proof).
- Reuse `WordListPackResolver` / pack-download service already built for the
  dictionary pack; add a JP pack descriptor to the registry.
- Quality ceiling: dictionary lookup gives good single-word conversion; it won't
  match librime's full sentence-level n-gram conversion. For a keyboard-rewrite
  product (short inputs, candidate bar) this is an acceptable v1; can add a
  bigram re-rank later.

## Decision needed from user (gate is a user decision point)
Approve the **engine pivot** (dictionary engine, drop librime)? If yes, next
tasks: port `JapaneseDictionaryEngine` to Kotlin, source a permissive JP dict,
build the JP pack + registry descriptor, wire the candidate bar to it when
Japanese is active.
