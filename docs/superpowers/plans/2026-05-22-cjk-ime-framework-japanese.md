# CJK IME Framework (Phase 1: Japanese) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add in-keyboard Japanese typing (romaji→kana→kanji with a candidate bar) to the DraftRight iOS + Android keyboards, powered by an on-device RIME engine with the Japanese dictionary delivered as a downloadable, mmap'd language pack — building the reusable framework Chinese/Korean will later plug into.

**Architecture:** On-device librime (BSD C++) bridged into the iOS keyboard extension (Swift↔Obj-C++) and the Android IME (Kotlin↔JNI/NDK). A romaji composer feeds RIME; a candidate bar renders results; selection commits text. The Japanese RIME schema + dictionary is a self-hosted **pack** the main app downloads into the App Group (iOS) / files dir (Android); the extension mmaps it. No network in the typing loop.

**Tech Stack:** librime (C++), CMake/NDK (Android), xcframework (iOS), Swift, Kotlin, Flutter (host-app download UI + settings), NestJS (pack manifest endpoint), existing `DraftRightKeyboardCore` (Swift) + `com.draftright.keyboard` (Kotlin).

---

## Progress (updated 2026-05-25)

- ✅ **Task 2** — backend manifest endpoint (`GET /ime-packs/manifest`, LanguageModule catalog). Committed.
- ✅ **Task 3** — RomajiComposer (Swift + Kotlin), Core suite green. Committed.
- ✅ **Task 5** — `ImePackService` (download/verify-sha256/atomic-install/remove) + native `sharedPackDir` resolvers (iOS App Group / Android files dir) + `forPlatform()` factory. 5 hermetic tests green. Committed.
- ✅ **Task 7** — `LanguageModule` model + `ImeManifestClient` + reusable `LanguagePacksSection` widget + Settings wiring ("Add a language (download)"). manifest 3/3 + widget 3/3 green. Committed.
- ⛔ **Task 1 (RIME spike, HARD GATE)** — NOT started. Device-bound. **Blocks Tasks 4, 6, 8.**
- ⏸ **Tasks 4, 6, 8** — gated on Task 1 GO.

**Next action:** run the Task 1 RIME spike on a real iPhone + Android device, write `rime-spike-findings.md` with GO/NO-GO, then resume Tasks 4 → 6 → 8.

---

## ⛔ Gate: Task 1 must pass before Tasks 4+ begin

Tasks 2–3 (manifest + composer) are safe to build in parallel and have no engine dependency. **Tasks 4 onward depend on Task 1 proving the RIME bridge is viable inside the iOS keyboard extension.** If Task 1's checkpoint fails (won't build, OOMs the extension, or can't return candidates), STOP and revisit the engine decision (Mozc, or a thinner converter) before continuing — do not build UI/packs on an unproven engine.

---

## File Structure

**New (shared concept, per-platform impls):**
- iOS engine bridge: `DraftRightMobile/ios/DraftRightKeyboardCore/Sources/DraftRightKeyboardCore/IME/RimeEngine.swift` + a C++ shim target `…/Sources/RimeBridge/` (Obj-C++ wrapping librime).
- Android engine bridge: `DraftRightMobile/android/app/src/main/cpp/rime_bridge.cpp` + `…/keyboard/ime/RimeEngine.kt` (JNI).
- Candidate model (shared logic, headless-testable): `…/DraftRightKeyboardCore/Sources/DraftRightKeyboardCore/IME/CandidateController.swift` and `…/keyboard/ime/CandidateController.kt`.
- Candidate bar UI: iOS `DraftRightKeyboard/CandidateBarView.swift`; Android `keyboard/CandidateBarView.kt`.
- Pack manager (host app): `DraftRightMobile/lib/services/ime_pack_service.dart` + platform channel handlers (iOS `Runner/ImePackChannel.swift` writing to App Group; Android `…/ImePackChannel.kt` writing to files dir).
- Backend manifest: `backend/src/ime-packs/ime-packs.controller.ts` + `…/ime-packs.service.ts` + `…/dto/`.

