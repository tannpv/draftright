# Plan — Linux app: runtime verification findings + Wayland hotkey

**Status:** local WIP is **mostly superseded** by `origin/develop`; what survives is a set of runtime bugs verified on a real Linux host.
**Date:** 2026-07-27, revised 2026-07-30 after syncing with the remote.
**Scope:** `DraftRightLinux/` only.

## ⚠️ Read this first — the base was stale

An earlier revision of this doc was written against a local `develop` that was **141 commits behind `origin/develop`** (0 ahead). Five Linux commits already on the remote supersede most of the local WIP:

| Commit | What it did |
|---|---|
| `2d8aba4b` | wire core rewrite loop (X11 MVP) + Rule #1 cleanup (#94, #95) |
| `b9067f7a` | enum dispatch + CSS tokens + panel CSS leak fix (#106, #104, #102) |
| `cc4b6e40` | record MVP status + backlog under EPIC #93 |
| `0e7e906a` | client-side rewrite cache (#108) |
| `64dbe632` | daily limit from admin-controlled plan |

`origin/develop` already implements `show_rewrite_panel()`, `show_settings()`, `_register_hotkey()`, `on_hotkey_pressed()`, a `HealthStatus` enum, `config.py`, `helpers/system_input.py`, `models/health.py`, `models/subscription.py`, `services/rewrite_cache.py`, and `test/test_config_and_tones.py`. **The authoritative backlog lives in `DraftRightLinux/CLAUDE.md` under EPIC #93 — read it, not this doc, for scope.**

So: **rebase onto `origin/develop` first.** Do not commit the local WIP as-is; most of it is duplicate work that will conflict.

## What the local WIP still contributes

`origin/develop`'s own notes say: *"⚠️ ZERO runtime verification. Phase A+B code merged and compiles + 7 unit tests pass, but the rewrite loop has NEVER executed — no X11/GTK host (dev host = macOS). Get a Linux host and smoke `python -m draftright` before closing #94."*

That smoke test has now been run on a real Linux host (Wayland + XWayland, GNOME). Everything below was **reproduced against a clean checkout of `origin/develop`** in a throwaway worktree, not against the WIP.

### Verified-broken on `origin/develop` today

1. **Tray icon is dead on every GTK4 system — not "gracefully degraded."**
   `ui/tray_icon.py:6` calls `gi.require_version("Gtk", "4.0")` at module top; line 12 then tries `gi.require_version('AyatanaAppIndicator3', '0.1')`, which pulls GTK 3.0 and raises:
   ```
   ImportError: Requiring namespace 'Gtk' version '3.0', but '4.0' is already loaded
   ```
   The `except (ImportError, ValueError)` swallows it and logs *"AppIndicator3 not available — install gir1.2-ayatanaappindicator3-0.1"* — **a package that is already installed**. Proven by A/B test: importing Ayatana alone succeeds; importing it after GTK4 fails. CLAUDE.md's "Tray icon gracefully degrades if AppIndicator3 is unavailable" is wrong — it degrades precisely when AppIndicator3 *is* available. **No amount of in-process fixing helps; the indicator must live in a separate GTK3 process** (see below).

2. **#101 blank main window — confirmed.** `do_activate()` builds `Adw.ApplicationWindow(...)`, sets size and title, and calls `present()` with **no `set_content()`**. The window is empty.

3. **"Suggest a feature" crashes on import.** `services/feedback_service.py:15` does `from .settings_service import settings_service`, but no module-level `settings_service` singleton exists → `ImportError` on import.

4. **Settings → Account page raises `AttributeError`.** `ui/settings_window.py:302` calls `auth_service.is_authenticated()` and `:309` calls `auth_service.get_user()`. Neither exists on `AuthService` (which exposes `is_logged_in`, `user`).

5. **Settings read/write raises `AttributeError`.** `ui/settings_window.py:285/296` call `settings_service.get(key, default)` / `.set(key, value)`. Neither method exists on `SettingsService`.

6. **Registration sends the name as the email.** `ui/settings_window.py:353` calls `register(name, email, password)`; the signature is `register(self, email, password, name)`. Arguments are transposed.

7. **#99 Wayland hotkey — dead, and it lies about it.** Runtime log from a clean run:
   ```
   hotkey_service: Wayland detected — using Wayland hotkey listener.
   hotkey_service: D-Bus GlobalShortcuts not implemented; trying xdotool.
   hotkey_service: Falling back to xdotool polling for Wayland hotkey 'Ctrl+Shift+R'...
   application:     Global hotkey registered: Ctrl+Shift+R      ← false
   ```
   `_try_libportal()` logs "not yet implemented" and returns `False`; `_try_dbus()` returns `False`; `_poll_xdotool()` is a busy-loop whose own comment concedes xdotool cannot detect hotkeys under Wayland. The app then reports success. Only `_X11Listener` (python-xlib `grab_key`) is real.

### Fixes worth salvaging from the WIP

Items 2–6 are small and already written locally — port them onto a fresh branch off `origin/develop`:
`_build_main_content()` for #101; `AuthService.is_authenticated()` / `get_user()`; `SettingsService.get()` / `set()`; the `register()` argument order; constructing `SettingsService()` in `feedback_service` instead of importing a singleton.

Item 1 needs the **tray-helper split**: `draftright/tray_helper.py` runs as a standalone GTK3 process (`python -m draftright.tray_helper`) owning the indicator; `TrayIcon` in the GTK4 process becomes a supervisor (`subprocess.Popen` / `terminate()`), and the two halves talk via `gapplication action com.draftright.app <show|settings|quit>` backed by `Gio.SimpleAction`. **Do not merge these back into one process.**

Known gaps in the WIP's own helper split, to fix while porting:
- An early `return` in `TrayIcon.__init__` leaves ~130 lines of dead in-process code below it.
- `TrayIcon.set_status()` is a silent no-op (`_status_item` is always `None`) — backend status never reaches the tray; needs an IPC path to the helper.
- The helper menu **lost** the status line, **Sign Out**, and **Suggest a feature…**; restoring them needs `sign-out` / `suggest-feature` actions plus that status IPC.
- The helper process **leaks** — it survives SIGTERM of the parent, since `stop()` only runs via `quit_app()`. Orphaned `python3 -m draftright.tray_helper` processes were observed. Use a parent-death watch or `atexit`.

## Wayland global hotkey (#99) — now known to be viable here

A clean run activated `org.gnome.Settings.GlobalShortcutsProvider` and `org.freedesktop.portal.Desktop` on this box, so **this GNOME session does implement the GlobalShortcuts portal** — the proper fix is testable locally.

Implement `org.freedesktop.portal.GlobalShortcuts`:
1. `CreateSession`, retain the session handle.
2. `BindShortcuts` with the shortcut id + `preferred_trigger` from `settings_service.hotkey` (default `Ctrl+Shift+R`).
3. Subscribe to the `Activated` signal → `GLib.idle_add(callback)`.
4. Use `Gio.DBusConnection` (already available via PyGObject) rather than adding `dbus-python`.

The portal, not the app, owns the final binding — the compositor may remap it, so read back `ListShortcuts` and show the **actual** trigger in Settings. Support varies by compositor (GNOME 45+/KDE Plasma 6 yes; wlroots varies) — degrade with a **visible** message in Settings, and stop logging "Global hotkey registered" when nothing was registered.

Also verify on Wayland: `clipboard_service._simulate_copy()` and `services/text_injector.py` — `xdotool key ctrl+c` cannot reach native Wayland clients, only XWayland ones.

## Test-environment gotcha (cost real time)

`DraftRightApplication` is a single-instance `Gio.Application` with id `com.draftright.app`. A crashed or backgrounded run can leave the bus name owned by a dead PID; a subsequent `python -m draftright` then silently activates the "remote" instance and **exits 0 immediately**, which reads exactly like a startup crash. Before diagnosing a fast exit, check:
```bash
gdbus call --session --dest org.freedesktop.DBus --object-path /org/freedesktop/DBus \
  --method org.freedesktop.DBus.NameHasOwner com.draftright.app
```
Run smoke tests under `dbus-run-session -- python3 -u -m draftright` to get an isolated bus.

Dev host is **Wayland** (`XDG_SESSION_TYPE=wayland`, `WAYLAND_DISPLAY=wayland-0`) with XWayland up (`DISPLAY=:0`), so `DISPLAY`-based X11 detection guesses wrong. `xdotool`, `xsel`, `wl-paste`, and the Ayatana typelib are installed. The X11 hotkey path **cannot** be exercised here — don't claim it works based on testing on this box.

## Suggested sequencing

1. `git pull` / rebase local `develop` onto `origin/develop` (local is 0 ahead — nothing to lose; the WIP is uncommitted).
2. Branch `feature/linux-runtime-fixes-20260730`.
3. Land items 2–6 (small, mechanical, unblock Settings + feedback entirely).
4. Land the tray-helper split for item 1, with the four gaps above closed.
5. Then #99 GlobalShortcuts portal as its own branch.
6. Update `DraftRightLinux/CLAUDE.md`: correct the "gracefully degrades" tray claim, and record that the #94 smoke test has now run on a real Linux host.
