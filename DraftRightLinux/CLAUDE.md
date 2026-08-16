# DraftRight Linux

Native Linux desktop app for DraftRight -- AI-powered text rewriting.

## Tech Stack

| Component | Technology |
|---|---|
| UI framework | GTK4 + libadwaita (Adw) |
| Language | Python 3.10+ |
| System tray | AyatanaAppIndicator3 (libayatana-appindicator) |
| Global hotkey | python-xlib (X11) / xdg-desktop-portal (Wayland) |
| Text capture | xdotool + xsel (X11) / wl-clipboard (Wayland) |
| Packaging | Flatpak, Debian (.deb), AppImage |

## Project Structure

```
DraftRightLinux/
  draftright/
    __init__.py              # Package init
    __main__.py              # Entry point (python -m draftright)
    main.py                  # CLI launcher
    application.py           # Adw.Application subclass
    ui/
      __init__.py
      rewrite_panel.py       # Floating rewrite window (Gtk.Window)
      settings_window.py     # Settings (Adw.PreferencesWindow)
      tray_icon.py           # System tray via AppIndicator3
    services/                # API client, auth, hotkeys, clipboard, settings
    models/                  # Data models (user, settings)
    helpers/
      __init__.py
      display_server.py      # Detect X11 vs Wayland
    resources/
      style.css              # Global CSS overrides
  data/
    com.draftright.app.desktop        # Desktop entry
    com.draftright.app.metainfo.xml   # AppStream metadata
  packaging/
    flatpak/
      com.draftright.app.yml          # Flatpak manifest
    deb/
      control                         # Debian package control
    appimage/                         # AppImage config (TBD)
  meson.build                         # Build system
  draftright-launch.sh                # Dev launcher script
  CLAUDE.md                           # This file
```

## How to Run (Development)

```bash
# Install system dependencies (Ubuntu/Fedora)
# Ubuntu:
sudo apt install python3-gi gir1.2-gtk-4.0 gir1.2-adw-1 \
  gir1.2-ayatanaappindicator3-0.1 xdotool xsel

# Fedora:
sudo dnf install python3-gobject gtk4 libadwaita \
  libayatana-appindicator-gtk3 xdotool xsel

# Install Python deps
pip install requests

# Run
python -m draftright
# or
./draftright-launch.sh
```

## How to Build

### Flatpak

```bash
cd packaging/flatpak
flatpak-builder --user --install build com.draftright.app.yml
flatpak run com.draftright.app
```

### Debian Package

```bash
# From project root, build with dpkg-buildpackage or checkinstall
sudo checkinstall --pkgname=draftright --pkgversion=1.0.0 \
  --requires="python3,python3-gi,gir1.2-gtk-4.0,gir1.2-adw-1,xdotool,xsel" \
  pip install --prefix=/usr .
```

## Design Tokens

| Token | Hex |
|---|---|
| Background | #0f172a |
| Card | #1e293b |
| Border | #334155 |
| Brand blue | #5d87ff |
| Text | #e2e8f0 |
| Muted | #94a3b8 |
| Success green | #10b981 |

## Status & Backlog (as of 2026-07-31 — EPIC #93 CLOSED)

**First runtime verification done (2026-07-30).** The app has now been smoke-run on a real Linux host (GNOME 49 / Wayland + XWayland). That first run exposed seven crash-level defects that compiled and passed unit tests — see "Fixed by runtime verification" below. **X11 is still unverified**: the dev host is Wayland, so `_X11Listener` has never been exercised.

**Done + verified (closed):** Phase B Rule#1 cleanup #95 · sign-out crash #16 · **#99 Wayland global hotkey** · **#101 blank main window** · **#103 cursor placement** (won't-fix: GTK4 removed client-side positioning and Wayland forbids self-placement — the dead no-op is gone) · **#104 PANEL_CSS tokens** (already done) · **#105 `Gdk.Clipboard.set(str)`** (verified working: set + read-back round-trip on GTK4).

**Fixed by runtime verification (2026-07-30):** Settings could not open at all (`SubscriptionPage` inherited a `Protocol` → metaclass conflict at import) · `auth_service.is_authenticated`/`get_user` missing · `settings_service.get`/`set` missing · `register()` args transposed (name sent as email) · `feedback_service` imported a nonexistent singleton · `SettingsService` never `load()`ed · tray dead on every GTK4 system.

