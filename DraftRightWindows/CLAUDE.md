# DraftRight Windows

WinUI 3 application shell (WindowsAppSDK) whose windows are **WPF UI** — the
WinForms→WPF migration (#156) is complete. A few surfaces stay WinForms by
design (below). C# 12 / .NET 8, MSIX for the Store, Inno Setup for direct download.

## UI toolkit — decided 2026-08-08, do not relitigate

| Toolkit | Verdict |
|---|---|
| **WPF UI** (lepoco `WPF-UI` 4.3.0) | ✅ **adopted** — renders, themes, runs in-process with WindowsAppSDK |
| WinUI 3 XAML | ❌ cannot render on unpackaged builds |
| WinForms | retained only for the app message loop, tray, screen capture, clipboard, MessageBox, and `LoadingIndicator` |

**WinUI 3 was tried twice and failed both times.** Unpackaged builds have no
usable `resources.pri`, so XAML theme lookups fail:

- first attempt: `STATUS_STOWED_EXCEPTION 0xc000027b` — *"Cannot find a Resource
  … TabViewScrollButtonBackground"*
- after `draftright.iss` began regenerating a PRI post-install: `COMException`
  **"element not found"** — one layer further, same wall

The MSIX/Store build *does* get a genuine PRI and never had the problem, so the
answer is channel-specific. Direct download is what rules WinUI out.

### WPF gotcha that costs an afternoon

This process has **no `App.xaml`** — it is a WindowsAppSDK app with no XAML
files at all. Nothing merges WPF-UI's resource dictionaries globally, so every
control resolves to no `ControlTemplate` and the window paints as a **bare white
rectangle** (bug report #58). Every WPF window must merge them itself:

```csharp
Resources.MergedDictionaries.Add(
    new Wpf.Ui.Markup.ThemesDictionary { Theme = Wpf.Ui.Appearance.ApplicationTheme.Dark });
Resources.MergedDictionaries.Add(new Wpf.Ui.Markup.ControlsDictionary());
```

Per-window rather than app-wide is deliberate: it keeps the WinForms surfaces
that stay (LoadingIndicator, tray) unstyled by it. This is now centralised in
`FluentWindowBase` — every WPF window derives it and gets the dictionaries, the
opaque dark background, backdrop-off, and a draggable title bar; new windows do
not re-merge them.

### Migration history (#156) — all shipped to prod

| Phase | Surface | Shipped |
|---|---|---|
| 1 | Rewrite panel + grammar + diff | 2.3.41–43 |
| 2 | `ReportBugDialog` (#158) | 2.3.44 |
| 3 | Settings + Subscription (#159) | 2.3.45 |
| 4 | Suggest / What's New / QR / Bank / status banner (#160) | 2.3.46 |
| 5 | WinForms leftovers removed (#161) | — nothing left to delete; only the by-design WinForms below remains |

**When adding a NEW WPF window, do the RULE #1 pass first** — clean, reusable,
extendable, no hardcoding. Concretely:

1. What does this view **restate** that a model or service already owns? Icons,
   labels, colours, size limits, lists of things. Read from the source instead.
   The tone grid's hardcoded array is the cautionary case: it shadowed the enum
   AND ignored a setting, and hid two tones for months.
2. What can it **reuse**? ViewModels, `Theme`, `GrammarFixer`, `WordDiff`. If
   the port is tempted to reimplement logic, that is a separate commit.
3. Would a **third case** fit? One more tone, one more issue type, one more
   payment method — does it drop in via data, or need another copy-paste?
4. Any **literal** that carries meaning? Name it. Layout coordinates may stay
   inline; sizes, timings and limits may not.

Then: one panel per PR, ViewModels never change, `Theme.cs` stays the single
palette source (`Color` for WinForms, `WpfBrush()` for WPF), and every panel is
run on Windows before its PR merges.

**Not migrating:** `LoadingIndicator` (click-through Win32 window styles, WPF
gains nothing) and the tray icon (`H.NotifyIcon.WinUI`).

## Building from macOS

The Windows app compiles on an Apple-silicon Mac — useful for catching type
errors without CI:

```bash
dotnet build DraftRightWindows/DraftRightWindows.csproj \
  -p:EnableWindowsTargeting=true -p:WindowsAppSDKSelfContained=false
```

Both flags are required; each alone fails with a different error. It only
**compiles** — nothing here can run a WinForms or WPF window, which is why
layout and threading bugs reach users. Two shipped that way: a capture button
painted behind a label, and a tone grid that hid two tones.

## Tests

`DraftRightWindows.PureTests` (net8.0, **no ProjectReference**) is the CI gate.
`DraftRightWindows.Tests` references the WinUI app assembly and hangs headless
discovery, so it never runs in CI. See the root `reference_windows_headless_tests`
memory. Adding a `ProjectReference` to PureTests reintroduces the hang.

## Releases

Full step-by-step runbook (all platforms, incl. the client auto-update
mechanism and verification): **`docs/release-runbook.md`**.

Tag `v<semver>` on `main`; CI builds x64 + arm64 + MSIX and publishes to the
update server. `.csproj <Version>` drives the built exe's ProductVersion, which
the installer reads; `Package.appxmanifest` drives the MSIX — **bump both**.

The published artifact's SHA-256 is recorded at publish time and the release
**fails** if `/updates/latest` serves an empty or mismatched hash (#22).

**The installer is unsigned** (#154) — the real auto-update blocker. Smart App
Control blocks the install launch: "An Application Control policy has blocked
this file." SAC has 3 states (Off / Evaluation=doesn't block / On=enforced); it
flips itself to enforced and only reverts on a clean Windows reinstall — so
"updates worked before" was luck, not stable. Per-file Unblock is for SmartScreen
NOT SAC; the reliable user workaround is turning SAC Off.

CI signing is **wired but PFX-shaped and needs rework** — `installer/sign-file.ps1`
+ `build-windows.yml`, gated on `WINDOWS_SIGNING_PFX_BASE64` + `_PASSWORD`. **Since
June 2023 there is no downloadable PFX** (CA/B Forum — key must be on FIPS hardware
or a cloud HSM), so this wiring can't sign a modern OV/EV cert as-is. For CI signing,
buy a **cloud-signing** cert and move the script to its CLI (eSigner/KeyLocker).

**Cert path: OV/EV + cloud signing, not Azure.** Azure Trusted Signing is NOT
available for Vietnam. **OV does NOT guarantee a Smart App Control pass** — it clears
SmartScreen "unknown publisher" + builds reputation; the Store is the only guaranteed
clean install (already shipped). Full purchase + wire-up runbook:
**`docs/windows-code-signing-cert-purchase.md`**.

### Ship a Windows change to the Store (the release trigger)
Owner just says **"release windows to store"** — no steps to memorize. The flow:
1. Land the fix/feature on `develop` (feature branch → `--no-ff`).
2. Bump `DraftRightWindows.csproj` `<Version>` (e.g. 2.3.52 → 2.3.53). **Do not**
   hand-edit `Package.appxmanifest` — CI stamps it from the csproj (#170).
3. Tag `v<semver>` on `main` → CI builds x64 + arm64 + **MSIX**.
4. `scripts/store-package.sh` stages the `.msixupload` → owner uploads it manually
   in Partner Center (API automation still pending an Azure AD link).
A re-upload of an unchanged version is rejected by the Store — there must be a real
change + a version bump. If nothing changed, there is nothing to ship.

## New WPF windows — conventions

(The migration is done — see "Migration history" above. These are the rules
every shipped window follows; apply them to any new one.)
- derive **`FluentWindowBase`** (owns theme dictionaries #58 + opaque dark bg +
  `WindowBackdropType.None` #163 — never set a Mica backdrop);
- set content via **`FluentWindowBase.SetBody(body, showMaximize)`**, never
  `Content =` directly — SetBody adds the draggable `Wpf.Ui` TitleBar (#167);
- launch detached windows via **`Services.StaWindowHost.Run`** (STA thread +
  crash handler + Dispatcher.Run — the crash handler is what keeps a WPF
  UI-thread exception from killing the app, #166);
- build fields with **`Views.FluentFormControls`** (one form vocabulary + button
  factory); feedback dialogs share **`FeedbackDialogBase`**.

## Known traps

- **Update dialog buttons must act immediately.** "Continue in background" used
  to wait on the download task unwinding, which at 100% is still hashing 134 MB
  for the integrity check — so the window sat there ignoring the click (#153).
- **WinForms z-order paints lower-index controls on top.** A label added before
  a button, overlapping it, hides the button completely. That is how the capture
  button shipped invisible.
- **A hardcoded list beside an enum becomes the real definition.** The tone grid
  restated icons and labels the enum already owned, listed six of eight, and
  ignored `Settings.EnabledTones` — so Grammar Check was unreachable and its
  Settings toggle did nothing.
