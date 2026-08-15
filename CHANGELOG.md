# DraftRight — Changelog

User-facing release notes. **One `## <version>` section per release**, with
**per-platform sub-sections** so each store/installer only sees notes that
apply to it.

```
## 2.3.X — YYYY-MM-DD
### macOS
- mac-only bullet
### Windows
- windows-only bullet
### All platforms
- cross-cutting bullet
```

The release pipeline (`scripts/release-publish.sh`) passes the target platform
to `scripts/changelog-extract.sh`, which keeps only the matching sub-section
plus `### All platforms` and writes the result into `app_releases.release_notes`.
A Windows-only line under `### Windows` will never appear in the macOS
"What's New" notice (and vice versa).

If a version needs the user to *do* something after updating, say so
explicitly under an **Action needed:** line inside the relevant sub-section.

## 2.3.56 — 2026-08-15
### macOS
- Fixed: selecting text no longer interferes with Copy (⌘C). The rewrite pencil now only reads your selection when you actually click it, so your own copying works normally.

## 2.3.55 — 2026-08-15
### macOS
- The rewrite pencil now appears when you select text in more apps — including Terminal — not just apps that hand their selection to macOS. Selecting by drag or by double/triple-click now shows the pencil reliably.

## 2.3.54 — 2026-08-15
### macOS
- Switching the rewrite trigger to the pencil button no longer forgets your keyboard shortcut. Pencil mode now shows a one-click button to switch back to your previous shortcut, instead of making you set it up again.

## 2.3.53 — 2026-08-15
### macOS
- Secure auto-updates: a downloaded update is now verified against a published checksum before it installs, and a corrupt or tampered download is refused instead of run.
- You can now attach a screenshot to a bug report with one click.
- Fixed a crash in the diff view that could happen on certain edits.
- Repeating the same text and tone is now faster — the result is served instantly instead of asking the server again.

## 2.3.52 — 2026-08-11
### Windows
- Fixed an occasional error when installing an update. The app now retries automatically, so updates install reliably without you having to try again.

## 2.3.51 — 2026-08-11
### Windows
- Added an internal self-check that reports if a window ever fails to display correctly, so we can catch and fix display problems faster. No visible change.

## 2.3.50 — 2026-08-10
### Windows
- The Submit, Sign In and other primary buttons are now brand blue instead of black.
- Prepared the Microsoft Store build (package version brought back in step with the app version).