**EPIC #93 closed.** The app is working and user-verified. Test suite 7 → 119.

**Still unverified — cannot be closed from this machine:**
1. **No signed-in rewrite round-trip** has ever run against the live backend; the core loop is only exercised against stubs.
2. **Wayland Replace** — the RemoteDesktop keystroke has never been confirmed to land in another app's field (needs a one-time permission grant).
3. **X11 is unexercised** — this host is Wayland, and `_X11Listener`'s key parsing was refactored without being run.
4. **Google sign-in completion** — the `client_secret` fix is verified against Google's token endpoint, but an actual sign-in has not been confirmed.

**Known gaps:** Simple mode has no progress indicator (Windows shows one at the cursor; Wayland cannot position a window). The Flatpak tray needs a libayatana-appindicator module. `Adw.PreferencesWindow` is deprecated since libadwaita 1.5 (this host runs 1.9.1) in favour of `Adw.PreferencesDialog`. `setup.py develop` editable installs are removed in pip 25.3 — needs a `pyproject.toml`.

**Landed, pending config/verification:** #96 One-Click · #97 Google login (needs a Desktop OAuth client id) · #98 Report-a-Bug · #100 KeepAlive

### Keep-alive (#100)

A systemd **user service** with `Restart=on-failure`, installed by the
"Auto-start on login" switch. Two things are load-bearing:

