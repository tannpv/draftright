"""GTK3 tray-indicator helper process for the GTK4 DraftRight application.

Why this is a separate process
------------------------------
Ayatana AppIndicator3 is a GTK3 library.  A single Python process cannot load
both GTK 3.0 and GTK 4.0 — the second ``gi.require_version`` raises
``ImportError: Requiring namespace 'Gtk' version '3.0', but '4.0' is already
loaded``.  The GTK4 app therefore spawns this module as a child process and
talks to it over pipes.

Both directions of the contract live in :mod:`draftright.models.tray`:
menu entries come from :class:`TrayAction` (activated on the app's exported
action group over the session bus), and parent → helper messages use
:class:`TrayCommand`.  Closing stdin yields EOF, which quits the helper — so
the indicator can never outlive the app that owns it.
"""

from __future__ import annotations

import logging
import os
import sys
import tempfile

import gi

gi.require_version("Gtk", "3.0")
gi.require_version("AyatanaAppIndicator3", "0.1")
from gi.repository import AyatanaAppIndicator3, GLib, Gio, Gtk

from draftright import config
from draftright.helpers import tray_icon_render
from draftright.models.health import HealthStatus
from draftright.models.tray import TrayAction, TrayCommand

log = logging.getLogger(__name__)


class TrayHelper:
    """Owns the indicator, its menu, and the pipe back to the parent."""

    def __init__(self) -> None:
        self._actions = self._connect_actions()
        self._status = HealthStatus.OFFLINE
        self._update_available = False
        # Busy pulse (#6): GLib source id while animating, None when idle.
        self._busy_source: int | None = None
        self._busy_frame = 0
        self._icon = self._resolve_default_icon()
        # Composited state icons are written here and the indicator is pointed
        # at it as an extra icon-theme directory.
        self._icon_dir = tempfile.mkdtemp(prefix=config.TRAY_ICON_NAME_PREFIX)

        self._status_item = Gtk.MenuItem(label=self._status.display_name)
        self._status_item.set_sensitive(False)
        self._update_item = Gtk.MenuItem(label="Update available — restart to install")
        self._update_item.set_sensitive(False)
        # show_all() below would otherwise force this visible again.
        self._update_item.set_no_show_all(True)
        self._update_item.set_visible(False)

        self._indicator = AyatanaAppIndicator3.Indicator.new(
            config.APP_ID,
            self._icon,
            AyatanaAppIndicator3.IndicatorCategory.APPLICATION_STATUS,
        )
        self._indicator.set_status(AyatanaAppIndicator3.IndicatorStatus.ACTIVE)
        self._indicator.set_title("DraftRight")
        self._indicator.set_icon_theme_path(self._icon_dir)
        self._indicator.set_menu(self._build_menu())

    @staticmethod
    def _resolve_default_icon() -> str:
        """Prefer the DraftRight icon; fall back if it was never installed.

        AppIndicator silently shows nothing for an unknown icon name, so check
        the theme rather than trusting the install.
        """
        try:
            if Gtk.IconTheme.get_default().has_icon(config.TRAY_ICON_DEFAULT):
                return config.TRAY_ICON_DEFAULT
        except Exception:  # noqa: BLE001 — theme lookup must never block startup
            pass
        log.warning(
            "Icon %r not in the icon theme — using a stock glyph. Install "
            "data/icons/hicolor into ~/.local/share/icons/.",
            config.TRAY_ICON_DEFAULT,
        )
        return config.TRAY_ICON_FALLBACK

    # -- wiring ------------------------------------------------------------

    @staticmethod
    def _connect_actions() -> "Gio.DBusActionGroup | None":
        """Return the app's exported action group, or None if unreachable."""
        try:
            bus = Gio.bus_get_sync(Gio.BusType.SESSION, None)
            actions = Gio.DBusActionGroup.get(
                bus, config.APP_ID, config.APP_OBJECT_PATH
            )
            # DBusActionGroup populates asynchronously; prime it so the first
            # menu click doesn't land on an empty group and get dropped.
            actions.list_actions()
            return actions
        except GLib.Error as exc:
            log.warning("Tray cannot reach the app on the session bus: %s", exc)
            return None

    def _build_menu(self) -> Gtk.Menu:
        """Build the menu from :class:`TrayAction` — declaration order is menu order."""
        menu = Gtk.Menu()
        menu.append(self._status_item)
        menu.append(self._update_item)
        for action in TrayAction:
            if action.starts_group:
                menu.append(Gtk.SeparatorMenuItem())
            item = Gtk.MenuItem(label=action.display_name)
            item.connect("activate", self._on_activate, action)
            menu.append(item)
        menu.show_all()
        return menu

    def _on_activate(self, _item: Gtk.MenuItem, action: TrayAction) -> None:
        if self._actions is None:
            log.warning("Dropping tray action %r — no connection to the app.", action.value)
            return
        self._actions.activate_action(action.value, None)

    # -- parent → helper ---------------------------------------------------

    def set_status(self, status: HealthStatus) -> None:
        if status is self._status:
            return
        self._status = status
        self._status_item.set_label(status.display_name)
        self._refresh_icon()

    def set_update_available(self, available: bool) -> None:
        """Flag that an app update is ready — shown as a red dot (#22)."""
        if available == self._update_available:
            return
        self._update_available = available
        self._update_item.set_visible(available)
        self._refresh_icon()

    def set_busy(self, busy: bool) -> None:
        """Pulse the icon while a One-Click rewrite is in flight (#6).

        One-Click replaces the selection with no window of its own, so without
        this the hotkey looks inert until the text changes.
        """
        if busy == (self._busy_source is not None):
            return
        if busy:
            self._busy_frame = 0
            # A positive interval, and the callback returns True to repeat: a
            # zero interval would re-arm instantly and spin the CPU.
            self._busy_source = GLib.timeout_add(
                config.TRAY_BUSY_FRAME_MS, self._on_busy_tick
            )
            self._on_busy_tick()      # show the first frame without waiting
        else:
            GLib.source_remove(self._busy_source)
            self._busy_source = None
            # Force a repaint: _refresh_icon short-circuits when the icon name
            # is unchanged, and the pulse left a busy frame showing.
            self._icon = None
            self._refresh_icon()

    def _on_busy_tick(self) -> bool:
        rendered = tray_icon_render.build_busy_frame(
            self._busy_frame, directory=self._icon_dir
        )
        self._busy_frame += 1
        if rendered is None:
            return True   # no symbolic to render; keep the timer, icon unchanged
        self._icon = rendered[1]
        self._indicator.set_icon_full(rendered[1], config.TRAY_BUSY_DESCRIPTION)
        return True

    def _refresh_icon(self) -> None:
        """Pick the icon for the current (status, update) pair.

        A plain connected state with no update uses the *named* symbolic so
        the shell recolours it for light/dark panels; anything else needs our
        own colours composited, which the shell must not repaint.

        No-op while the busy pulse is running: it owns the icon until it ends,
        and a health poll landing mid-rewrite would otherwise stamp the status
        icon over a frame and stall the animation visually.
        """
        if self._busy_source is not None:
            return
        rendered = tray_icon_render.build(
            self._status, self._update_available, directory=self._icon_dir
        )
        icon = rendered[1] if rendered else self._resolve_default_icon()
        if icon == self._icon:
            return
        self._icon = icon
        description = f"DraftRight — {self._status.display_name}"
        if self._update_available:
            description += ", update available"
        # set_icon() is deprecated; set_icon_full() also carries the label
        # screen readers announce.
        self._indicator.set_icon_full(icon, description)

    def handle_line(self, line: str) -> bool:
        """Apply one protocol line.  Returns False when asked to quit."""
        command, payload = TrayCommand.parse(line)
        if command is TrayCommand.QUIT:
            return False
        if command is TrayCommand.STATUS:
            self.set_status(HealthStatus.from_wire(payload))
        elif command is TrayCommand.UPDATE:
            self.set_update_available(payload == "1")
        elif command is TrayCommand.BUSY:
            self.set_busy(payload == "1")
        else:
            log.debug("Ignoring unknown tray command: %r", line)
        return True


def _watch_parent(helper: TrayHelper) -> None:
    """Read the protocol off stdin; EOF means the parent is gone."""
    buffer = ""

    def on_readable(fd: int, condition: int) -> bool:
        nonlocal buffer
        if condition & (GLib.IO_HUP | GLib.IO_ERR):
            Gtk.main_quit()
            return False
        try:
            chunk = os.read(fd, 4096)
        except OSError:
            Gtk.main_quit()
            return False
        if not chunk:  # EOF — parent exited or closed the pipe.
            Gtk.main_quit()
            return False
        buffer += chunk.decode("utf-8", errors="replace")
        while "\n" in buffer:
            line, buffer = buffer.split("\n", 1)
            if not helper.handle_line(line):
                Gtk.main_quit()
                return False
        return True

    GLib.io_add_watch(
        sys.stdin.fileno(),
        GLib.PRIORITY_DEFAULT,
        GLib.IO_IN | GLib.IO_HUP | GLib.IO_ERR,
        on_readable,
    )


def main() -> None:
    logging.basicConfig(level=logging.WARNING)
    helper = TrayHelper()
    _watch_parent(helper)
    Gtk.main()


if __name__ == "__main__":
    main()