## 2.3.49 — 2026-08-10
### Windows
- Fixed the app crashing and vanishing from the taskbar when you opened Report a Bug (or another window) after having opened Settings. All windows now open reliably.
- When Windows Smart App Control blocks an update (because the app isn't code-signed yet), DraftRight now explains what happened and offers to open Windows Security so you can turn it off, instead of just showing a generic error.

## 2.3.48 — 2026-08-10
### Windows
- Every window (Settings, Report a Bug, Suggest a Feature, the payment dialogs) can now be dragged by its title bar.
- Fixed the Settings window still opening at full height on scaled displays (e.g. 150% on a 1080p laptop), where the close button could end up off-screen. It now always fits your screen, opens centred, and stays closable.

## 2.3.47 — 2026-08-10
### Windows
- Under-the-hood cleanup: the app now creates its windows and buttons through one shared path, so the crash protection added in 2.3.46 applies uniformly to every window. No visible changes.

## 2.3.46 — 2026-08-10
### Windows
- Fixed the Settings window opening taller than the screen on some displays, which could hide the close button. It now always fits your screen and scrolls within each tab.
- Report a Bug, Suggest a Feature and the payment dialogs are rebuilt on the modern Windows (Fluent) UI, matching the rest of the app.
- More crash-resistant: a problem inside one window no longer closes the whole app, and crashes are now recorded (to a file on your Desktop and sent to us) so we can find and fix them faster.

## 2.3.45 — 2026-08-10
### Windows
- Settings is rebuilt on the modern Windows (Fluent) UI. All six tabs — General, Rewrite, Trigger, Account, Subscription and Advanced — are refreshed with cleaner spacing and a solid dark background, and the window now resizes cleanly. Every setting works as before.

## 2.3.44 — 2026-08-10
### Windows
- The Report a Bug form is rebuilt on the modern Windows (Fluent) UI, matching the rewrite panel. Same four ways to attach a screenshot — browse, drag & drop, paste (Ctrl+V), and Capture screen — with cleaner spacing and a solid dark background.

## 2.3.43 — 2026-08-09
### Windows
- Fixed the rewrite panel still looking see-through and washed out on Windows 11 with a bright wallpaper — the desktop showed through the panel and the text was hard to read. The panel now always uses a solid dark background instead of the translucent effect, so it stays readable on every setup.

## 2.3.42 — 2026-08-09
### Windows
- Fixed the new rewrite panel showing washed-out, hard-to-read text on some setups (Windows 10, virtual machines, or with transparency effects turned off). The panel now uses a solid dark background so the text is always legible.

## 2.3.41 — 2026-08-09
### Windows
- The rewrite panel has a fresh look, rebuilt on the modern Windows (Fluent) UI — the same rewrite, tones, diff and grammar-check you already use, with cleaner styling. This is the first surface to move over; the rest follow.

## 2.3.40 — 2026-08-08
### Windows
- Removed the experimental test window. It answered the question it was there for: DraftRight can move to the modern Windows look, and that work starts next.

## 2.3.39 — 2026-08-08
### Windows
- Fixed the experimental WPF test window opening blank, and the clipped button and description text next to it. Thanks for the bug report — the screenshot you attached is what diagnosed it.

## 2.3.38 — 2026-08-08
### Windows
- Added a WPF test window under Settings → Advanced → Experimental, checking whether DraftRight can move to the modern Windows look. This is the second toolkit being tried; the first could not render. It touches nothing else and will be removed once we know.

## 2.3.37 — 2026-08-08
### Windows
- Fixed the "Capture screen" button on the bug report form being invisible. It was drawn underneath the filename label, so it had been unusable since it was added.
- Removed the experimental test window added in 2.3.36. It had done its job.

## 2.3.36 — 2026-08-08
### Windows
- Added a test window under Settings → Advanced → Experimental. It checks whether DraftRight can move to the modern Windows look instead of the current panels. It does nothing to your settings or your text — it just draws some controls so we can see whether they render. It will be removed in a later release.

## 2.3.35 — 2026-08-08
### Windows
- The bug report form can now take the screenshot for you. Click "Capture screen" and DraftRight briefly hides itself, grabs the screen and attaches it — no need to alt-tab to a snipping tool, which often disturbs whatever you were trying to show us in the first place. Large screens are scaled down automatically so the report still sends.

## 2.3.34 — 2026-08-08
### Windows
- Added "Report a Bug…" to the tray menu. It was previously tucked away in Settings under the Advanced tab, which meant hunting for it at exactly the moment something had gone wrong.

## 2.3.33 — 2026-08-08
### Windows
- Grammar Check now shows its results. Run it and you get a list of what it found — spelling, grammar and style each colour-coded — with the correction next to the original and the reason underneath. Fix them one at a time, or use Fix All. Corrections land on the word they were meant for even when an earlier fix has changed the length of the sentence, and anything already fixed by an overlapping correction quietly drops off the list instead of being applied twice.
- Added a Diff button that shows your original and the rewrite side by side, with the removed words marked on the left and the added ones on the right. It appears once a rewrite exists, and toggles back to the normal view.
- The "Continue in background" button on the update window now closes it straight away. Previously, if you clicked it once the download had reached 100%, the window sat there still showing 100% while the installer was being checked — so it looked like the click had done nothing. The download continues in the background either way; you'll get the "ready to install" notice when it lands.

## 2.3.32 — 2026-08-06
### Windows
- Repeating a rewrite you already asked for is now instant. Ask for the same text in the same tone again — after switching tones and back, or reopening the panel — and the previous result appears immediately with no waiting and no round trip to the server. Translations are kept separately per target language, so changing the language always fetches a fresh translation.
- Updates are now checked against a published checksum before they run. DraftRight compares the installer it downloaded against the fingerprint published alongside the release, and refuses to run it if they do not match. Previously the download was trusted on the strength of the HTTPS connection alone.

## 2.3.31 — 2026-07-23
### Windows
- Fixed the rewrite shortcut (Ctrl+Shift+R) sometimes doing nothing at all after you selected text. When Windows blocks DraftRight from copying your highlighted text — usually because the active window is running as administrator — the app now tells you what happened instead of failing silently, and you can copy the text yourself (Ctrl+C) then press the shortcut. If nothing is selected, or the panel fails to open, you now get a clear tray message explaining why.

## 2.3.30 — 2026-07-22
### macOS
- Fixed Grammar Check scrambling your text when applying fixes (letters spliced into the middle of words). Corrections are now located by their actual content, so every "Fix" and "Fix All" lands exactly on the intended word — including in Vietnamese text.

## 2.3.29 — 2026-07-20
### macOS
- Fixed a second DraftRight icon sometimes appearing in the menu bar. When launch-at-login was on, the background helper and a manual launch could start two copies; the app now keeps a single instance.

## 2.3.28 — 2026-07-20
### macOS
- Fixed the rewrite shortcut not working in Terminal. Recent macOS versions stop apps from reading a Terminal selection automatically, so DraftRight now uses the text you copy: in Terminal, press ⌘C first, then your rewrite shortcut. Every other app still works with just the shortcut (no copy needed).

## 2.3.27 — 2026-07-20
### Windows
- Fixed the DraftRight icon showing blank on the taskbar and in the system tray (and on the Settings / bug-report dialogs). The icon now loads reliably regardless of how the app was installed.

## 2.3.26 — 2026-07-20
### Windows
- Sign-in errors now read in plain language. When the server rejects a login (for example a disabled account), the app shows the actual reason instead of a raw block of code/JSON.
- The "Downloading DraftRight" window now shows a real progress bar while an update downloads, instead of a blank spinner that made a large update look stuck. You can still choose "Continue in background".
- The Settings window is larger, resizable, and scales properly on high-DPI displays, so it no longer looks tiny or cramped.

## 2.3.25 — 2026-07-16
### Windows
- Fixed the update process hanging forever. Installing from the Microsoft Store no longer shows a "Downloading / Updating" window that never finishes — Store and MSIX builds now update through the correct path instead of trying to run a desktop installer that can't replace a Store app.
- When an update's size isn't reported by the server, the progress bar now animates as a moving "in progress" bar instead of sitting frozen at 0%.
- Fixed a "your session has expired" popup appearing on a brand-new install where you had never signed in.

## 2.3.24 — 2026-07-04
### macOS
- The rewrite shortcut no longer fails silently. If no text is selected (or the app you're in doesn't share its selection), a small hint now appears at your cursor — "Select text first, then press the shortcut" — instead of nothing happening.
- More reliable text capture in terminals and Console, where copying can take a moment longer: the app now waits and retries briefly instead of giving up instantly.

## 2.3.23 — 2026-06-12
### Windows
- Fixed an error when opening **Manage subscription** on a plan that has no self-service billing portal (for example a plan granted by an administrator, or paid via QR code / bank transfer). Instead of a raw "API 404" error, the app now explains that the plan has no billing portal and to contact support.

## 2.3.22 — 2026-06-12
### Windows
- New **Subscription** tab in Settings: see your current plan and upgrade, renew, or manage billing without leaving the app. Pay by card, QR code, or bank transfer, with live payment status shown right in the window.
- Added a **yearly billing** option so you can switch between monthly and annual pricing.
- Updates are now verified by **SHA-256 checksum** before the installer runs — a corrupted or tampered download is rejected instead of being launched.
- Sign-in now validates your email + password before sending, and login errors show the server's actual reason ("Invalid credentials", etc.) instead of a raw stack trace. (Carried over from the unreleased 2.3.15.)
- Security: removed a leftover hardcoded test login that could auto-sign-in in some builds.

## 2.3.21 — 2026-05-30
### macOS
- Internal: server-controlled rollout now decides which `/rewrite` backend the macOS app talks to (NestJS or the new Go service). Default routing is unchanged; rollout is staged via the server-side `GO_BACKEND_RAMP_PERCENT` knob.

## 2.3.20 — 2026-05-30
### macOS
- Internal: test release used to verify the menu-bar badge fix shipped in 2.3.19. No user-facing changes.

## 2.3.19 — 2026-05-30
### macOS
- Fixed the "update available" red dot that only appeared on the menu-bar icon after opening Settings — it now shows automatically the moment a new release is detected, with no clicks required.

## 2.3.18 — 2026-05-30
### macOS
- Internal: test release used to verify the auto-detection of new releases shipped in 2.3.17. No user-facing changes.

## 2.3.17 — 2026-05-30
### macOS
- The menu-bar red "update available" dot now appears automatically when a new release is published — no need to open Settings to trigger the check. The app polls every hour in the background.

## 2.3.16 — 2026-05-30
### macOS
- Rewrites no longer fail with "Request timed out" when the upstream AI provider takes longer than 60 s. The internal request ceiling was bumped from 60 s to 180 s, with a 30 s idle-gap watchdog that still fails fast on a dead network.

## 2.3.15 — 2026-05-29
### Windows
- Sign-in now validates your email + password before sending — empty fields and obvious typos surface as friendly inline messages instead of triggering a server round-trip.
- Login errors show the server's actual reason ("Invalid credentials", "email must be an email", etc.) instead of the raw stack trace some users were seeing.

## 2.3.14 — 2026-05-28
### Windows
- When DraftRight is installed from the Microsoft Store, updates now go through the Store automatically. The "Update available" badge on the tray icon still works — clicking it asks the Store to download and install the new version immediately instead of waiting for the Store's own schedule.
- Sideload (.exe) installs are unaffected: they continue to use the built-in updater.

## 2.3.13 — 2026-05-28
### macOS
- The menu-bar icon now shows a small red dot when a new version is available — no more silent waiting; you can see at a glance when there's something new to install.
### Windows
- Sign in with Google now works: a Google button on the Settings → Account screen opens your browser, completes sign-in, and signs you into DraftRight.
### All platforms
- Internal: the Backend URL field has been removed from Settings. All builds point at the production server by default.

## 2.3.12 — 2026-05-27
- Fixed "Sign in with Google" on macOS, which was failing with an authorization error. Google login now works again.

## 2.3.11 — 2026-05-20
- The "Default Tone (auto-run)" setting now appears under Advanced mode, where it actually applies.
- Internal cleanup and stability improvements across the tray, settings, and update flow.

## 2.3.10 — 2026-05-20
- Advanced mode now auto-runs your chosen "Default Tone" the moment the rewrite panel opens — no need to click a tone first. Leave the default empty to keep picking manually.

## 2.3.9 — 2026-05-20
- Completed the fix for the "update ready" tray badge — it now reliably appears (and the tray menu updates) when a new version is available.

## 2.3.8 — 2026-05-20
- Fixed the tray icon not showing the "update ready" badge (and the tray menu not updating to "Update available") when a new version was detected.

## 2.3.7 — 2026-05-20
- DraftRight now shows a brief "What's New" summary the first time you open it after an update.
- Added centrally-managed logging controls so support can request more detail only when troubleshooting.

## 2.3.6 — 2026-05-20
- The tray icon now shows a badge when an update is downloaded and ready to install, so you don't miss it.

## 2.3.5 — 2026-05-20
- Logs now tag each line as INFO/WARN/ERROR, so problems are easy to spot and bug reports are more useful.

## 2.3.4 — 2026-05-20
- Fixed an "Update failed" error that could appear when starting an install while the update was still downloading in the background. Updates now install smoothly.
- Fixed the background auto-restart helper so it registers correctly.

## 2.3.3 — 2026-05-19
- The update window now has a "Continue in background" button — you can keep working while an update downloads, and install it when it's ready.

## 2.3.2 — 2026-05-18
- Fixed a loop where the app kept offering the same update without it ever applying.
