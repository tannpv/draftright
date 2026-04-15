"""Auto-update service for DraftRight Linux."""

import json
import logging
import os
import shutil
import stat
import sys
import tempfile
import time
from urllib.request import urlopen, urlretrieve, Request
from urllib.error import URLError

logger = logging.getLogger(__name__)

CHECK_INTERVAL = 86400  # 24 hours


class UpdateInfo:
    def __init__(self, data: dict):
        self.version = data.get("version", "")
        self.linux_url = data.get("linux_url", "")
        self.release_notes = data.get("release_notes", "")
        self.required = data.get("required", False)


class UpdateService:
    def __init__(self, current_version: str, backend_url: str):
        self._current_version = current_version
        self._backend_url = backend_url.rstrip("/")
        self._last_check = 0.0

    def check_if_needed(self):
        now = time.monotonic()
        if now - self._last_check < CHECK_INTERVAL:
            return None
        self._last_check = now

        info = self._fetch_latest()
        if info is None:
            return None
        if not self._is_newer(info.version, self._current_version):
            return None
        if not info.linux_url:
            return None

        return info

    def download_and_install(self, info, progress_callback=None):
        """Download the update and replace the current binary.

        Args:
            info: UpdateInfo with version and linux_url
            progress_callback: Optional callable(fraction: float, status: str)
        """
        try:
            temp_dir = tempfile.mkdtemp(prefix="draftright-update-")
            temp_path = os.path.join(temp_dir, f"DraftRight-{info.version}.AppImage")

            logger.info("Downloading update from %s", info.linux_url)
            if progress_callback:
                progress_callback(0.0, f"Downloading DraftRight v{info.version}...")

            # Download with progress
            req = Request(info.linux_url)
            with urlopen(req, timeout=60) as resp:
                total = int(resp.headers.get("Content-Length", 0))
                downloaded = 0
                block_size = 8192
                with open(temp_path, "wb") as f:
                    while True:
                        chunk = resp.read(block_size)
                        if not chunk:
                            break
                        f.write(chunk)
                        downloaded += len(chunk)
                        if total > 0 and progress_callback:
                            progress_callback(downloaded / total, f"Downloading... {downloaded * 100 // total}%")

            if progress_callback:
                progress_callback(1.0, "Installing...")

            current_binary = os.path.realpath(sys.argv[0])
            shutil.move(temp_path, current_binary)
            os.chmod(current_binary, os.stat(current_binary).st_mode | stat.S_IEXEC)
            shutil.rmtree(temp_dir, ignore_errors=True)

            logger.info("Update installed successfully to %s", current_binary)
            return True
        except Exception as exc:
            logger.error("Update failed: %s", exc)
            return False

    def check_now(self):
        """Manual check — skips the throttle. Returns (has_update, info_or_message)."""
        self._last_check = time.monotonic()
        info = self._fetch_latest()
        if info is None:
            return False, f"You're running the latest version (v{self._current_version})."
        if not self._is_newer(info.version, self._current_version):
            return False, f"You're running the latest version (v{self._current_version})."
        if not info.linux_url:
            return False, "No Linux update available."
        return True, info

    def relaunch(self):
        current_binary = os.path.realpath(sys.argv[0])
        logger.info("Relaunching %s", current_binary)
        os.execv(current_binary, sys.argv)

    def _fetch_latest(self):
        try:
            url = f"{self._backend_url}/updates/latest"
            req = Request(url, headers={"Accept": "application/json"})
            with urlopen(req, timeout=10) as resp:
                data = json.loads(resp.read().decode())
                return UpdateInfo(data)
        except (URLError, json.JSONDecodeError, OSError) as exc:
            logger.debug("Update check failed: %s", exc)
            return None

    @staticmethod
    def _is_newer(remote, local):
        def parse(v):
            return [int(x) for x in v.split(".") if x.isdigit()]

        r = parse(remote)
        l_val = parse(local)
        length = max(len(r), len(l_val))
        for i in range(length):
            rv = r[i] if i < len(r) else 0
            lv = l_val[i] if i < len(l_val) else 0
            if rv > lv:
                return True
            if rv < lv:
                return False
        return False
