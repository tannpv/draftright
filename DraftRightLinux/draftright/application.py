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
from draftright.helpers.display_server import is_wayland, is_x11
from draftright.models.app_mode import AppMode
from draftright.models.health import HealthStatus
from draftright.models.rewrite import RewriteResult
from draftright.models.tone import Tone
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
from draftright.services.pencil_trigger import PencilTrigger
from draftright.services.rewrite_cache import RewriteCache
from draftright.services.settings_service import SettingsService
from draftright.ui import styles

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
        # X11-only on-selection trigger (#188); None until Pencil mode is active
        # on an X11 session. Mutually exclusive with the hotkey.
        self.pencil_trigger = None
        # The bubble that offers the rewrite; built on first highlight, then
        # re-shown, so highlights cannot stack windows.
        self.pencil_bubble = None
        self._pending_pencil_text = ""
        self._hotkey_active = False
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
        self._rewrite_panel = None
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

        # Load CSS. do_activate() runs again on every re-activation (including
        # the tray's own "Show"), so this must be idempotent — it used to add
        # another provider to the display each time.
        styles.ensure_resource_css_loaded()

        # Set up tray icon
        self._setup_tray()

        # Wire the trigger — the hotkey and, on X11 in Pencil mode, the
        # selection-polling pencil. Idempotent on re-activate.
        self.apply_trigger_mode()

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

        # Immediate first check, then on a fixed interval.
        #
        # This used to be timeout_add_seconds(0, ...) for the "immediate" one.
        # _trigger_health_check returns True to keep the interval timer alive,
        # so a 0-second timer re-armed itself instantly — measured at ~1.3M
        # calls per 10s, each spawning a network thread. That hammered the
        # backend and made the tray icon flicker between the app mark and the
        # offline warning as results flapped. Just call it once instead.
        self._trigger_health_check()
        GLib.timeout_add_seconds(config.HEALTH_CHECK_INTERVAL, self._trigger_health_check)

        # Start update check — shortly after launch
        GLib.timeout_add_seconds(10, self._trigger_update_check)

        # Post-update "What's New" — shortly after the window is up.
        GLib.timeout_add_seconds(2, self._trigger_whats_new_check)

    def _build_main_content(self):
        """Build the main window body (#101 — the window was presented empty).

        The app is hotkey-driven, so this screen's job is to report state:
        whether the backend is reachable, what the capture shortcut is, and
        whether that shortcut actually works on this display server.
        """
        # Adw.ApplicationWindow draws NO titlebar of its own — the header has
        # to be part of the content. Without this the window had no close
        # button at all, so once Settings closed there was no way out of the
        # app except the tray.
        root = Gtk.Box(orientation=Gtk.Orientation.VERTICAL)
        header = Adw.HeaderBar()
        header.set_title_widget(Adw.WindowTitle(title="DraftRight"))
        # Only meaningful inside an AdwNavigationView; this window is not in
        # one, so the arrow would be dead weight next to the title.
        header.set_show_back_button(False)
        root.append(header)

        box = Gtk.Box(orientation=Gtk.Orientation.VERTICAL, spacing=12)
        box.set_vexpand(True)
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

        # Closing the window keeps the app alive in the tray, which is what a
        # tray app should do — but say so, or the close button looks like Quit.
        hide_btn = Gtk.Button(label="Hide to tray")
        hide_btn.add_css_class("flat")
        hide_btn.set_tooltip_text("DraftRight keeps running in the top bar")
        hide_btn.connect("clicked", lambda _b: self.props.active_window.set_visible(False))
        box.append(hide_btn)

        root.append(box)
        return root

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

    def apply_trigger_mode(self):
        """Start the mechanism the user chose — pencil or hotkey — and only it.

        The two are mutually exclusive (matching macOS/Windows). The pencil is
        X11-only (Wayland forbids the global selection monitoring it needs, #103),
        so Pencil mode on Wayland falls back to the hotkey — the enum's safe
        default. Idempotent: safe to call again on every re-activate or when the
        Settings picker changes the mode.

        Public because Settings calls it: saving ``trigger_mode`` only records
        the choice, and the live mechanism has to be swapped for it to take
        effect without a restart.
        """
        if self.settings_service is None:
            return
        pencil_wanted = self.settings_service.trigger_mode.uses_pencil and is_x11()

        if pencil_wanted:
            if self.pencil_trigger is None and self.clipboard_service is not None:
                self.pencil_trigger = PencilTrigger(
                    # PRIMARY only — NOT get_selected_text, whose Ctrl+C fallback
                    # would fire into the focused window several times a second.
                    read_selection=self.clipboard_service.get_primary_selection,
                    # Offer, don't impose: a highlight raises the bubble, and
                    # only clicking it routes the text on (#188).
                    on_selection=self._offer_pencil_bubble,
                )
            if self.pencil_trigger is not None:
                self.pencil_trigger.start()
        elif self.pencil_trigger is not None:
            self.pencil_trigger.stop()
            self._dismiss_pencil_bubble()

        # The hotkey is the other half of the pair: live only when the pencil is
        # not (i.e. Hotkey mode, or anywhere the pencil can't run — Wayland).
        self._set_hotkey_active(not pencil_wanted)

    def _offer_pencil_bubble(self, text: str) -> None:
        """Show the pencil bubble at the pointer for *text* (#188).

        The bubble is built once and re-shown: a new one per highlight would
        stack windows the same way per-press panels used to. Clicking it hands
        the text to the shared chokepoint, so both triggers still end up in one
        place — the pencil just gains a confirmation step the hotkey (already an
        explicit action) does not need.
        """
        if not text:
            return
        if self.pencil_bubble is None:
            from draftright.ui.pencil_bubble import PencilBubble

            self.pencil_bubble = PencilBubble(on_click=self._on_pencil_bubble_clicked)
        self._pending_pencil_text = text
        self.pencil_bubble.show_at_pointer()

    def _on_pencil_bubble_clicked(self) -> None:
        """The user accepted the offer — route the text they had highlighted."""
        self._route_captured_text(self._pending_pencil_text)

    def _dismiss_pencil_bubble(self) -> None:
        """Take the offer down (mode switch, or quit)."""
        if self.pencil_bubble is not None:
            self.pencil_bubble.dismiss()

    def _set_hotkey_active(self, active: bool) -> None:
        """Register or release the global hotkey, tracking state to stay idempotent.

        Register/unregister must be balanced — re-registering an already-bound
        shortcut is the Windows 1408 ALREADY_REGISTERED trap (#186) — so a flag
        guards against double start/stop across re-activates.
        """
        if active and not self._hotkey_active:
            self._register_hotkey()
            self._hotkey_active = True
        elif not active and self._hotkey_active:
            if self.hotkey_service is not None:
                self.hotkey_service.stop()
            self._hotkey_active = False

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
        """Handle a global hotkey press — capture the selection and route it."""
        text = ""
        if self.clipboard_service:
            text = self.clipboard_service.get_selected_text()
        self._route_captured_text(text)

    def _route_captured_text(self, text: str) -> None:
        """Route captured text to the rewrite flow (RULE #1 chokepoint).

        The single entry point both triggers funnel through — the hotkey and the
        X11 pencil (#188) — so the app-mode routing lives in exactly one place.
        Advanced mode opens the panel so the user picks a tone; One-Click (#96)
        rewrites immediately with the preset tone and replaces the selection,
        matching macOS and Windows.
        """
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
        # Only the Translate tone depends on the language; sending it for the
        # others would fragment the cache for no benefit.
        selected = Tone.from_api_value(tone)
        language = (
            self.settings_service.translate_language
            if selected is not None and selected.uses_target_language
            else None
        )
        self._is_rewriting = True

        def worker():
            try:
                cached = self.rewrite_cache.get(text, tone, language)
                if cached is not None:
                    GLib.idle_add(self._finish_one_click, cached, None)
                    return
                payload = self.api_client.rewrite(text, tone, language)
                result = RewriteResult.from_wire(payload).text
                self.rewrite_cache.set(text, tone, result, language)
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
        # Reuse a single panel. Building a fresh one per hotkey press stacked
        # identical windows on top of each other, so closing the top one just
        # revealed the next — it looked like the panel would not close.
        # Windows does the same (App.cs: reuse _rewritePanel, update its text).
        if self._rewrite_panel is None:
            self._rewrite_panel = RewritePanel(self)
            self._rewrite_panel.connect("close-request", self._on_rewrite_panel_closed)
        self._rewrite_panel.show_with_text(text)

    def _on_rewrite_panel_closed(self, _panel) -> bool:
        """Drop the cached panel if the compositor destroys it."""
        self._rewrite_panel = None
        return False  # allow the default close

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
        if self.pencil_trigger is not None:
            self.pencil_trigger.stop()
        self._dismiss_pencil_bubble()
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

    @property
    def update_service(self) -> UpdateService:
        """The app's one UpdateService, built on first use.

        The single construction site: Settings' "Check for Updates" goes through
        this too, so the version and backend URL it checks against cannot drift
        from the automatic check. Issue #22 is what a second, hand-rolled copy
        of this construction costs.
        """
        if self._update_service is None:
            backend_url = (
                self.settings_service.backend_url
                if self.settings_service
                else config.LOCALHOST_BACKEND_URL
            )
            self._update_service = UpdateService(__version__, backend_url)
        return self._update_service

    def _do_update_check(self):
        """Run update check on background thread, show dialog via GLib.idle_add."""
        info = self.update_service.check_if_needed()
        if info is not None:
            GLib.idle_add(self._on_update_available, info)

    def _on_update_available(self, info) -> bool:
        """Badge the tray, then offer the update (#22)."""
        if self._tray_icon is not None:
            self._tray_icon.set_update_available(True)
        self.show_update_dialog(info)
        return False

    def _trigger_whats_new_check(self) -> bool:
        """Kick the one-time post-update notice in a background thread."""
        threading.Thread(target=self._do_whats_new_check, daemon=True).start()
        return False  # one-shot

    def _do_whats_new_check(self):
        """Background thread: if the running version changed since last launch,
        fetch its release notes and show them once."""
        if self.settings_service is None:
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
        notes = self.update_service.release_notes_for_version(current)
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

    def show_update_dialog(self, info) -> bool:
        """Show update dialog on the GTK main thread.

        Public because Settings' "Check for Updates" offers the same dialog for
        a manual check — one dialog, whichever check found the update.
        """
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
            success = self.update_service.download_and_install(info, progress_callback=on_progress)
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
        success = self.update_service.download_and_install(info)
        if success:
            GLib.idle_add(self._relaunch_after_update)

    def _relaunch_after_update(self) -> bool:
        """Relaunch the app after update."""
        self.update_service.relaunch()
        return False
