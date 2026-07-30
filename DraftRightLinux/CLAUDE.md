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
    com.draftright.app.gschema.xml    # GSettings schema
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

## Status & Backlog (as of 2026-07-30 — EPIC #93)

**First runtime verification done (2026-07-30).** The app has now been smoke-run on a real Linux host (GNOME 49 / Wayland + XWayland). That first run exposed seven crash-level defects that compiled and passed unit tests — see "Fixed by runtime verification" below. **X11 is still unverified**: the dev host is Wayland, so `_X11Listener` has never been exercised.

**Done + verified (closed):** Phase B Rule#1 cleanup #95 · sign-out crash #16 · **#99 Wayland global hotkey** · **#101 blank main window**.

**Fixed by runtime verification (2026-07-30):** Settings could not open at all (`SubscriptionPage` inherited a `Protocol` → metaclass conflict at import) · `auth_service.is_authenticated`/`get_user` missing · `settings_service.get`/`set` missing · `register()` args transposed (name sent as email) · `feedback_service` imported a nonexistent singleton · `SettingsService` never `load()`ed · tray dead on every GTK4 system.

**Open (7 under #93):**
- Features/parity: #96 One-Click mode · #97 Google login · #98 Report-a-Bug · #100 KeepAlive agent (all present on Mac+Win)
- Runtime bugs: #103 cursor-position no-op · #105 verify `Gdk.Clipboard.set(str)`
- Rule#1: #104 PANEL_CSS hardcoded hex → tokens

### Wayland global shortcut (#99) — operational requirement

The hotkey uses `org.freedesktop.portal.GlobalShortcuts`. The portal **refuses any caller it cannot identify** (`NotAllowed: An app id is required`), and for an unsandboxed app it derives that id from the systemd scope. Consequences:

- A bare `python3 -m draftright` from a terminal gets **no hotkey**. Use `./draftright-launch.sh`, which wraps the app in an `app-gnome-com.draftright.app-<pid>.scope`.
- `data/com.draftright.app.desktop` must be installed to `~/.local/share/applications/` (the launcher does this). Without it the app id will not resolve.
- The compositor owns the final binding and may substitute a different trigger, so the app displays what `BindShortcuts`/`ListShortcuts` report back — never the requested combination.
- Under a nested `dbus-run-session` the GNOME portal backend cannot reach Mutter, so the handshake stalls. Test the hotkey on the **real** session bus.

**Cross-platform (Win+Linux, not under #93):** #107 Grammar-check + Diff view (macOS-only) · #108 Rewrite cache (macOS-only). `models/payment.py` is the Rule#1 reference (enum + `from_wire` + `display_name`) — bring UI/services up to it.

**Partial:** #22 desktop updater — Linux DONE (version 2.4.1, 401 refresh wired, sha256 integrity in `update_service.py`); macOS `UpdateService.swift` still has NO integrity check → stays open.

## Key Patterns

- All UI is built programmatically in Python -- no .ui XML files
- API calls run in `threading.Thread` with `GLib.idle_add` for UI updates
- Display server detection in `helpers/display_server.py` drives X11 vs Wayland code paths
- Tray icon gracefully degrades if AppIndicator3 is unavailable
- Settings window uses Adw.PreferencesWindow with two pages (Account, Preferences)
- Rewrite panel is an undecorated always-on-top Gtk.Window