- The unit is named `app-com.draftright.app@autostart.service`. xdg-desktop-portal identifies an unsandboxed app by its systemd unit, so a name like `draftright.service` would cost the Wayland global shortcut (#99). Verified: the hotkey binds when launched under this unit.
- `on-failure`, never `always` — a clean Quit exits 0 and must stay quit. Verified empirically: exit 0 → 1 start, exit 1 → respawned.
- Enabling it **removes** the XDG autostart entry; both would launch at login and the loser dies silently against the single-instance lock. Without systemd it falls back to the XDG entry.

**Known gaps (not yet issues):**
- **X11 is unverified.** The dev host is Wayland, so `_X11Listener` has never run. Its key parsing was refactored onto `models/hotkey.py` without being exercised.
- **Tray is disabled under Flatpak.** `org.gnome.Platform` ships GTK3 but not libayatana-appindicator; it must be built as a manifest module.
- `RewritePanel` is a bare `Gtk.Window` (no `application=`), so it never appears in `app.get_windows()`.

### Google sign-in (#97) — needs a Desktop-type OAuth client

The flow is implemented (PKCE + loopback redirect, RFC 8252) but **inert until
a client id is configured** — `config.DEFAULT_GOOGLE_CLIENT_ID` is empty, and
`google_sign_in_available()` hides the Settings button while it is.

- macOS uses an **iOS-type** client (`...-dvkn61dhibse9fu83ohh51mlovd7269a`)
  with a reversed-scheme redirect. Google only accepts custom schemes from
  iOS-type clients and only accepts `http://127.0.0.1:<port>` from
  **Desktop app** clients, so the macOS id **cannot be reused** on Linux.
- Create a "Desktop app" OAuth client in the Google Cloud console, then either
  set `DEFAULT_GOOGLE_CLIENT_ID` or export `DRAFTRIGHT_GOOGLE_CLIENT_ID`.
- No client secret is used or needed: a native app is a public client and PKCE
  is the proof-of-possession.

> ✅ **Backend Google `aud` validation — FIXED (both backends).** Google
> `id_token` verification now rejects any token whose `aud` is not an accepted
> client id, closing the replay/account-takeover hole. Node
> (`verifyGoogleToken` in `backend/src/auth/auth.service.ts`) and the Go
> production backend (`verifyGoogle` in
> `backend-rewrite-go/internal/auth/social_http.go`) both check `aud` + `iss` +
> `sub` with byte-identical error messages. Accepted set = the shipped
> per-platform client ids (or `GOOGLE_AUDIENCES` env override); Node also unions
> `app_settings.google_client_id`. Go is boot-frozen (env-or-defaults), same as
> its Apple path — so a **custom** admin-set `google_client_id` that is not also
> in `GOOGLE_AUDIENCES` is honoured by Node but not Go; set `GOOGLE_AUDIENCES`
> when customising.

### Wayland global shortcut (#99) — operational requirement

The hotkey uses `org.freedesktop.portal.GlobalShortcuts`. The portal **refuses any caller it cannot identify** (`NotAllowed: An app id is required`), and for an unsandboxed app it derives that id from the systemd scope. Consequences:

- A bare `python3 -m draftright` from a terminal gets **no hotkey**. Use `./draftright-launch.sh`, which wraps the app in an `app-gnome-com.draftright.app-<pid>.scope`.
- `data/com.draftright.app.desktop` must be installed to `~/.local/share/applications/` (the launcher does this). Without it the app id will not resolve.
- The compositor owns the final binding and may substitute a different trigger, so the app displays what `BindShortcuts`/`ListShortcuts` report back — never the requested combination.
- Under a nested `dbus-run-session` the GNOME portal backend cannot reach Mutter, so the handshake stalls. Test the hotkey on the **real** session bus.

**Cross-platform (Win+Linux, not under #93):** #107 Grammar-check + Diff view (macOS-only) · #108 Rewrite cache (macOS-only). `models/payment.py` is the Rule#1 reference (enum + `from_wire` + `display_name`) — bring UI/services up to it.

**Partial:** #22 desktop updater — Linux DONE (version 2.4.1, 401 refresh wired, sha256 integrity in `update_service.py`); macOS `UpdateService.swift` HAS the integrity check in code + tests (`verifyIntegrity`, commit `73112702`, on main; `UpdateServiceTests`) but it is **NOT in the shipped binary** — macOS 2.3.30 (shipped 2026-07-23) predates the check (2026-07-30), so deployed macOS still auto-updates unverified. Closing #22's macOS half needs the **next macOS notarized release** (manual, Developer ID) to carry `73112702`, not more code.

## Rewrite trigger — Pencil / Hotkey (#188)

Two mutually-exclusive triggers via `models/trigger_mode.py` `TriggerMode
{pencil, hotkey}` (wire values `"pencil"/"hotkey"`, byte-identical to
macOS/Windows so a synced `settings.json` means the same thing everywhere).
Settings → Rewrite → **Trigger** picker; default **Hotkey**.

**The pencil is X11-only.** It needs global selection monitoring and a
cursor-placed affordance; Wayland forbids both (same wall as #103). So on
Wayland the app stays on the hotkey regardless of the setting —
`_apply_trigger_mode()` gates the pencil on `display_server.is_x11()`.

Design — no synthetic copy, no floating button (unlike macOS/Windows):
- X11 already holds highlighted text in the **PRIMARY selection**, and GTK4
  can't position a window at the cursor anyway (#103). So the pencil
  (`services/pencil_trigger.py`) **polls PRIMARY** (`GLib.timeout_add`, 400 ms,
  via `ClipboardService`'s existing PRIMARY read) and a new highlight opens the
  **existing `RewritePanel`** — no new overlay code.
- Both triggers funnel through one chokepoint, `application._route_captured_text`
  (RULE #1) — app-mode routing (Advanced panel vs One-Click) lives in one place.
- Pencil and hotkey are mutually exclusive: `_apply_trigger_mode` runs the
  pencil **or** the hotkey, never both. Register/unregister is flag-guarded to
  avoid the Windows #186 ALREADY_REGISTERED trap. Because the pencil only runs
  on X11, the hotkey is only ever stopped/restarted on X11 (XGrabKey), never the
  fragile Wayland portal binding.
- Pure decision `services/pencil_trigger_decision.should_trigger(prev, cur)` is
  GTK-free and unit-tested (`test/test_pencil_trigger_decision.py`), like the
  macOS/Windows `PencilTriggerDecision`.

**UNVERIFIED — never run on a Linux X11 host** (dev Mac has no `gi`, so the GTK
paths can't even be compile-checked here; only the pure enum + decision are
tested — 9 tests). Debug the polling loop + picker via the app log on a real
X11 session, the same log-driven loop as Windows #180.

## Hard-won gotchas

Things that cost real debugging time here. Each one makes correct code look
broken, or broken code look fine.

### The app must actually be run

The Linux app had never been executed on a Linux host (dev host was macOS).
That single fact produced seven crash-level defects that **compiled and passed
the unit suite** — Settings could not open at all, the tray was dead on every
GTK4 system, and every *successful* rewrite crashed on a dict/str mismatch
(invisible because the app had never been signed in, so the success branch had
never executed). A green suite says the units work, not that the app does.

### Install / restart

`draftright` is an **editable** install, so `~/.local/bin/draftright` always
runs the working tree. **After a change, restart the app — do not reinstall.**
Python loads modules at startup, so a running process keeps the code it began
with. A real reinstall is only needed when `setup.py` changes.

Restart via `./draftright-launch.sh`, or
`systemd-run --user --scope -q --unit="app-gnome-com.draftright.app-<uniq>" ~/.local/bin/draftright`
— the scope name is load-bearing (see below).

### Wayland / portals

- **The global shortcut needs an app id**, which xdg-desktop-portal derives
  from the systemd unit. A bare `python -m draftright` from a terminal gets
  `NotAllowed: An app id is required` and no hotkey. Same for the keep-alive
  unit name (`app-com.draftright.app@autostart.service`).
- **Clipboard reads need a focused surface.** A headless harness sees an empty
  clipboard even when `wl-paste --list-types` shows `image/png`. Present a real
  window first.
- **A nested `dbus-run-session` cannot reach Mutter** — portal handshakes stall
  there. Isolated buses are right for most tests, wrong for portal work.
- **Non-interactive screenshots are refused** for unsandboxed apps
  ("ended by the portal"). Use `interactive=True`; the compositor picks.
- **`xdotool` only reaches XWayland clients**, which is why Replace goes
  through the RemoteDesktop portal.

### GTK4 / libadwaita window traps

All four of these were reported as *"I cannot close it"*:

1. **`Adw.ApplicationWindow` draws no titlebar** — the header must be inside
   the content. A bare `Gtk.Box` yields a window with no close button at all.
2. **One window per hotkey press stacks.** Cache and reuse the panel; closing
   the top of a stack just reveals the next.
3. **Modal + a hidden transient parent is unclosable.** This is a tray app, so
   parents are often hidden. Only adopt a visible parent; preferences are not
   modal.
4. **Undecorated windows need an Escape binding** — the compositor gives them
   no close button.

Also: `timeout_add_seconds(0, cb)` where `cb` returns `True` re-arms instantly
— measured at ~1.3M calls in 10s, each spawning a network thread. For an
immediate run, just call the function.

### Google sign-in

Google's **Desktop-type** clients require `client_secret` at the token
endpoint **even with PKCE**, contradicting RFC 8252. macOS avoids this by
using an iOS-type client (reversed scheme, genuinely no secret); Linux and
Windows use Desktop clients and must send it. Each platform has its **own**
client id, and the backend validates `aud` — a new id must be added to
`GOOGLE_AUDIENCES` or sign-in fails with "Invalid Google token".

### Environment traps that fake a bug

- **Single-instance:** a stale `com.draftright.app` bus name makes a launch
  exit 0 instantly, looking exactly like a startup crash. Check with
  `gdbus call ... NameHasOwner com.draftright.app`.
- **An old build autostarts at login** from `~/.config/autostart/` and holds
  that bus name. `pgrep -f "python3 -m draftright"` does **not** match it;
  use `ps -eo pid,cmd | grep local/bin/draftright`.
- **`DraftRightLinux/data/` was gitignored** as "runtime data" while actually
  holding packaging assets, so `git add` silently no-op'd and two commits
  claimed to add files that never landed. Fixed, but check `git status` after
  adding new asset paths.

## Key Patterns

- All UI is built programmatically in Python -- no .ui XML files
- API calls run in `threading.Thread` with `GLib.idle_add` for UI updates
- Display server detection in `helpers/display_server.py` drives X11 vs Wayland code paths
- Tray icon gracefully degrades if AppIndicator3 is unavailable
- Settings window uses Adw.PreferencesWindow with two pages (Account, Preferences)
- Rewrite panel is an undecorated always-on-top Gtk.Window