**Modified:**
- `…/DraftRightKeyboardCore/.../LanguagePack.swift` + Kotlin `LanguageRegistry` — add a Japanese pack descriptor that declares it requires a downloaded data pack + a candidate composer.
- iOS `KeyboardViewController.swift` / Android `QwertyKeyboardView.kt` + `DraftRightIME.kt` — host the candidate bar + route keys through the RIME composer when Japanese is active.
- `DraftRightMobile/lib/screens/settings_screen.dart` — language enable/download/remove UI.

---

## Task 1: RIME bridge spike (de-risk — HARD GATE)

Prove librime builds, links, loads a tiny schema, and returns candidates on **a real iOS device (keyboard extension) and a real Android device**, within the iOS extension memory budget. This is exploratory; the goal is a yes/no on the engine, not production code.

**Files:**
- Create: `DraftRightMobile/ios/RimeBridge/` (Obj-C++ shim + a vendored/prebuilt librime xcframework)
- Create: `DraftRightMobile/android/app/src/main/cpp/rime_bridge.cpp`, `CMakeLists.txt`
- Create: `docs/superpowers/notes/rime-spike-findings.md`

- [ ] **Step 1: Vendor librime.** Obtain a prebuilt librime (or build from source via its CMake) for `arm64` device + `arm64 simulator` (iOS) and `arm64-v8a`/`x86_64` (Android). Record exact source/version + build flags in the findings doc. Reference Trime (Android) and Hamster (iOS) build configs.

- [ ] **Step 2: Minimal Obj-C++ shim (iOS).** Expose `rime_start(dataDir)`, `rime_process(romaji) -> [String]` (top candidates) over a C interface; call it from a throwaway Swift test in the keyboard extension target.

