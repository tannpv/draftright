"""Central configuration + constants for DraftRight Linux.

Single source of truth for values that were previously hardcoded across
service and UI modules (backend URL, timeouts, intervals, magic numbers).
Rule #1: no scattered literals — import from here.

The backend URL resolves with this precedence:
  1. ``DRAFTRIGHT_BACKEND`` environment variable (dev/test/prod swap, no rebuild)
  2. the persisted GSettings value (see :class:`SettingsService`)
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


# ── Input / hotkey ───────────────────────────────────────────────────────────
DEFAULT_HOTKEY = "Ctrl+Shift+R"

# ── HTTP timeouts (seconds) ──────────────────────────────────────────────────
API_TIMEOUT = 30          # /rewrite, /auth, health
SUBPROCESS_TIMEOUT = 3    # xsel/wl-copy/xdotool one-shot calls
UPDATE_METADATA_TIMEOUT = 10   # version-check fetch
UPDATE_DOWNLOAD_TIMEOUT = 60   # binary download
QR_FETCH_TIMEOUT = 15     # payment QR image fetch

# ── Intervals (seconds) ──────────────────────────────────────────────────────
HEALTH_CHECK_INTERVAL = 30
UPDATE_CHECK_INTERVAL = 86400    # once per day
AUTO_RECOVERY_COOLDOWN = 120

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

# ── Design tokens (mirror resources/style.css + CLAUDE.md) ───────────────────
COLOR_BACKGROUND = "#0f172a"
COLOR_CARD = "#1e293b"
COLOR_BORDER = "#334155"
COLOR_BRAND_BLUE = "#5d87ff"
COLOR_TEXT = "#e2e8f0"
COLOR_MUTED = "#94a3b8"
COLOR_SUCCESS = "#10b981"
