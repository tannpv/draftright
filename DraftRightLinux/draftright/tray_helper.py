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

import gi

gi.require_version("Gtk", "3.0")
gi.require_version("AyatanaAppIndicator3", "0.1")
from gi.repository import AyatanaAppIndicator3, GLib, Gio, Gtk

from draftright import config
from draftright.models.health import HealthStatus
from draftright.models.tray import TrayAction, TrayCommand

log = logging.getLogger(__name__)


class TrayHelper:
    """Owns the indicator, its menu, and the pipe back to the parent."""

    def __init__(self) -> None:
        self._actions = self._connect_actions()
        self._status = HealthStatus.OFFLINE
        self._icon = config.TRAY_ICON_DEFAULT

        self._status_item = Gtk.MenuItem(label=self._status.display_name)
        self._status_item.set_sensitive(False)

        self._indicator = AyatanaAppIndicator3.Indicator.new(
            config.APP_ID,
            config.TRAY_ICON_DEFAULT,
            AyatanaAppIndicator3.IndicatorCategory.APPLICATION_STATUS,
        )
        self._indicator.set_status(AyatanaAppIndicator3.IndicatorStatus.ACTIVE)
        self._indicator.set_title("DraftRight")
        self._indicator.set_menu(self._build_menu())

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

        icon = (
            config.TRAY_ICON_ATTENTION
            if status is HealthStatus.OFFLINE
            else config.TRAY_ICON_DEFAULT
        )
        if icon != self._icon:
            # Only repaint on a real change: each call is a round trip to the
            # tray host, and the health probe pushes on a timer.
            self._icon = icon
            # set_icon() is deprecated; set_icon_full() also carries the label
            # screen readers announce.
            self._indicator.set_icon_full(icon, f"DraftRight — {status.display_name}")

    def handle_line(self, line: str) -> bool:
        """Apply one protocol line.  Returns False when asked to quit."""
        command, payload = TrayCommand.parse(line)
        if command is TrayCommand.QUIT:
            return False
        if command is TrayCommand.STATUS:
            self.set_status(HealthStatus.from_wire(payload))
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
