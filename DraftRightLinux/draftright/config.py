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
# The app icon, installed into the hicolor theme as com.draftright.app.
# Falls back to a stock glyph only if the icon set was never installed.
TRAY_ICON_DEFAULT = APP_ID
TRAY_ICON_FALLBACK = "edit-paste-symbolic"
TRAY_ICON_ATTENTION = "dialog-warning-symbolic"   # shown while the backend is unreachable
TRAY_HELPER_MODULE = "draftright.tray_helper"
TRAY_SHUTDOWN_TIMEOUT = 2    # seconds to wait for the helper at each escalation step

# ── Input / hotkey ───────────────────────────────────────────────────────────
DEFAULT_HOTKEY = "Ctrl+Shift+R"

# ── xdg-desktop-portal (Wayland global shortcuts, #99) ───────────────────────
PORTAL_BUS_NAME = "org.freedesktop.portal.Desktop"
PORTAL_OBJECT_PATH = "/org/freedesktop/portal/desktop"
PORTAL_GLOBAL_SHORTCUTS_IFACE = "org.freedesktop.portal.GlobalShortcuts"
# Sanctioned keystroke injection on Wayland — xdotool reaches only XWayland
# clients, and wtype needs a virtual-keyboard protocol Mutter does not provide.
PORTAL_REMOTE_DESKTOP_IFACE = "org.freedesktop.portal.RemoteDesktop"
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
DEFAULT_GOOGLE_CLIENT_ID = ""


def google_client_id() -> str:
    """Resolve the OAuth client id, env var winning over the built-in default."""
    return (
        os.environ.get(GOOGLE_CLIENT_ID_ENV_VAR, "").strip()
        or DEFAULT_GOOGLE_CLIENT_ID
    )


def google_sign_in_available() -> bool:
    """False when no client id is configured, so the UI can hide the button."""
    return bool(google_client_id())


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
COLOR_WHITE = "#ffffff"
