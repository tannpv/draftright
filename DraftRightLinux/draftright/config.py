"""Central configuration + constants for DraftRight Linux.

Single source of truth for values that were previously hardcoded across
service and UI modules (backend URL, timeouts, intervals, magic numbers).
Rule #1: no scattered literals — import from here.

The backend URL resolves with this precedence:
  1. ``DRAFTRIGHT_BACKEND`` environment variable (dev/test/prod swap, no rebuild)
  2. the persisted value in ``~/.config/draftright/settings.json``
     (see :class:`SettingsService` — the app uses JSON, not GSettings)
  3. :data:`DEFAULT_BACKEND_URL`
Modules that have a SettingsService should prefer its ``backend_url``; those
that run before settings exist (e.g. the early crash reporter) use
:func:`default_backend_url`.
"""

import os

# ── Backend ────────────────────────────────────────────────────────────────
DEFAULT_BACKEND_URL = "https://api.draftright.info"
LOCALHOST_BACKEND_URL = "http://localhost:3000"
# Canonical env override, matching the macOS + Windows apps. Consumers that
# have a SettingsService go through its `backend_url`; this is the single name
# both it and the early crash reporter honour.
BACKEND_ENV_VAR = "DRAFTRIGHT_BACKEND"


def backend_url_override() -> str:
    """Return the env-var backend override, or '' if unset/blank."""
    return os.environ.get(BACKEND_ENV_VAR, "").strip()


def default_backend_url() -> str:
    """Resolve the backend URL for code that runs before SettingsService.

    Honours :data:`BACKEND_ENV_VAR` so dev/test/prod can be swapped without a
    rebuild, falling back to the production default.
    """
    return backend_url_override() or DEFAULT_BACKEND_URL


# ── Application identity ─────────────────────────────────────────────────────
# The GApplication id, and the object path GApplication derives from it. The
# tray helper needs both to reach the app's exported actions over the session
# bus, so they cannot live as literals inside application.py alone.
APP_ID = "com.draftright.app"
APP_OBJECT_PATH = "/" + APP_ID.replace(".", "/")

# ── Tray ─────────────────────────────────────────────────────────────────────
# Top-bar icons are symbolic by convention on GNOME: flat, single-colour, and
# recoloured by the shell to match the theme. The full-colour app icon is for
# the dock/launcher, not here — it cannot adapt and looks heavy beside the
# other indicators.
TRAY_ICON_DEFAULT = f"{APP_ID}-symbolic"
TRAY_ICON_FALLBACK = "edit-paste-symbolic"
# Tray state colours. macOS tints its menu-bar symbol by backend status and
# composites a red "update ready" dot; Windows composites the same dot on its
# NotifyIcon. These are the same values so all three platforms read alike.
TRAY_BADGE_COLOR = "#ef4444"        # red-500, identical to Windows/macOS
TRAY_BADGE_RING_COLOR = "#ffffff"   # ring so the dot reads on any icon
# Windows/macOS use 0.45, but their base icon is a padded app tile. Our
# symbolic fills its viewBox, so the same ratio reads much heavier — 0.36
# keeps the dot at a comparable visual weight.
TRAY_BADGE_SIZE_RATIO = 0.36        # dot diameter as a fraction of the icon
TRAY_BADGE_RING_RATIO = 0.035       # ring width; separates a red dot on a red mark
TRAY_ICON_RENDER_SIZE = 64          # px the composited tray PNG is rendered at
TRAY_HELPER_MODULE = "draftright.tray_helper"
TRAY_SHUTDOWN_TIMEOUT = 2    # seconds to wait for the helper at each escalation step

# ── Keep-alive service (#100) ────────────────────────────────────────────────
KEEPALIVE_RESTART_SEC = 3        # pause before systemd respawns after a crash
KEEPALIVE_START_LIMIT_BURST = 5  # give up after this many failures...
KEEPALIVE_START_LIMIT_INTERVAL = 60   # ...within this window, to avoid a loop

# ── Input / hotkey ───────────────────────────────────────────────────────────
DEFAULT_HOTKEY = "Ctrl+Shift+R"
# How long the X11 listener waits on the X socket before re-checking its stop
# flag. Only a backstop — stop() wakes it immediately through a self-pipe.
X11_EVENT_WAIT_TIMEOUT = 0.5
# How long stop() waits for the listener thread to release its grab. Must
# exceed X11_EVENT_WAIT_TIMEOUT or stop() would routinely return while the old
# grab is still held and the rebind would fail.
X11_STOP_JOIN_TIMEOUT = 2.0

