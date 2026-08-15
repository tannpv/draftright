# Handoff / Progress — 2026-08-15 (rewrite triggers, macOS + Windows)

Committed to the repo so it transfers to any machine. (Local `~/.claude/projects/.../memory`
does **not** transfer — the repo-committed knowledge is the scoped `CLAUDE.md`
files + this doc.)

## State snapshot

| Area | State |
|---|---|
| macOS | **2.3.62 LIVE** (Developer ID signed + notarized). Pencil trigger complete. |
| Windows | Pencil engine on `develop` + `main`, **compile-verified only, NEVER run on Windows**. Store live = **2.3.52**. |
| Windows Store MSIX | **2.3.61** built (CI run `31876169928`, commit `6406fa4a`) for manual Partner Center upload. Contains the pencil. |

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

0. **macOS pencil — DONE + user-verified** (2.3.63, works in Terminal; #176-#183 closed).
   Final fix: raise the pencil `NSPanel` to `.statusBar` so it isn't composited behind
   the target app (#183). Nothing open here.
1. **Windows pencil debug loop** (tracked in **#180**) — the engine has never run. Test via the sideload build
   (CI run `31874220687` → artifact `DraftRight-Setup-win-x64`, unsigned → SmartScreen/SAC
   bypass). Read `%LOCALAPPDATA%\DraftRight\Logs\draftright.log` (`Pencil:` lines) → fix →
   rebuild. Risk: a buggy `WH_MOUSE_LL` hook can freeze system input. **Do NOT ship the
   Store version live until tested.**
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
