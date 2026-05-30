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
