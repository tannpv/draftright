# Handoff / Progress — 2026-08-15 (rewrite triggers, macOS + Windows)

Committed to the repo so it transfers to any machine. (Local `~/.claude/projects/.../memory`
does **not** transfer — the repo-committed knowledge is the scoped `CLAUDE.md`
files + this doc.)

## State snapshot (updated 2026-08-16)

| Area | State |
|---|---|
| macOS | **2.3.65 LIVE**. Pencil trigger COMPLETE + user-verified: drag-only, requires real selection, drag-distance threshold (no jitter-click), composited above windows. #176-183 + #187 closed. |
| Windows pencil | On develop+main, **compile-verified only, STILL UNVERIFIED on Windows** (#180). |
| Windows update fixes | **#184** (Store update-click HWND) + **#186** (hotkey thread-affinity + clipboard diagnostics) merged develop+main, shipped. |
| Windows Store | **MSIX 2.3.64** (carries #184+#186+pencil) uploaded to Partner Center 2026-08-16 — in pre-processing/cert. HOLD go-live until pencil tested. Store-live was 2.3.52. |
| Windows sideload/update-server | at **2.3.63** (the `v2.3.63` tag published the pencil to sideload). 2.3.64 NOT tagged (no sideload push of the hotkey fix yet). |
| Linux pencil (#188) | On develop, **UNVERIFIED** (no X11 host; `gi` not on the dev Mac → GTK paths uncompilable here). **X11-only** (Wayland can't). Reuses PRIMARY-selection polling → the existing RewritePanel (no overlay/no synthetic copy). Pure enum + decision unit-tested (9 tests). See `DraftRightLinux/CLAUDE.md` → "Rewrite trigger". |

Branches: `develop` = integration, `main` = release. macOS releases tag `macos-vX.Y.Z`
(do NOT build Windows). Windows Store release tags `vX.Y.Z` (triggers Windows CI publish
to the update server). A branch push builds artifacts only (no publish).

Branches: `develop` = integration, `main` = release. macOS releases tag `macos-vX.Y.Z`
(do NOT build Windows). Windows Store release tags `vX.Y.Z` (triggers Windows CI publish).

## Rewrite trigger — what shipped

Two mutually-exclusive triggers via a `TriggerMode` enum (`{pencil, hotkey}`),
decoupled from the hotkey combo. Settings → Trigger picker. Design + the hard-won
rules are in the scoped docs:
- macOS: `DraftRight/CLAUDE.md` → "Rewrite trigger" section.
- Windows: `DraftRightWindows/CLAUDE.md` → "Rewrite trigger — Pencil / Hotkey" section.

Rules (both platforms): pencil shows on a **drag that selected text only** (never a
click/double-click); **never synthesize ⌘C/Ctrl+C on selection** — only on a
deliberate click/hotkey (it broke the user's own Copy, macOS #178); Terminal/Electron
are AX/UIA-blind → a drag gesture is the only signal there.

macOS bug saga (all shipped): #176 reversible switch, #177 Terminal drag flag,
#178 copy-break, #179/#180 drag-only + removed "Both", #181 require selection,
#182 pencil position/on-screen clamp.

## OPEN TASKS (the plan)

0. **macOS pencil — DONE + user-verified** (2.3.65; #176-#183 + #187 closed). Full behavior:
   drag-only (no click/double-click), requires a real selection, drag-distance threshold
   (`dragThresholdPoints=6`, no jitter-click), `.statusBar` level so it composites above the
   target app. Nothing open. Pure decisions unit-tested: `shouldShowPencil` + `isDragGesture`.
1. **Windows pencil debug loop** (tracked in **#180**) — the engine has never run. Test via the
   Store MSIX **2.3.64** (uploaded, in cert) or a sideload build. **#186 added clipboard-capture
   diagnostics** (clipboard sequence number across the synthetic Ctrl+C, capture reason) — read
   `%LOCALAPPDATA%\DraftRight\Logs\draftright.log` (`Pencil:` + `GetSelectedTextAsync` lines) →
   fix → rebuild. Risk: a buggy `WH_MOUSE_LL` hook can freeze system input. **Do NOT let the
   Store build go live until tested.** LESSON: `RegisterHotKey`/`UnregisterHotKey` are
   **thread-affine** — the pencil work's `ApplyTriggerMode`-from-Settings first ran unregister
   on the wrong thread (#186 fix routes it via `App.OnHotkeyThread`). Any Win32 register/unregister
   from a non-owner thread must be marshalled.
3. **Windows pencil — missing vs macOS** — add the #181 "require actual selection" check
   (needs UI Automation `TextPattern.GetSelection`) and the #182 on-screen clamp. Currently
   position = cursor, text grabbed by Ctrl+C on click.
4. **Windows in-app update click does nothing — FIXED (PR #184, merged).** It was a
   **Store/MSIX** install, not sideload: `StoreContext` in a WinUI 3 desktop process needs
   `IInitializeWithWindow` HWND binding or `RequestDownloadAndInstallStorePackageUpdatesAsync`
   throws `0x80070578`, caught + logged silently. PR binds the HWND + adds a Store fallback,
   and stops advertising the Store's untrusted package version (that caused "Update 2.3.52
   available while on 2.3.52"). Needs a Windows Store re-cut to reach users; reconcile the
   version (PR added CHANGELOG `## 2.3.63 / ### Windows` but csproj is 2.3.61 — bump to match).

## How to resume

- **macOS release:** `scripts/release.sh macos <version> --yes` (builds notarized DMG →
  merges → tags `macos-v*` → publishes). NOTE: its final `/updates/latest` poll
  **false-times-out (exit 1) every time** — the release still publishes; verify with
  `curl -s "https://api.draftright.info/updates/latest?platform=mac"` + compare the served
  DMG sha, then `scripts/deploy-website.sh` (download-card version drifts until then).
- **Windows Store build:** bump `DraftRightWindows/DraftRightWindows.csproj <Version>` only
  (CI stamps the MSIX manifest, #170) + a `## <ver> / ### Windows` CHANGELOG entry, push
  `develop`/`main` (branch push builds artifacts, no auto-publish), download the
  `DraftRight-MSIX-Store` `.msixupload` artifact, upload manually in Partner Center.
- **macOS pencil debug:** read `~/Library/Logs/DraftRight/draftright.log` (`[MONITOR]` lines)
  directly on the maintainer's Mac. Windows debug: the user sends the Windows log.
- **Key files:** macOS `DraftRight/TriggerMode.swift`, `DraftRight/Accessibility/{SelectionMonitor,AXTextService}.swift`.
  Windows `DraftRightWindows/DraftRightWindows/Models/TriggerMode.cs`,
  `Services/{PencilTrigger,PencilTriggerDecision}.cs`, `Views/PencilOverlayForm.cs`,
  `App.cs` (`ApplyTriggerMode`, `HandleTriggerAsync`).
