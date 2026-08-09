# DraftRight Windows

WinUI 3 application shell (WindowsAppSDK) whose panels are **WPF UI**, migrating
off WinForms. C# 12 / .NET 8, MSIX for the Store, Inno Setup for direct download.

## UI toolkit — decided 2026-08-08, do not relitigate

| Toolkit | Verdict |
|---|---|
| **WPF UI** (lepoco `WPF-UI` 4.3.0) | ✅ **adopted** — renders, themes, runs in-process with WindowsAppSDK |
| WinUI 3 XAML | ❌ cannot render on unpackaged builds |
| WinForms | legacy, being replaced (#156) |

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

Per-window rather than app-wide is deliberate: it keeps the remaining WinForms
surfaces unstyled by it during the migration.

### Migration state (#156)

| Phase | Surface | Status |
|---|---|---|
| 1 | Rewrite panel + grammar + diff | built, **unrun** |
| 2 | `ReportBugDialog` | not started |
| 3 | `SettingsFormBuilder` + `SubscriptionTab` | not started |
| 4 | `SuggestFeatureDialog` + payment/info dialogs | not started |
| 5 | Delete WinForms leftovers | not started |

**Before writing a line of any phase, do the RULE #1 pass** — clean, reusable,
extendable, no hardcoding. For a panel migration that means, concretely:

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

**The installer is unsigned** (#154). Smart App Control blocks it outright —
"An Application Control policy has blocked this file" — so direct-download users
must Unblock the file manually. The Store build is signed by Microsoft and
sidesteps it. Unresolved; needs a certificate decision.

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