# ── xdg-desktop-portal (Wayland global shortcuts, #99) ───────────────────────
PORTAL_BUS_NAME = "org.freedesktop.portal.Desktop"
PORTAL_OBJECT_PATH = "/org/freedesktop/portal/desktop"
PORTAL_GLOBAL_SHORTCUTS_IFACE = "org.freedesktop.portal.GlobalShortcuts"
# Sanctioned keystroke injection on Wayland — xdotool reaches only XWayland
# clients, and wtype needs a virtual-keyboard protocol Mutter does not provide.
PORTAL_REMOTE_DESKTOP_IFACE = "org.freedesktop.portal.RemoteDesktop"
# Screen capture for the bug report (#85). Wayland forbids a client grabbing
# the screen itself; the compositor runs its own picker and the user consents
# to exactly what is captured.
PORTAL_SCREENSHOT_IFACE = "org.freedesktop.portal.Screenshot"
# Time for the compositor to unmap the dialog before capture starts, so the
# bug-report window is not itself in the screenshot.
SCREENSHOT_HIDE_DELAY_MS = 300
# Settings key holding the portal's restore token, so the permission prompt
# appears once rather than on every launch.
SETTING_INPUT_RESTORE_TOKEN = "input_restore_token"
PORTAL_REQUEST_IFACE = "org.freedesktop.portal.Request"
PORTAL_SESSION_IFACE = "org.freedesktop.portal.Session"
# Stable id for our one shortcut; the compositor keys its saved binding on it.
PORTAL_SHORTCUT_ID = "rewrite-selection"
PORTAL_SHORTCUT_DESCRIPTION = "Rewrite the selected text with DraftRight"
PORTAL_CALL_TIMEOUT_MS = 30000   # portal prompts are user-interactive; allow time

# ── Google sign-in (#97) ─────────────────────────────────────────────────────
GOOGLE_AUTH_ENDPOINT = "https://accounts.google.com/o/oauth2/v2/auth"
GOOGLE_TOKEN_ENDPOINT = "https://oauth2.googleapis.com/token"
GOOGLE_OAUTH_SCOPES = "openid email profile"
GOOGLE_OAUTH_TIMEOUT = 300   # the user has to sign in over in a browser
GOOGLE_PROVIDER = "google"   # wire value for POST /auth/social

# Linux uses a loopback redirect (RFC 8252), which Google only accepts from a
# **Desktop app** client — the macOS client is iOS-type and takes a reversed
# custom scheme instead, so it cannot be reused here.  Override with
# DRAFTRIGHT_GOOGLE_CLIENT_ID until a Linux desktop client is provisioned.
GOOGLE_CLIENT_ID_ENV_VAR = "DRAFTRIGHT_GOOGLE_CLIENT_ID"
# Google's Desktop-type clients REQUIRE client_secret at the token endpoint,
# even with PKCE — omitting it fails with "client_secret is missing". That is
# a Google-specific departure from RFC 8252, which treats a native app as a
# public client. The secret ships with every installed copy and is not
# security-critical (Google's own docs say so); PKCE remains the real
# proof-of-possession. Same env var name Windows uses, and like Windows it
# stays out of git — hygiene and secret-scanning, not confidentiality.
GOOGLE_CLIENT_SECRET_ENV_VAR = "GOOGLE_OAUTH_CLIENT_SECRET"
# Optional local file for dev, gitignored: one line containing the secret.
GOOGLE_CLIENT_SECRET_FILE = "~/.config/draftright/google_client_secret"
DEFAULT_GOOGLE_CLIENT_ID = "22951518033-oaf0ptahsjrsnu2v2qr0kpul5tslpgf6.apps.googleusercontent.com"


def google_client_id() -> str:
    """Resolve the OAuth client id, env var winning over the built-in default."""
    return (
        os.environ.get(GOOGLE_CLIENT_ID_ENV_VAR, "").strip()
        or DEFAULT_GOOGLE_CLIENT_ID
    )


def google_client_secret() -> str:
    """Resolve the OAuth client secret: env var first, then the local file."""
    from pathlib import Path as _Path

    value = os.environ.get(GOOGLE_CLIENT_SECRET_ENV_VAR, "").strip()
    if value:
        return value
    path = _Path(GOOGLE_CLIENT_SECRET_FILE).expanduser()
    try:
        return path.read_text(encoding="utf-8").strip()
    except OSError:
        return ""


def google_sign_in_available() -> bool:
    """False unless BOTH the client id and secret are configured.

    Google's Desktop clients reject the token exchange without the secret, so
    offering the button with only an id would fail after the user had already
    signed in in the browser — the worst point to fail.
    """
    return bool(google_client_id()) and bool(google_client_secret())


# ── HTTP timeouts (seconds) ──────────────────────────────────────────────────
API_TIMEOUT = 30          # /rewrite, /auth, health
HEALTH_TIMEOUT = 5        # /health + /auth/me probe (kept short so the tray stays responsive)
SUBPROCESS_TIMEOUT = 3    # xsel/wl-copy/xdotool one-shot calls
UPDATE_METADATA_TIMEOUT = 10   # version-check fetch
UPDATE_DOWNLOAD_TIMEOUT = 60   # binary download
QR_FETCH_TIMEOUT = 15     # payment QR image fetch
ERROR_REPORT_TIMEOUT = 10      # crash/error upload
AUTO_RECOVERY_TIMEOUT = 60     # start-server.sh run when the backend is down