- [ ] **Step 3: Run on a real iPhone.** Build the keyboard extension with the shim + a minimal bundled JP schema (e.g. RIME `luna_pinyin`-style sample swapped for a JP schema). Type "nihongo", confirm kanji candidates come back. Watch the extension's memory in Instruments.
  Expected: candidates returned; extension RSS stays well under the ~50–60 MB jetsam cap (with mmap'd dict). Record numbers.

- [ ] **Step 4: Minimal JNI bridge (Android).** Mirror the shim via `rime_bridge.cpp` + CMake; call from a throwaway instrumented test. Confirm candidates on a real Android phone.

- [ ] **Step 5: Checkpoint (GATE).** Write `rime-spike-findings.md`: build approach, library version, on-device memory numbers, candidate quality, and a clear **GO / NO-GO**. If NO-GO (OOM, won't build, poor candidates), STOP — revisit engine choice with the user before any further task.

- [ ] **Step 6: Commit**
```bash
git add DraftRightMobile/ios/RimeBridge DraftRightMobile/android/app/src/main/cpp docs/superpowers/notes/rime-spike-findings.md
git commit -m "spike(ime): prove librime bridge + candidates on iOS/Android (GATE)"
```

---

## Task 2: Pack manifest endpoint (backend — no engine dependency)

**Files:**
- Create: `backend/src/ime-packs/ime-packs.controller.ts`, `ime-packs.service.ts`, `ime-packs.module.ts`, `dto/pack.dto.ts`
- Test: `backend/test/ime-packs.e2e-spec.ts`
- Modify: `backend/src/app.module.ts` (register module)

- [ ] **Step 1: Write the failing test**
```typescript
// ime-packs.e2e-spec.ts
it('GET /ime-packs/manifest returns the japanese pack entry', async () => {
  const res = await request(app.getHttpServer()).get('/ime-packs/manifest').expect(200);
  const jp = res.body.packs.find((p) => p.id === 'ja');
  expect(jp).toMatchObject({ language: 'Japanese', sha256: expect.any(String), url: expect.stringContaining('.pack') });
  expect(typeof jp.size_bytes).toBe('number');
  expect(typeof jp.min_engine_version).toBe('number');
});
```

- [ ] **Step 2: Run it, verify it fails**
Run: `cd backend && npm run test:e2e -- ime-packs`
Expected: FAIL (route 404).

- [ ] **Step 3: Implement the manifest** (static config now; DB-backed later if needed)
```typescript
// ime-packs.service.ts
@Injectable()
export class ImePacksService {
  private readonly packs = [{
    id: 'ja', language: 'Japanese', version: 1, min_engine_version: 1,
    size_bytes: 0, // filled by a build step that hashes the real pack
    sha256: '', // filled at publish time
    url: 'https://draftright.info/ime-packs/draftright-ime-ja-v1.pack',
  }];
  list() { return { packs: this.packs }; }
}
// ime-packs.controller.ts
@Controller('ime-packs')
export class ImePacksController {
  constructor(private readonly svc: ImePacksService) {}
  @Get('manifest') manifest() { return this.svc.list(); }
}
```

- [ ] **Step 4: Run test, verify pass** (after seeding a dummy sha256/size in the test fixture)
Run: `cd backend && npm run test:e2e -- ime-packs`
Expected: PASS.

- [ ] **Step 5: Commit**
```bash
git add backend/src/ime-packs backend/src/app.module.ts backend/test/ime-packs.e2e-spec.ts
git commit -m "feat(backend): IME pack manifest endpoint"
```

---

## Task 3: Romaji→kana composer (headless, no engine dependency)

A deterministic romaji→kana mapping (RIME consumes kana or romaji depending on schema; we normalize to a clean composing buffer). Pure logic, fully unit-testable, in the existing Core.

**Files:**
- Create: `…/DraftRightKeyboardCore/Sources/DraftRightKeyboardCore/IME/RomajiComposer.swift`
- Test: `…/DraftRightKeyboardCoreTests/RomajiComposerTests.swift`
- (Android mirror: `…/keyboard/ime/RomajiComposer.kt` + `RomajiComposerTest.kt`)

- [ ] **Step 1: Write the failing test**
```swift
func test_basic_romaji_to_hiragana() {
  let c = RomajiComposer()
  XCTAssertEqual(c.feed("nihongo"), "にほんご")
  XCTAssertEqual(c.feed("tt"), "っt")     // sokuon: doubled consonant
  XCTAssertEqual(c.feed("n'"), "ん")       // explicit n
}
```

- [ ] **Step 2: Run it, verify it fails**
Run: `cd DraftRightMobile/ios/DraftRightKeyboardCore && swift test --filter RomajiComposerTests`
Expected: FAIL (type not found).

- [ ] **Step 3: Implement** the romaji→kana table + sokuon/ん rules (mirror the existing `TelexComposer` structure: a `feed`/`reset`/`currentComposingText` surface conforming to `Composer`).

- [ ] **Step 4: Run tests, verify pass**
Run: `swift test --filter RomajiComposerTests`
Expected: PASS.

- [ ] **Step 5: Port + test on Kotlin** (`RomajiComposer.kt`, `RomajiComposerTest.kt`); run `./gradlew :app:testDebugUnitTest --tests '*RomajiComposer*'`.

- [ ] **Step 6: Commit**
```bash
git add DraftRightMobile/ios/.../RomajiComposer.swift DraftRightMobile/ios/.../RomajiComposerTests.swift DraftRightMobile/android/.../RomajiComposer.kt DraftRightMobile/android/.../RomajiComposerTest.kt
git commit -m "feat(ime): romaji→kana composer (iOS + Android, headless-tested)"
```

---

## Task 4: CandidateController (depends on Task 1) — headless candidate state

Wraps the RIME engine behind a testable seam: feed composing text → ordered candidates; select index → committed string + reset.

**Files:**
- Create: `…/DraftRightKeyboardCore/.../IME/CandidateController.swift` (+ Kotlin mirror)
- Create: `…/IME/RimeEngine.swift` protocol + the bridge-backed impl from Task 1 (+ Kotlin)
- Test: `…/DraftRightKeyboardCoreTests/CandidateControllerTests.swift` (with a fake `RimeEngine`)

- [ ] **Step 1: Write the failing test** (against a fake engine, so it's hermetic)
```swift
func test_select_commits_and_resets() {
  let fake = FakeRimeEngine(candidates: ["日本語", "にほんご"])
  let cc = CandidateController(engine: fake)
  cc.input("nihongo")
  XCTAssertEqual(cc.candidates, ["日本語", "にほんご"])
  XCTAssertEqual(cc.select(0), "日本語")
  XCTAssertTrue(cc.candidates.isEmpty)   // reset after commit
}
```

- [ ] **Step 2: Run it, verify it fails**
Run: `swift test --filter CandidateControllerTests`
Expected: FAIL.

- [ ] **Step 3: Implement** `RimeEngine` protocol (`func candidates(for: String) -> [String]`), `CandidateController` (input/candidates/select/reset), and the real bridge-backed `RimeEngine` using the Task 1 shim.

- [ ] **Step 4: Run tests, verify pass**
Run: `swift test --filter CandidateControllerTests`
Expected: PASS.

- [ ] **Step 5: Port + test on Kotlin.**

- [ ] **Step 6: Commit**
```bash
git commit -am "feat(ime): CandidateController over RimeEngine seam (iOS + Android)"
```

---

## Task 5: Pack download/install/mmap (host app + shared storage)

**Files:**
- Create: `DraftRightMobile/lib/services/ime_pack_service.dart` (download, verify sha256, atomic install, remove)
- Create: iOS `Runner/ImePackChannel.swift` (writes pack to App Group container), Android `…/ImePackChannel.kt` (writes to files dir)
- Test: `DraftRightMobile/test/ime_pack_service_test.dart`

- [ ] **Step 1: Write the failing test** (mock HTTP + temp dir)
```dart
test('downloads, verifies sha256, installs atomically', () async {
  final svc = ImePackService(httpClient: fakeClient, storage: tempStorage);
  final path = await svc.install(packId: 'ja', url: u, sha256: goodHash, sizeBytes: n);
  expect(File(path).existsSync(), isTrue);
});
test('rejects a pack whose hash mismatches', () async {
  expect(() => svc.install(packId: 'ja', url: u, sha256: 'bad', sizeBytes: n), throwsA(isA<PackIntegrityError>()));
});
```

- [ ] **Step 2: Run it, verify it fails**
Run: `cd DraftRightMobile && flutter test test/ime_pack_service_test.dart`
Expected: FAIL.

- [ ] **Step 3: Implement** `ImePackService`: stream download to temp, hash while writing, compare sha256, move into the shared dir via the platform channel; `remove(packId)` deletes it. The shared dir is the App Group (iOS) / files dir (Android) so the keyboard can read it.

- [ ] **Step 4: Run tests, verify pass**
Run: `flutter test test/ime_pack_service_test.dart`
Expected: PASS.

- [ ] **Step 5: Commit**
```bash
git commit -am "feat(ime): downloadable language-pack service + shared-storage install"
```

---

## Task 6: Candidate bar UI + keyboard wiring (depends on Tasks 4, 5)

**Files:**
- Create: iOS `DraftRightKeyboard/CandidateBarView.swift`, Android `keyboard/CandidateBarView.kt`
- Modify: iOS `KeyboardViewController.swift` (host bar; route keys via `RomajiComposer`→`CandidateController` when Japanese active; commit on tap/space/number), Android `QwertyKeyboardView.kt` + `DraftRightIME.kt`
- Modify: `LanguagePack.swift` / Kotlin `LanguageRegistry` — Japanese pack descriptor (requires data pack; uses candidate flow)

- [ ] **Step 1: Write the failing test (XCUITest, extends the existing harness).** Add a Japanese case to `KBUITests/TelexTypingUITests.swift` (or a new `JapaneseTypingUITests.swift`): with the JP pack present on the sim, type `nihongo`, tap the first candidate, assert the field contains `日本語`.
```swift
func test_japanese_romaji_candidate_commit() {
  launchOnDraftRight(lang: "ja")
  type("nihongo")
  let cand = app.buttons["dr_candidate_0"].firstMatch
  XCTAssertTrue(cand.waitForExistence(timeout: 5))
  cand.tap()
  XCTAssertEqual(field.value as? String, "日本語")
}
```

- [ ] **Step 2: Run it, verify it fails**
Run: `ios/KBUITests/run-ui-tests.sh <udid>` (note: must build the JP appex for the sim + place the JP pack in the App Group, per the iOS-26 harness notes).
Expected: FAIL (no candidate bar / `dr_candidate_0` missing).

- [ ] **Step 3: Implement** the candidate bar (renders `CandidateController.candidates`, each as `dr_candidate_<i>`), wire Japanese key routing in the keyboard, add the JP language-pack descriptor, and gate Japanese availability on the pack being installed.

- [ ] **Step 4: Run test, verify pass**
Run: `ios/KBUITests/run-ui-tests.sh <udid>`
Expected: PASS.

- [ ] **Step 5: Port + verify on Android** (instrumented or manual on a real device).

- [ ] **Step 6: Commit**
```bash
git commit -am "feat(ime): candidate bar + Japanese keyboard wiring (iOS + Android)"
```

---

## Task 7: Settings UI — enable/download/remove Japanese (host app)

**Files:**
- Modify: `DraftRightMobile/lib/screens/settings_screen.dart` (Languages section: toggle Japanese → prompt download via `ImePackService` with size + progress; "Remove data")
- Test: widget test `DraftRightMobile/test/settings_language_pack_test.dart`

- [ ] **Step 1: Write the failing widget test** — enabling Japanese with no pack shows a "Download (≈18 MB)" affordance; tapping it calls `ImePackService.install`.
```dart
testWidgets('enabling Japanese offers download', (t) async {
  await t.pumpWidget(wrap(SettingsScreen(), packService: fakeSvc));
  await t.tap(find.text('日本語'));
  await t.pumpAndSettle();
  expect(find.textContaining('Download'), findsOneWidget);
});
```

- [ ] **Step 2: Run it, verify it fails**
Run: `flutter test test/settings_language_pack_test.dart`
Expected: FAIL.

- [ ] **Step 3: Implement** the Languages section: list available packs from the manifest, show installed/needs-download state, download with progress, remove-data action.

- [ ] **Step 4: Run test, verify pass**
Run: `flutter test test/settings_language_pack_test.dart`
Expected: PASS.

- [ ] **Step 5: Commit**
```bash
git commit -am "feat(ime): settings UI to download/remove Japanese language pack"
```

---

## Task 8: Build + publish the Japanese pack; end-to-end verify

**Files:**
- Create: `scripts/build-ime-pack.sh` (compiles the RIME JP schema+dict into `draftright-ime-ja-v1.pack`, prints sha256 + size)
- Modify: backend manifest fixture with the real sha256/size; deploy pack to `draftright.info/ime-packs/`

- [ ] **Step 1:** Build the JP pack with `scripts/build-ime-pack.sh`; record sha256 + bytes. Vet the dictionary's data license; note it in `rime-spike-findings.md`.
- [ ] **Step 2:** Publish the pack to the droplet under `/var/www/draftright/ime-packs/` (reuse `release-publish` patterns; do NOT touch `downloads/`). Update the manifest service with the real hash/size; deploy backend.
- [ ] **Step 3:** On a **real iPhone + Android phone**: enable Japanese → pack downloads → type `nihongo` → 日本語 candidate → commit → run a tone rewrite on the committed Japanese text (confirm the existing rewrite path still works).
- [ ] **Step 4:** Confirm base app size is unchanged for users who never enable Japanese; confirm iOS extension memory stays under budget while Japanese active.
- [ ] **Step 5: Commit**
```bash
git add scripts/build-ime-pack.sh backend/src/ime-packs
git commit -m "feat(ime): build + publish Japanese pack; e2e verified on device"
```

---

## Self-Review

- **Spec coverage:** engine=RIME (T1,T4); on-device/no-network typing (T4,T6); downloadable self-hosted packs + manifest (T2,T5,T8); App Group/files-dir + mmap (T1,T5,T6); romaji-QWERTY composer (T3); candidate bar (T6); settings download/remove (T7); framework-ready for ZH/KO (Composer/CandidateController/pack seams are language-agnostic). Korean/Chinese explicitly out of scope (separate phases) — matches spec.
- **Placeholders:** the manifest `sha256`/`size_bytes` are intentionally filled by the Task 8 build step (real artifact hashing) — not a plan placeholder, it's a publish-time value; flagged explicitly.
- **Gate:** Task 1 NO-GO halts before Tasks 4+, per spec's "validate the bridge first" risk mitigation.
- **Types:** `RimeEngine` (protocol, `candidates(for:)`), `CandidateController` (`input`/`candidates`/`select`/`reset`), `RomajiComposer` (`feed`/`reset`/`currentComposingText`, conforms to existing `Composer`), `ImePackService` (`install`/`remove`) — consistent across tasks.
