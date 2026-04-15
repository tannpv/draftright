"""Settings window using Adw.PreferencesWindow."""

import os
import threading

import gi
gi.require_version("Gtk", "4.0")
gi.require_version("Adw", "1")

from gi.repository import Adw, GLib, Gtk

from draftright.models.tone import Tone

# Languages available for translation
TRANSLATE_LANGUAGES = [
    "Arabic", "Bengali", "Chinese (Simplified)", "Chinese (Traditional)",
    "Czech", "Danish", "Dutch", "English", "Finnish", "French",
    "German", "Greek", "Hebrew", "Hindi", "Hungarian", "Indonesian",
    "Italian", "Japanese", "Korean", "Malay", "Norwegian", "Polish",
    "Portuguese", "Romanian", "Russian", "Spanish", "Swedish",
    "Thai", "Turkish", "Vietnamese",
]


class SettingsWindow(Adw.PreferencesWindow):
    """Preferences window for DraftRight.

    Provides two pages:
    - Account: sign in / register or view account details
    - Preferences: backend URL, hotkey, translate language, auto-start
    """

    def __init__(self, app):
        """Initialize the settings window.

        Args:
            app: The DraftRightApplication instance.
        """
        super().__init__()
        self.app = app
        self._register_mode = False

        self.set_title("DraftRight Settings")
        self.set_default_size(500, 600)
        self.set_transient_for(app.props.active_window)
        self.set_modal(True)

        self._build_account_page()
        self._build_preferences_page()
        self._refresh_account_ui()

    # ------------------------------------------------------------------
    # Account page
    # ------------------------------------------------------------------

    def _build_account_page(self):
        """Build the Account preferences page."""
        self._account_page = Adw.PreferencesPage(
            title="Account",
            icon_name="avatar-default-symbolic",
        )
        self.add(self._account_page)

        # --- Sign-in group ---
        self._signin_group = Adw.PreferencesGroup(title="Sign In")
        self._account_page.add(self._signin_group)

        self._email_row = Adw.EntryRow(title="Email")
        self._signin_group.add(self._email_row)

        self._password_row = Adw.PasswordEntryRow(title="Password")
        self._signin_group.add(self._password_row)

        self._name_row = Adw.EntryRow(title="Name")
        self._name_row.set_visible(False)
        self._signin_group.add(self._name_row)

        # Login / Register button
        self._auth_btn = Gtk.Button(label="Sign In")
        self._auth_btn.add_css_class("suggested-action")
        self._auth_btn.set_margin_top(8)
        self._auth_btn.connect("clicked", self._on_auth_clicked)
        self._signin_group.add(self._auth_btn)

        # Toggle link
        self._toggle_btn = Gtk.Button(label="Don't have an account? Register")
        self._toggle_btn.add_css_class("flat")
        self._toggle_btn.connect("clicked", self._on_toggle_mode)
        self._signin_group.add(self._toggle_btn)

        # Error label
        self._auth_error = Gtk.Label(label="")
        self._auth_error.add_css_class("error")
        self._auth_error.set_halign(Gtk.Align.START)
        self._auth_error.set_visible(False)
        self._signin_group.add(self._auth_error)

        # --- Logged-in group ---
        self._loggedin_group = Adw.PreferencesGroup(title="Account")
        self._loggedin_group.set_visible(False)
        self._account_page.add(self._loggedin_group)

        self._email_info_row = Adw.ActionRow(title="Email", subtitle="")
        self._loggedin_group.add(self._email_info_row)

        self._plan_info_row = Adw.ActionRow(title="Plan", subtitle="Free")
        self._loggedin_group.add(self._plan_info_row)

        self._signout_btn = Gtk.Button(label="Sign Out")
        self._signout_btn.add_css_class("destructive-action")
        self._signout_btn.set_margin_top(8)
        self._signout_btn.connect("clicked", self._on_sign_out)
        self._loggedin_group.add(self._signout_btn)

    # ------------------------------------------------------------------
    # Preferences page
    # ------------------------------------------------------------------

    def _build_preferences_page(self):
        """Build the Preferences page."""
        prefs_page = Adw.PreferencesPage(
            title="Preferences",
            icon_name="emblem-system-symbolic",
        )
        self.add(prefs_page)

        # --- Connection group ---
        conn_group = Adw.PreferencesGroup(title="Connection")
        prefs_page.add(conn_group)

        self._url_row = Adw.EntryRow(title="Backend URL")
        self._url_row.set_text(self._get_setting("backend-url", "https://api.draftright.app"))
        self._url_row.connect("changed", self._on_url_changed)
        conn_group.add(self._url_row)

        # --- Behavior group ---
        behavior_group = Adw.PreferencesGroup(title="Behavior")
        prefs_page.add(behavior_group)

        # Hotkey display
        hotkey_row = Adw.ActionRow(
            title="Hotkey",
            subtitle=self._get_setting("hotkey", "Ctrl+Shift+R"),
        )
        hotkey_row.set_activatable(False)
        behavior_group.add(hotkey_row)

        # Translate language
        self._lang_model = Gtk.StringList.new(TRANSLATE_LANGUAGES)
        self._lang_row = Adw.ComboRow(
            title="Translate Language",
            model=self._lang_model,
        )
        # Set current selection
        current_lang = self._get_setting("translate-language", "Spanish")
        for i, lang in enumerate(TRANSLATE_LANGUAGES):
            if lang == current_lang:
                self._lang_row.set_selected(i)
                break
        self._lang_row.connect("notify::selected", self._on_lang_changed)
        behavior_group.add(self._lang_row)

        # Auto-start
        self._autostart_row = Adw.SwitchRow(title="Auto-start on login")
        self._autostart_row.set_active(
            self._get_setting("auto-start", False)
        )
        self._autostart_row.connect("notify::active", self._on_autostart_changed)
        behavior_group.add(self._autostart_row)

        # --- Panel Tones group ---
        tones_group = Adw.PreferencesGroup(
            title="Panel Tones",
            description="Choose which tones appear in the rewrite panel",
        )
        prefs_page.add(tones_group)

        enabled_tones = (
            self.app.settings_service.enabled_tones
            if self.app.settings_service
            else [t.api_value for t in Tone]
        )
        self._tone_switches: dict[str, Adw.SwitchRow] = {}

        for tone in Tone:
            row = Adw.SwitchRow(
                title=f"{tone.icon}  {tone.display_name}",
                subtitle=tone.description,
            )
            row.set_active(tone.api_value in enabled_tones)
            row.connect("notify::active", self._on_tone_toggled, tone.api_value)
            tones_group.add(row)
            self._tone_switches[tone.api_value] = row

        # Default tone dropdown
        default_tone_values = [""] + [t.api_value for t in Tone]
        default_tone_labels = ["None (manual)"] + [
            f"{t.icon}  {t.display_name}" for t in Tone
        ]
        self._default_tone_model = Gtk.StringList.new(default_tone_labels)
        self._default_tone_values = default_tone_values

        self._default_tone_row = Adw.ComboRow(
            title="Default Tone",
            subtitle="Auto-run this tone when the panel opens",
            model=self._default_tone_model,
        )
        current_default = (
            self.app.settings_service.default_tone
            if self.app.settings_service
            else ""
        )
        for i, val in enumerate(default_tone_values):
            if val == current_default:
                self._default_tone_row.set_selected(i)
                break
        self._default_tone_row.connect(
            "notify::selected", self._on_default_tone_changed
        )
        tones_group.add(self._default_tone_row)

        # --- Updates group ---
        updates_group = Adw.PreferencesGroup(title="Updates")

        version_row = Adw.ActionRow(title="Version", subtitle="1.0.0")
        updates_group.add(version_row)

        check_update_row = Adw.ActionRow(title="Check for Updates", activatable=True)
        check_update_row.add_suffix(Gtk.Image.new_from_icon_name("emblem-synchronizing-symbolic"))
        check_update_row.connect("activated", self._on_check_updates)
        updates_group.add(check_update_row)

        prefs_page.add(updates_group)

        # --- Logs group ---
        logs_group = Adw.PreferencesGroup(title="Logs")
        prefs_page.add(logs_group)

        from draftright.services.logger import get_log_path
        log_row = Adw.ActionRow(
            title="Log File",
            subtitle=get_log_path(),
        )
        open_log_btn = Gtk.Button(label="Open", valign=Gtk.Align.CENTER)
        open_log_btn.connect("clicked", self._on_open_log)
        log_row.add_suffix(open_log_btn)
        logs_group.add(log_row)

    # ------------------------------------------------------------------
    # Helpers
    # ------------------------------------------------------------------

    def _get_setting(self, key: str, default=None):
        """Read a setting from the app's settings service.

        Args:
            key: The setting key.
            default: Fallback value if the setting is not found.

        Returns:
            The setting value or the default.
        """
        if self.app.settings_service:
            return self.app.settings_service.get(key, default)
        return default

    def _save_setting(self, key: str, value):
        """Persist a setting.

        Args:
            key: The setting key.
            value: The value to store.
        """
        if self.app.settings_service:
            self.app.settings_service.set(key, value)

    def _refresh_account_ui(self):
        """Update the account page to reflect current auth state."""
        logged_in = (
            self.app.auth_service is not None
            and self.app.auth_service.is_authenticated()
        )

        self._signin_group.set_visible(not logged_in)
        self._loggedin_group.set_visible(logged_in)

        if logged_in:
            user = self.app.auth_service.get_user()
            self._email_info_row.set_subtitle(user.get("email", ""))
            self._plan_info_row.set_subtitle(user.get("plan", "Free"))

    # ------------------------------------------------------------------
    # Callbacks
    # ------------------------------------------------------------------

    def _on_toggle_mode(self, _button):
        """Toggle between sign-in and register mode."""
        self._register_mode = not self._register_mode
        self._name_row.set_visible(self._register_mode)
        self._auth_error.set_visible(False)

        if self._register_mode:
            self._auth_btn.set_label("Register")
            self._toggle_btn.set_label("Already have an account? Sign in")
        else:
            self._auth_btn.set_label("Sign In")
            self._toggle_btn.set_label("Don't have an account? Register")

    def _on_auth_clicked(self, _button):
        """Handle login or register button click."""
        email = self._email_row.get_text().strip()
        password = self._password_row.get_text().strip()
        name = self._name_row.get_text().strip() if self._register_mode else None

        if not email or not password:
            self._show_auth_error("Email and password are required.")
            return

        if self._register_mode and not name:
            self._show_auth_error("Name is required for registration.")
            return

        self._auth_btn.set_sensitive(False)
        self._auth_error.set_visible(False)

        def _do_auth():
            try:
                if self.app.auth_service is None:
                    raise RuntimeError("Auth service not available.")

                if self._register_mode:
                    self.app.auth_service.register(name, email, password)
                else:
                    self.app.auth_service.login(email, password)

                GLib.idle_add(self._on_auth_success)
            except Exception as exc:
                GLib.idle_add(self._on_auth_failure, str(exc))

        thread = threading.Thread(target=_do_auth, daemon=True)
        thread.start()

    def _on_auth_success(self):
        """Handle successful authentication on the main thread."""
        self._auth_btn.set_sensitive(True)
        self._email_row.set_text("")
        self._password_row.set_text("")
        self._name_row.set_text("")
        self._refresh_account_ui()

    def _on_auth_failure(self, message: str):
        """Handle authentication failure on the main thread.

        Args:
            message: The error message.
        """
        self._auth_btn.set_sensitive(True)
        self._show_auth_error(message)

    def _show_auth_error(self, message: str):
        """Display an error message in the auth section.

        Args:
            message: The error text.
        """
        self._auth_error.set_text(message)
        self._auth_error.set_visible(True)

    def _on_sign_out(self, _button):
        """Sign the user out and refresh the UI."""
        self.app.sign_out()
        self._refresh_account_ui()

    def _on_url_changed(self, row):
        """Persist the backend URL change."""
        self._save_setting("backend-url", row.get_text().strip())

    def _on_lang_changed(self, row, _pspec):
        """Persist the translate language change."""
        idx = row.get_selected()
        if 0 <= idx < len(TRANSLATE_LANGUAGES):
            self._save_setting("translate-language", TRANSLATE_LANGUAGES[idx])

    def _on_autostart_changed(self, row, _pspec):
        """Persist the auto-start preference."""
        self._save_setting("auto-start", row.get_active())

    def _on_tone_toggled(self, row, _pspec, tone_api_value: str):
        """Persist the enabled/disabled state of a tone."""
        if not self.app.settings_service:
            return
        enabled = list(self.app.settings_service.enabled_tones)
        if row.get_active():
            if tone_api_value not in enabled:
                enabled.append(tone_api_value)
        else:
            if tone_api_value in enabled:
                enabled.remove(tone_api_value)
        self.app.settings_service.enabled_tones = enabled
        self.app.settings_service.save()

    def _on_default_tone_changed(self, row, _pspec):
        """Persist the default tone selection."""
        if not self.app.settings_service:
            return
        idx = row.get_selected()
        if 0 <= idx < len(self._default_tone_values):
            self.app.settings_service.default_tone = self._default_tone_values[idx]
            self.app.settings_service.save()

    def _on_check_updates(self, row):
        """Handle Check for Updates click — reuses the app's UpdateService."""
        app = self.app
        svc = getattr(app, "_update_service", None)
        if svc is None:
            # Fallback: create one if the app hasn't initialised it yet
            from draftright.services.update_service import UpdateService
            backend_url = "http://localhost:3000"
            if app.settings_service:
                backend_url = app.settings_service.backend_url
            svc = UpdateService("1.0.0", backend_url)

        has_update, result = svc.check_now()

        if has_update and hasattr(app, '_show_update_dialog'):
            app._show_update_dialog(result)
        else:
            dialog = Gtk.MessageDialog(
                transient_for=self,
                modal=True,
                message_type=Gtk.MessageType.INFO,
                text="No Updates Available",
            )
            dialog.set_property("secondary-text", result if isinstance(result, str) else "You're up to date.")
            dialog.add_button("OK", Gtk.ResponseType.OK)
            dialog.connect("response", lambda d, _: d.destroy())
            dialog.present()

    def _on_open_log(self, _button):
        """Open the log file location."""
        from draftright.services.logger import get_log_path
        import subprocess
        log_path = get_log_path()
        subprocess.Popen(["xdg-open", os.path.dirname(log_path)])
