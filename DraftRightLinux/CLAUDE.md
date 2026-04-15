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

## Key Patterns

- All UI is built programmatically in Python -- no .ui XML files
- API calls run in `threading.Thread` with `GLib.idle_add` for UI updates
- Display server detection in `helpers/display_server.py` drives X11 vs Wayland code paths
- Tray icon gracefully degrades if AppIndicator3 is unavailable
- Settings window uses Adw.PreferencesWindow with two pages (Account, Preferences)
- Rewrite panel is an undecorated always-on-top Gtk.Window
