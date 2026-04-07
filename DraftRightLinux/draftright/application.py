"""DraftRight GTK4 Application."""

import gi
gi.require_version("Gtk", "4.0")
gi.require_version("Adw", "1")

from gi.repository import Adw, Gio, Gtk, GLib

import threading
import logging

from draftright.services.logger import setup_logging

logger = logging.getLogger(__name__)


class DraftRightApplication(Adw.Application):
    """Main application class for DraftRight Linux."""

    def __init__(self):
        setup_logging()

        super().__init__(
            application_id="com.draftright.app",
            flags=Gio.ApplicationFlags.FLAGS_NONE,
        )

        # Service references (initialized in do_activate)
        self.api_client = None
        self.auth_service = None
        self.settings_service = None
        self.hotkey_service = None
        self.clipboard_service = None

        self._backend_status = "offline"
        self._is_rewriting = False
        self._tray_icon = None

    def do_activate(self):
        """Called when the application is activated."""
        # Force dark color scheme
        style_manager = Adw.StyleManager.get_default()
        style_manager.set_color_scheme(Adw.ColorScheme.FORCE_DARK)

        # Load CSS
        self._load_css()

        # Set up tray icon
        self._setup_tray()

        # Register global hotkey
        self._register_hotkey()

        # Restore auth session
        self._restore_session()

        # Show main window if no windows exist
        win = self.props.active_window
        if not win:
            win = Adw.ApplicationWindow(application=self)
            win.set_default_size(400, 500)
            win.set_title("DraftRight")
        win.present()

        # Start health check — immediate first check, then every 30 seconds
        GLib.timeout_add_seconds(0, self._trigger_health_check)
        GLib.timeout_add_seconds(30, self._trigger_health_check)

    def _load_css(self):
        """Load custom CSS from resources."""
        css_provider = Gtk.CssProvider()
        import importlib.resources as pkg_resources
        try:
            css_path = pkg_resources.files("draftright.resources").joinpath("style.css")
            css_provider.load_from_path(str(css_path))
        except Exception:
            pass
        Gtk.StyleContext.add_provider_for_display(
            self.props.active_window.get_display() if self.props.active_window else
            __import__("gi.repository", fromlist=["Gdk"]).Gdk.Display.get_default(),
            css_provider,
            Gtk.STYLE_PROVIDER_PRIORITY_APPLICATION,
        )

    def _setup_tray(self):
        """Set up system tray icon."""
        from draftright.ui.tray_icon import TrayIcon
        self._tray_icon = TrayIcon(self)

    def _register_hotkey(self):
        """Register global hotkey for text capture."""
        # Will be implemented by HotkeyService using
        # X11 (python-xlib) or Wayland (portal) bindings.
        pass

    def _restore_session(self):
        """Restore saved authentication session."""
        if self.auth_service:
            self.auth_service.restore_session()

    # ------------------------------------------------------------------
    # Public API
    # ------------------------------------------------------------------

    def on_hotkey_pressed(self):
        """Handle global hotkey press — capture selected text and show rewrite panel."""
        text = ""
        if self.clipboard_service:
            text = self.clipboard_service.get_selected_text()
        if text:
            self.show_rewrite_panel(text)

    def show_rewrite_panel(self, text: str):
        """Open the rewrite panel with the captured text.

        Args:
            text: The selected text to rewrite.
        """
        # Will instantiate and present RewritePanel UI
        print(f"[DraftRight] Rewrite panel requested for {len(text)} chars")

    def show_settings(self):
        """Open the settings window."""
        # Will instantiate and present SettingsWindow UI
        print("[DraftRight] Settings window requested")

    def sign_out(self):
        """Clear auth session and notify the user."""
        if self.auth_service:
            self.auth_service.clear_session()

        notification = Gio.Notification.new("DraftRight")
        notification.set_body("You have been signed out.")
        self.send_notification("sign-out", notification)

    def quit_app(self):
        """Clean up resources and quit the application."""
        if self.hotkey_service:
            self.hotkey_service.unregister_all()
        self.quit()

    def _trigger_health_check(self) -> bool:
        """Start a health check in a background thread."""
        if self._is_rewriting:
            return True  # Keep the timer alive but skip this check
        thread = threading.Thread(target=self._do_health_check, daemon=True)
        thread.start()
        return True  # Return True to keep GLib.timeout repeating

    def _do_health_check(self):
        """Run health check on background thread, update UI via GLib.idle_add."""
        if self.api_client is None:
            return
        status = self.api_client.check_health()
        if status != self._backend_status:
            logger.info("Health status: %s → %s", self._backend_status, status)
        GLib.idle_add(self._update_health_status, status)

    def _update_health_status(self, status: str) -> bool:
        """Update health status on the GTK main thread."""
        self._backend_status = status
        if self._tray_icon is not None:
            self._tray_icon.set_status(status)
        return False  # Don't repeat GLib.idle_add