# PATH prepended when shelling out to the auto-recovery script: a GUI app
# launched from a desktop entry inherits a minimal PATH.
SUBPROCESS_PATH_PREFIX = "/usr/local/bin:/usr/bin:/bin"

# ── Intervals (seconds) ──────────────────────────────────────────────────────
HEALTH_CHECK_INTERVAL = 30
UPDATE_CHECK_INTERVAL = 86400    # once per day
AUTO_RECOVERY_COOLDOWN = 120

# ── External links ───────────────────────────────────────────────────────────
FEEDBACK_BOARD_URL = "https://draftright.info/feedback"

# ── Error reporter ───────────────────────────────────────────────────────────
ERROR_QUEUE_MAX = 100
ERROR_STACK_TAIL_LIMIT = 20000   # chars kept from the tail of a stack trace

# ── Download ─────────────────────────────────────────────────────────────────
DOWNLOAD_BLOCK_SIZE = 8192

# ── UI layout ────────────────────────────────────────────────────────────────
PANEL_TONE_GRID_COLUMNS = 3
PANEL_WIDTH = 420
PANEL_HEIGHT = 520
PANEL_MARGIN = 16
PANEL_PREVIEW_CHARS = 200
FEEDBACK_FLASH_MS = 1500
# Height of the attachment preview in the bug report form.
BUG_REPORT_PREVIEW_HEIGHT = 200

# ── Client-side rewrite cache ────────────────────────────────────────────────
# Mirrors the macOS RewriteCache: avoid re-hitting the backend when the user
# retries the same (text, tone). Bounded; evicts the oldest slice when full.
REWRITE_CACHE_MAX_ENTRIES = 200
REWRITE_CACHE_EVICT_FRACTION = 4   # evict ~1/N of entries when full (N=4 → 25%)

# ── Design tokens (mirror resources/style.css + CLAUDE.md) ───────────────────
COLOR_BACKGROUND = "#0f172a"
COLOR_CARD = "#1e293b"
COLOR_BORDER = "#334155"
COLOR_BRAND_BLUE = "#5d87ff"
COLOR_TEXT = "#e2e8f0"
COLOR_MUTED = "#94a3b8"
COLOR_SUCCESS = "#10b981"
COLOR_BRAND_BLUE_HOVER = "#4a6fe0"   # darker brand blue for :hover states
COLOR_ERROR = "#ef4444"              # error / destructive text
COLOR_WARNING = "#f59e0b"            # amber-500, the macOS systemOrange slot
COLOR_WHITE = "#ffffff"

# ── Diff view (#107) ─────────────────────────────────────────────────────────
# Same red/green reading as macOS DiffView and the Windows port: removed on the
# left, added on the right. Aliases, not new hexes — retinting the palette
# retints the diff.
COLOR_DIFF_DELETED = COLOR_ERROR
COLOR_DIFF_INSERTED = COLOR_SUCCESS
DIFF_HIGHLIGHT_ALPHA = 0.7       # tint strength behind a changed word
# The LCS table is O(old × new) tokens; past this the diff degrades to a whole
# text replacement instead of stalling the GTK main loop on a document-sized
# selection. Counts whitespace tokens too, so ~750 words a side.
DIFF_MAX_TOKENS_PER_SIDE = 1500
PANEL_DIFF_COLUMN_MIN_WIDTH = 240   # each diff column wraps at this width
PANEL_RESULT_MIN_HEIGHT = 120

# ── Grammar check (#107) ─────────────────────────────────────────────────────
# Issue types are colour-coded as macOS GrammarCheckView: spelling red,
# grammar orange, style blue, anything unrecognised grey.
COLOR_GRAMMAR_SPELLING = COLOR_ERROR
COLOR_GRAMMAR_GRAMMAR = COLOR_WARNING
COLOR_GRAMMAR_STYLE = COLOR_BRAND_BLUE
COLOR_GRAMMAR_OTHER = COLOR_MUTED
GRAMMAR_SCORE_MAX = 100
GRAMMAR_SCORE_GOOD = 90      # ≥ this reads green
GRAMMAR_SCORE_FAIR = 70      # ≥ this reads amber, below reads red
GRAMMAR_HIGHLIGHT_ALPHA = 0.15   # tint behind a flagged span in the text view
# A Gtk.TextView inside a scroller has no natural height and collapses to a
# sliver, so the flagged text needs an explicit floor.
GRAMMAR_TEXT_MIN_HEIGHT = 90
PANEL_GRAMMAR_MIN_HEIGHT = 280
# Shown in place of an empty suggestion, i.e. an issue fixed by deleting the
# flagged text — an empty label would read as a broken card.
GRAMMAR_EMPTY_SUGGESTION_LABEL = "(remove)"
