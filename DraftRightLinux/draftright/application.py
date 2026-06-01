"""DraftRight GTK4 Application."""

import gi
gi.require_version("Gtk", "4.0")
gi.require_version("Adw", "1")

from gi.repository import Adw, Gio, Gtk, GLib

import os
import subprocess
import threading
import time
import logging
from pathlib import Path

from draftright.__version__ import __version__
from draftright.services.logger import setup_logging
from draftright.services.update_service import UpdateService
from draftright.services import error_reporter
from draftright.services.api_client import APIClient
from draftright.services.auth_service import AuthService
from draftright.services.settings_service import SettingsService

# Wire crash reporting as early as possible — sys.excepthook covers
# anything that throws after this point.
error_reporter.configure(backend_url="https://api.draftright.info")

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
        self._last_auto_recovery = 0.0
        self._update_service = None

    def do_activate(self):
        """Called when the application is activated."""
        # Force dark color scheme
        style_manager = Adw.StyleManager.get_default()
        style_manager.set_color_scheme(Adw.ColorScheme.FORCE_DARK)

        # Wire core services on first activate.  These were declared in
        # __init__ but never instantiated — every consumer was guarding
        # `if app.api_client is None` and falling back, which left the
        # Subscription page permanently in "Not signed in" state.
        # Lazy-init is idempotent on subsequent activates (e.g. when
        # the user reopens via the tray) so the dock-style relaunch
        # doesn't churn instances.
        if self.settings_service is None:
            self.settings_service = SettingsService()
        if self.api_client is None:
            self.api_client = APIClient(self.settings_service.backend_url)
        if self.auth_service is None:
            self.auth_service = AuthService(self.api_client)

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

        # Start update check — 10 seconds after launch
        backend_url = self.settings_service.backend_url if self.settings_service else "http://localhost:3000"
        self._update_service = UpdateService(__version__, backend_url)
        GLib.timeout_add_seconds(10, self._trigger_update_check)

        # Post-update "What's New" — shortly after the window is up.
        GLib.timeout_add_seconds(2, self._trigger_whats_new_check)

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

        # Auto-recovery: if offline and targeting localhost, try to start the backend
        backend_url = ""
        if self.settings_service:
            backend_url = self.settings_service.backend_url
        if status == "offline" and "localhost" in backend_url:
            self._attempt_auto_recovery()

        return False  # Don't repeat GLib.idle_add

    def _attempt_auto_recovery(self) -> None:
        """Run start-server.sh to bring up Docker services. Throttled to once per 2 minutes."""
        now = time.monotonic()
        if now - self._last_auto_recovery < 120:
            return
        self._last_auto_recovery = now

        # Look for start-server.sh in the DraftRightLinux directory
        script = Path(__file__).resolve().parent.parent / "start-server.sh"
        if not script.is_file():
            logger.debug("Auto-recovery: start-server.sh not found at %s", script)
            return

        logger.info("Auto-recovery: running %s", script)

        def _run():
            try:
                env = dict(os.environ, PATH="/usr/local/bin:/usr/bin:/bin:" + os.environ.get("PATH", ""))
                result = subprocess.run(
                    [str(script)],
                    capture_output=True, text=True, timeout=60, env=env,
                )
                logger.info("Auto-recovery: exit code %d", result.returncode)
            except Exception as exc:
                logger.warning("Auto-recovery failed: %s", exc)

        threading.Thread(target=_run, daemon=True).start()

    def _trigger_update_check(self) -> bool:
        """Check for updates in a background thread."""
        thread = threading.Thread(target=self._do_update_check, daemon=True)
        thread.start()
        return False  # Don't repeat — health check timer handles periodic checks

    def _do_update_check(self):
        """Run update check on background thread, show dialog via GLib.idle_add."""
        if self._update_service is None:
            return
        info = self._update_service.check_if_needed()
        if info is not None:
            GLib.idle_add(self._show_update_dialog, info)

    def _trigger_whats_new_check(self) -> bool:
        """Kick the one-time post-update notice in a background thread."""
        threading.Thread(target=self._do_whats_new_check, daemon=True).start()
        return False  # one-shot

    def _do_whats_new_check(self):
        """Background thread: if the running version changed since last launch,
        fetch its release notes and show them once."""
        if self._update_service is None or self.settings_service is None:
            return
        current = __version__
        last_seen = self.settings_service.last_seen_version
        if last_seen == current:
            return
        # Record now so the notice can't repeat; skip on a fresh install.
        self.settings_service.last_seen_version = current
        self.settings_service.save()
        if not last_seen:
            return
        notes = self._update_service.release_notes_for_version(current)
        if notes:
            GLib.idle_add(self._show_whats_new_dialog, current, notes)

    def _show_whats_new_dialog(self, version, notes) -> bool:
        """Show the 'What's New' notice on the GTK main thread."""
        dialog = Gtk.MessageDialog(
            transient_for=self.props.active_window,
            modal=True,
            message_type=Gtk.MessageType.INFO,
            text=f"What's new in DraftRight v{version}",
        )
        dialog.set_property("secondary-text", notes)
        dialog.add_button("Got it", Gtk.ResponseType.OK)
        dialog.connect("response", lambda d, _r: d.destroy())
        dialog.present()
        return False

    def _show_update_dialog(self, info) -> bool:
        """Show update dialog on the GTK main thread."""
        dialog = Gtk.MessageDialog(
            transient_for=self.props.active_window,
            modal=True,
            message_type=Gtk.MessageType.INFO,
            text=f"DraftRight v{info.version} is available",
        )
        dialog.set_property("secondary-text", info.release_notes)

        dialog.add_button("Install Now", Gtk.ResponseType.YES)
        if not info.required:
            dialog.add_button("Later", Gtk.ResponseType.NO)

        def on_response(d, response_id):
            d.destroy()
            if response_id == Gtk.ResponseType.YES:
                self._show_progress_and_install(info)

        dialog.connect("response", on_response)
        dialog.present()
        return False

    def _show_progress_and_install(self, info):
        """Show a progress window and download the update."""
        # Create progress window
        progress_win = Gtk.Window(
            title="Updating DraftRight",
            transient_for=self.props.active_window,
            modal=True,
            default_width=350,
            default_height=120,
            resizable=False,
        )
        box = Gtk.Box(orientation=Gtk.Orientation.VERTICAL, spacing=10)
        box.set_margin_top(20)
        box.set_margin_bottom(20)
        box.set_margin_start(20)
        box.set_margin_end(20)

        status_label = Gtk.Label(label=f"Downloading DraftRight v{info.version}...")
        status_label.set_xalign(0)
        box.append(status_label)

        progress_bar = Gtk.ProgressBar()
        progress_bar.set_show_text(True)
        progress_bar.set_text("0%")
        box.append(progress_bar)

        progress_win.set_child(box)
        progress_win.present()

        def on_progress(fraction, status):
            GLib.idle_add(self._update_progress_ui, progress_bar, status_label, fraction, status)

        def do_download():
            if self._update_service is None:
                GLib.idle_add(progress_win.destroy)
                return
            success = self._update_service.download_and_install(info, progress_callback=on_progress)
            if success:
                GLib.idle_add(progress_win.destroy)
                GLib.idle_add(self._relaunch_after_update)
            else:
                GLib.idle_add(progress_win.destroy)
                GLib.idle_add(self._show_update_error)

        threading.Thread(target=do_download, daemon=True).start()

    def _update_progress_ui(self, progress_bar, status_label, fraction, status) -> bool:
        """Update progress UI on the GTK main thread."""
        progress_bar.set_fraction(fraction)
        progress_bar.set_text(f"{int(fraction * 100)}%")
        status_label.set_text(status)
        return False

    def _show_update_error(self) -> bool:
        """Show error dialog if update failed."""
        dialog = Gtk.MessageDialog(
            transient_for=self.props.active_window,
            modal=True,
            message_type=Gtk.MessageType.ERROR,
            text="Update Failed",
        )
        dialog.set_property("secondary-text", "Could not install the update. Please try again later.")
        dialog.add_button("OK", Gtk.ResponseType.OK)
        dialog.connect("response", lambda d, _: d.destroy())
        dialog.present()
        return False

    def _install_update(self, info):
        """Download and install update in background thread."""
        if self._update_service is None:
            return
        success = self._update_service.download_and_install(info)
        if success:
            GLib.idle_add(self._relaunch_after_update)

    def _relaunch_after_update(self) -> bool:
        """Relaunch the app after update."""
        if self._update_service:
            self._update_service.relaunch()
        return False
