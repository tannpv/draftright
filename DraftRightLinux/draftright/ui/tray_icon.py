"""System tray icon using AppIndicator3 (libayatana-appindicator)."""

import logging

import gi
gi.require_version("Gtk", "4.0")
from gi.repository import Gtk

try:
    gi.require_version('AyatanaAppIndicator3', '0.1')
    from gi.repository import AyatanaAppIndicator3 as AppIndicator3
except (ImportError, ValueError):
    try:
        gi.require_version('AppIndicator3', '0.1')
        from gi.repository import AppIndicator3
    except (ImportError, ValueError):
        AppIndicator3 = None

logger = logging.getLogger(__name__)


class TrayIcon:
    """System tray icon for DraftRight.

    Uses AyatanaAppIndicator3 or AppIndicator3 to display a persistent
    tray icon with a context menu. Falls back gracefully if neither
    library is available.
    """

    def __init__(self, app):
        """Initialize the tray icon.

        Args:
            app: The DraftRightApplication instance.
        """
        self.app = app
        self.indicator = None
        self._status_item = None

        if AppIndicator3 is None:
            logger.warning(
                "AppIndicator3 not available -- system tray icon disabled. "
                "Install gir1.2-ayatanaappindicator3-0.1 for tray support."
            )
            return

        self.indicator = AppIndicator3.Indicator.new(
            "com.draftright.app",
            "edit-paste-symbolic",
            AppIndicator3.IndicatorCategory.APPLICATION_STATUS,
        )
        self.indicator.set_status(AppIndicator3.IndicatorStatus.ACTIVE)
        self.indicator.set_title("DraftRight")

        # Build context menu (GTK3-style menu required by AppIndicator)
        self.indicator.set_menu(self._build_menu())

    def _build_menu(self):
        """Build the indicator context menu.

        Note: AppIndicator3 requires a Gtk3-style Gtk.Menu.  On GTK4-only
        systems this may need the gtk3 compatibility layer.  We import the
        Gtk 3.0 Menu directly to satisfy the AppIndicator API.
        """
        try:
            gi.require_version("Gtk", "3.0")
            from gi.repository import Gtk as Gtk3
        except ValueError:
            # Already loaded GTK4; fall back to building menu with GTK4
            # (will only work on systems where AppIndicator accepts it)
            Gtk3 = Gtk

        menu = Gtk3.Menu()

        # Status label (disabled, shows connectivity)
        self._status_item = Gtk3.MenuItem(label="Offline")
        self._status_item.set_sensitive(False)
        menu.append(self._status_item)
        menu.append(Gtk3.SeparatorMenuItem())

        # Open Settings
        item_settings = Gtk3.MenuItem(label="Open Settings")
        item_settings.connect("activate", self._on_open_settings)
        menu.append(item_settings)

        # Separator
        menu.append(Gtk3.SeparatorMenuItem())

        # Sign Out
        item_sign_out = Gtk3.MenuItem(label="Sign Out")
        item_sign_out.connect("activate", self._on_sign_out)
        menu.append(item_sign_out)

        # Quit
        item_quit = Gtk3.MenuItem(label="Quit")
        item_quit.connect("activate", self._on_quit)
        menu.append(item_quit)

        menu.show_all()
        return menu

    # ------------------------------------------------------------------
    # Callbacks
    # ------------------------------------------------------------------

    def _on_open_settings(self, _widget):
        """Open the settings window."""
        self.app.show_settings()

    def _on_sign_out(self, _widget):
        """Sign the user out."""
        self.app.sign_out()

    def _on_quit(self, _widget):
        """Quit the application."""
        self.app.quit_app()

    # ------------------------------------------------------------------
    # Public helpers
    # ------------------------------------------------------------------

    def set_icon(self, icon_name: str):
        """Change the tray icon.

        Args:
            icon_name: A freedesktop icon name or absolute path to an icon file.
        """
        if self.indicator:
            self.indicator.set_icon(icon_name)

    def set_status(self, status: str):
        """Update the status menu item label.

        Args:
            status: One of 'connected', 'not_logged_in', 'offline'.
        """
        if self._status_item is None:
            return
        labels = {
            "connected": "Connected",
            "not_logged_in": "Not Logged In",
            "offline": "Offline",
        }
        self._status_item.set_label(labels.get(status, "Offline"))
