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

from draftright import config
from draftright.helpers.display_server import is_wayland
from draftright.models.app_mode import AppMode
from draftright.models.health import HealthStatus
from draftright.models.rewrite import RewriteResult
from draftright.models.tray import TrayAction
from draftright.__version__ import __version__
from draftright.services.logger import setup_logging
from draftright.services.update_service import UpdateService
from draftright.services import error_reporter
from draftright.services.api_client import APIClient
from draftright.services.auth_service import AuthService
from draftright.services.clipboard_service import ClipboardService
from draftright.services.hotkey_service import HotkeyService
from draftright.services.input_portal import RemoteDesktopInjector
from draftright.services.rewrite_cache import RewriteCache
from draftright.services.settings_service import SettingsService

# Wire crash reporting as early as possible — sys.excepthook covers
# anything that throws after this point.
error_reporter.configure(backend_url=config.default_backend_url())

logger = logging.getLogger(__name__)


class DraftRightApplication(Adw.Application):
    """Main application class for DraftRight Linux."""

    def __init__(self):
        setup_logging()

        super().__init__(
            application_id=config.APP_ID,
            flags=Gio.ApplicationFlags.FLAGS_NONE,
        )

        # Service references (initialized in do_activate)
        self.api_client = None
        self.auth_service = None
        self.settings_service = None
        self.hotkey_service = None
        self.clipboard_service = None
        self.input_injector = None
        # Client-side rewrite cache, shared across panel instances (a fresh
        # RewritePanel is built per hotkey press). Mirrors macOS.
        self.rewrite_cache = RewriteCache()

        # Actions the GTK3 tray helper activates over the session bus.  The
        # names come from TrayAction so the two processes cannot drift apart;
        # registering here (not in do_activate) means they exist as soon as
        # the app owns its bus name.
        handlers = {
            TrayAction.SHOW: self._on_show_action,
            TrayAction.SETTINGS: self._on_settings_action,
            TrayAction.SUGGEST_FEATURE: self._on_suggest_feature_action,
            TrayAction.REPORT_BUG: self._on_report_bug_action,
            TrayAction.SIGN_OUT: self._on_sign_out_action,
            TrayAction.QUIT: self._on_quit_action,
        }
        missing = [a.value for a in TrayAction if a not in handlers]
        if missing:  # a new TrayAction member without a handler here
            raise RuntimeError(f"TrayAction(s) with no handler: {missing}")
        for tray_action, handler in handlers.items():
            action = Gio.SimpleAction.new(tray_action.value, None)
            action.connect("activate", handler)
            self.add_action(action)

        self._backend_status = HealthStatus.OFFLINE
        self._is_rewriting = False
        self._tray_icon = None
        self._settings_window = None
        self._status_label = None
        self._hotkey_label = None
        # None + not resolved = still asking the portal; None + resolved =
        # the compositor refused.  Distinguishing the two keeps the UI from
        # saying "declined" while the request is still in flight.
        self._active_trigger = None
        self._hotkey_resolved = False
        self._last_auto_recovery = 0.0
        self._update_service = None

    def do_activate(self):
        """Called when the application is activated."""
        # Name the window icon so the shell shows the DraftRight icon rather
        # than a generic fallback; resolves against the installed hicolor set.
        Gtk.Window.set_default_icon_name(config.APP_ID)

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
            # __init__ only seeds defaults — without this the app ignores the
            # user's saved backend_url / hotkey / tones, and the first
            # settings write would persist defaults over their file.
            self.settings_service.load()
        if self.api_client is None:
            self.api_client = APIClient(self.settings_service.backend_url)
        if self.auth_service is None:
            self.auth_service = AuthService(self.api_client)
        # Input services back the core rewrite loop: grab the selection on
        # hotkey, inject the result back. Idempotent on re-activate.
        if self.clipboard_service is None:
            # On Wayland, Replace has to synthesise Ctrl+V through the
            # RemoteDesktop portal; xdotool only reaches XWayland clients.
            if is_wayland() and self.input_injector is None:
                self.input_injector = RemoteDesktopInjector(self.settings_service)
            self.clipboard_service = ClipboardService(injector=self.input_injector)
        if self.hotkey_service is None:
            self.hotkey_service = HotkeyService()
        # Let the API client recover from a 401 by refreshing the session,
        # mirroring the macOS / Windows / mobile behaviour (issue #22).
        self.api_client.on_unauthorized = self.auth_service.refresh_session

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
            win.set_content(self._build_main_content())
        win.present()

        # Start health check — immediate first check, then on a fixed interval.
        GLib.timeout_add_seconds(0, self._trigger_health_check)
        GLib.timeout_add_seconds(config.HEALTH_CHECK_INTERVAL, self._trigger_health_check)

        # Start update check — shortly after launch
        backend_url = (
            self.settings_service.backend_url
            if self.settings_service
            else config.LOCALHOST_BACKEND_URL
        )
        self._update_service = UpdateService(__version__, backend_url)
        GLib.timeout_add_seconds(10, self._trigger_update_check)

        # Post-update "What's New" — shortly after the window is up.
        GLib.timeout_add_seconds(2, self._trigger_whats_new_check)

    def _build_main_content(self):
        """Build the main window body (#101 — the window was presented empty).

        The app is hotkey-driven, so this screen's job is to report state:
        whether the backend is reachable, what the capture shortcut is, and
        whether that shortcut actually works on this display server.
        """
        box = Gtk.Box(orientation=Gtk.Orientation.VERTICAL, spacing=12)
        box.set_margin_top(36)
        box.set_margin_bottom(36)
        box.set_margin_start(32)
        box.set_margin_end(32)

        title = Gtk.Label(label="DraftRight")
        title.add_css_class("title-1")
        box.append(title)

        subtitle = Gtk.Label(label="AI-powered text rewriting")
        subtitle.add_css_class("dim-label")
        box.append(subtitle)

        self._status_label = Gtk.Label(label="Checking connection…")
        self._status_label.set_margin_top(16)
        box.append(self._status_label)

        self._hotkey_label = Gtk.Label(label=self._hotkey_hint())
        self._hotkey_label.add_css_class("dim-label")
        self._hotkey_label.set_wrap(True)
        self._hotkey_label.set_justify(Gtk.Justification.CENTER)
        box.append(self._hotkey_label)

        rewrite_btn = Gtk.Button(label="Rewrite clipboard text")
        rewrite_btn.add_css_class("suggested-action")
        rewrite_btn.add_css_class("pill")
        rewrite_btn.set_margin_top(16)
        rewrite_btn.connect("clicked", self._on_rewrite_clipboard_clicked)
        box.append(rewrite_btn)

        settings_btn = Gtk.Button(label="Settings")
        settings_btn.add_css_class("pill")
        settings_btn.connect("clicked", lambda _b: self.show_settings())
        box.append(settings_btn)

        return box

    def _hotkey_hint(self) -> str:
        """Describe the capture shortcut honestly for the current session.

        On Wayland the compositor owns the binding: it may prompt, refuse, or
        substitute a different trigger, so report what it granted rather than
        what was requested (#99).
        """
        keystring = self.settings_service.hotkey if self.settings_service else ""
        if not is_wayland():
            return f"Select text anywhere, then press {keystring}"
        if self._active_trigger:
            # The portal hands back a localised, human-readable description
            # (GNOME: "Press <Shift><Control>r"), so lead in neutrally rather
            # than prefixing another "press".
            return f"Select text anywhere — {self._active_trigger}"
        if self._hotkey_resolved:
            return (
                "Your desktop declined the global shortcut — "
                "use the button below, or grant it in system settings."
            )
        return f"Requesting the global shortcut ({keystring}) from your desktop…"

    def _on_rewrite_clipboard_clicked(self, _button):
        """Rewrite whatever is on the clipboard — the Wayland-safe entry point."""
        text = ""
        if self.clipboard_service:
            text = self.clipboard_service.get_clipboard()
        if text:
            self.show_rewrite_panel(text)
        elif self._status_label is not None:
            self._status_label.set_text("Clipboard is empty — copy some text first.")

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
        """Set up the system tray icon (idempotent).

        do_activate() runs again whenever the app is re-activated — including
        from the tray's own "Show" action — so without this guard each
        re-activation would spawn another helper process.
        """
        if self._tray_icon is not None:
            return
        from draftright.ui.tray_icon import TrayIcon
        self._tray_icon = TrayIcon(self)

    def _register_hotkey(self):
        """Register the global hotkey for text capture.

        Fully wired on X11 (python-xlib XGrabKey); Wayland uses HotkeyService's
        best-effort fallback until the portal path lands (#99). The callback is
        marshalled onto the GTK main thread by HotkeyService.
        """
        if self.hotkey_service is None or self.settings_service is None:
            return
        keystring = self.settings_service.hotkey
        try:
            # Wayland binding is asynchronous and the compositor may refuse or
            # remap it, so success is reported by the callback — not here.
            # Logging "registered" at this point was how the old build claimed
            # a working hotkey while the listener could never fire (#99).
            self.hotkey_service.on_trigger_changed = self._on_hotkey_trigger_changed
            self.hotkey_service.start(keystring, self.on_hotkey_pressed)
            logger.info("Requested global hotkey: %s", keystring)
        except Exception as exc:
            logger.warning("Failed to register hotkey %s: %s", keystring, exc)

    def _on_hotkey_trigger_changed(self, trigger):
        """Reflect the trigger the compositor actually bound (Wayland, #99)."""
        if trigger:
            logger.info("Global hotkey is live: %s", trigger)
        else:
            logger.warning(
                "No global shortcut is bound — the tray and window still work."
            )
        self._active_trigger = trigger
        self._hotkey_resolved = True
        if self._hotkey_label is not None:
            self._hotkey_label.set_text(self._hotkey_hint())

    def _restore_session(self):
        """Restore saved authentication session."""
        if self.auth_service:
            self.auth_service.restore_session()

    # ------------------------------------------------------------------
    # Public API
    # ------------------------------------------------------------------

    def on_hotkey_pressed(self):
        """Handle a global hotkey press.

        Advanced mode opens the panel so the user picks a tone; One-Click
        (#96) rewrites immediately with the preset tone and replaces the
        selection, matching macOS and Windows.
        """
        text = ""
        if self.clipboard_service:
            text = self.clipboard_service.get_selected_text()
        if not text:
            return

        mode = self.settings_service.app_mode if self.settings_service else AppMode.ADVANCED
        if mode is AppMode.ONE_CLICK:
            self.run_one_click_rewrite(text)
        else:
            self.show_rewrite_panel(text)

    def run_one_click_rewrite(self, text: str):
        """Rewrite *text* with the preset tone and replace the selection (#96).

        Runs without any window, so failures have nowhere to render — they go
        to a desktop notification instead of being swallowed.
        """
        if self.api_client is None or self._is_rewriting:
            return
        tone = self.settings_service.one_click_tone
        self._is_rewriting = True

        def worker():
            try:
                cached = self.rewrite_cache.get(text, tone)
                if cached is not None:
                    GLib.idle_add(self._finish_one_click, cached, None)
                    return
                payload = self.api_client.rewrite(text, tone)
                result = RewriteResult.from_wire(payload).text
                self.rewrite_cache.set(text, tone, result)
                GLib.idle_add(self._finish_one_click, result, None)
            except Exception as exc:  # noqa: BLE001 — surface any failure
                GLib.idle_add(self._finish_one_click, None, str(exc))

        threading.Thread(target=worker, daemon=True).start()

    def _finish_one_click(self, result, error) -> bool:
        """Inject the rewrite, or tell the user why it didn't happen."""
        self._is_rewriting = False
        if error is not None:
            logger.warning("One-Click rewrite failed: %s", error)
            self._notify("Rewrite failed", error)
            return False

        def on_injected(delivered: bool):
            if not delivered:
                # The text is on the clipboard either way — say so, since
                # there is no panel to show a fallback button.
                self._notify(
                    "Rewrite ready",
                    "Couldn't paste automatically — press Ctrl+V to insert it.",
                )

        self.clipboard_service.inject_text(result, on_done=on_injected)
        return False

    def _notify(self, title: str, body: str) -> None:
        """Send a desktop notification (One-Click has no window to write to)."""
        notification = Gio.Notification.new(title)
        notification.set_body(body)
        self.send_notification("one-click", notification)

    def show_rewrite_panel(self, text: str):
        """Open the rewrite panel with the captured text.

        Args:
            text: The selected text to rewrite.
        """
        from draftright.ui.rewrite_panel import RewritePanel

        if not text:
            return
        panel = RewritePanel(self)
        panel.show_with_text(text)

    def show_settings(self):
        """Open the settings window."""
        from draftright.ui.settings_window import SettingsWindow

        # Reuse an already-open settings window instead of stacking duplicates.
        if getattr(self, "_settings_window", None) is not None:
            try:
                self._settings_window.present()
                return
            except Exception:
                self._settings_window = None
        window = SettingsWindow(self)
        window.connect("close-request", self._on_settings_closed)
        self._settings_window = window
        window.present()

    def _on_settings_closed(self, _window) -> bool:
        """Drop the cached settings window so the next open builds fresh."""
        self._settings_window = None
        return False  # allow the default close

    # ------------------------------------------------------------------
    # Tray actions (activated over D-Bus by the GTK3 helper)
    # ------------------------------------------------------------------

    def _on_show_action(self, _action, _param):
        """Bring the main window back, recreating it if it was closed."""
        window = self.props.active_window
        if window is None:
            self.activate()  # rebuilds and presents the main window
        else:
            window.present()

    def _on_settings_action(self, _action, _param):
        if self.props.active_window is None:
            self.activate()  # services must exist before Settings can build
        self.show_settings()

    def _on_suggest_feature_action(self, _action, _param):
        from draftright.ui.suggest_feature_dialog import open_suggest_feature_dialog

        token = self.auth_service.access_token if self.auth_service else None
        open_suggest_feature_dialog(self.props.active_window, bearer_token=token)

    def _on_report_bug_action(self, _action, _param):
        from draftright.ui.report_bug_dialog import open_report_bug_dialog

        token = self.auth_service.access_token if self.auth_service else None
        email = self.auth_service.get_user().get("email") if self.auth_service else None
        open_report_bug_dialog(
            self.props.active_window, bearer_token=token, user_email=email
        )

    def _on_sign_out_action(self, _action, _param):
        self.sign_out()

    def _on_quit_action(self, _action, _param):
        self.quit_app()

    def sign_out(self):
        """Clear auth session and notify the user."""
        if self.auth_service:
            self.auth_service.logout()
        # Drop cached rewrites so a different user can't read the prior
        # user's results from memory.
        self.rewrite_cache.clear()

        notification = Gio.Notification.new("DraftRight")
        notification.set_body("You have been signed out.")
        self.send_notification("sign-out", notification)

    def quit_app(self):
        """Clean up resources and quit the application."""
        if self.hotkey_service:
            self.hotkey_service.stop()
        if self.input_injector is not None:
            self.input_injector.stop()
        if self._tray_icon is not None:
            # Reap the helper explicitly; it also self-exits on stdin EOF, but
            # that only fires once this process is gone.
            self._tray_icon.stop()
            self._tray_icon = None
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
            logger.info("Health status: %s → %s",
                        self._backend_status.value, status.value)
        GLib.idle_add(self._update_health_status, status)

    def _update_health_status(self, status: HealthStatus) -> bool:
        """Update health status on the GTK main thread."""
        self._backend_status = status
        if self._tray_icon is not None:
            self._tray_icon.set_status(status)
        if self._status_label is not None:
            self._status_label.set_text(status.display_name)

        # Auto-recovery: if offline and targeting localhost, try to start the backend
        backend_url = ""
        if self.settings_service:
            backend_url = self.settings_service.backend_url
        if status is HealthStatus.OFFLINE and "localhost" in backend_url:
            self._attempt_auto_recovery()

        return False  # Don't repeat GLib.idle_add

    def _attempt_auto_recovery(self) -> None:
        """Run start-server.sh to bring up Docker services. Throttled to once per 2 minutes."""
        now = time.monotonic()
        if now - self._last_auto_recovery < config.AUTO_RECOVERY_COOLDOWN:
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
                env = dict(
                    os.environ,
                    PATH=config.SUBPROCESS_PATH_PREFIX + ":" + os.environ.get("PATH", ""),
                )
                result = subprocess.run(
                    [str(script)],
                    capture_output=True, text=True,
                    timeout=config.AUTO_RECOVERY_TIMEOUT, env=env,
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
