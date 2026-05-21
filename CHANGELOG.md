# DraftRight — Changelog

User-facing release notes. **One `## <version>` section per release.** The
release pipeline extracts the section whose heading matches the version being
published and writes it into the update server (`app_releases.release_notes`),
so every client can show a "What's New" notice on the first launch after
updating. Keep entries short and written for end users; if a version needs the
user to *do* something after updating, say so explicitly under a **Action
needed:** line.

Heading format must stay `## <version>` (a date after an em dash is optional and
ignored by the extractor), e.g. `## 2.3.6 — 2026-05-20`.

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
