"""In-memory client-side rewrite cache.

Mirrors the macOS ``RewriteCache`` 1:1: key = (tone, text), bounded size,
evict the oldest slice when full. Avoids a redundant backend round-trip
when the user retries the same text+tone combo (e.g. reopening the panel
or toggling tones back and forth).

Pure logic — no GTK / network — so it is unit-testable without a display.
Thread-safe: the panel populates it from a worker thread and reads it on
the GTK main thread.
"""

from __future__ import annotations

import threading
from collections import OrderedDict

from draftright import config


class RewriteCache:
    """Bounded, insertion-ordered (text, tone) → result cache."""

    def __init__(
        self,
        max_entries: int = config.REWRITE_CACHE_MAX_ENTRIES,
        evict_fraction: int = config.REWRITE_CACHE_EVICT_FRACTION,
    ) -> None:
        self._max = max(1, max_entries)
        self._evict_fraction = max(1, evict_fraction)
        self._data: "OrderedDict[str, str]" = OrderedDict()
        self._lock = threading.Lock()

    @staticmethod
    def _key(text: str, tone: str, language: str | None = None) -> str:
        # Same shape as macOS ("{tone}::{text}") so behaviour matches, with the
        # target language appended when it affects the result — otherwise
        # switching language would serve the old language from cache.
        base = f"{tone}::{text}"
        return f"{language}::{base}" if language else base

    def get(self, text: str, tone: str, language: str | None = None) -> str | None:
        with self._lock:
            return self._data.get(self._key(text, tone, language))

    def set(self, text: str, tone: str, result: str,
            language: str | None = None) -> None:
        key = self._key(text, tone, language)
        with self._lock:
            if key not in self._data and len(self._data) >= self._max:
                # Evict the oldest ~1/evict_fraction entries in one pass so we
                # don't trim on every single insert once full.
                remove = max(1, self._max // self._evict_fraction)
                for _ in range(remove):
                    if not self._data:
                        break
                    self._data.popitem(last=False)  # FIFO: drop oldest
            self._data[key] = result

    def clear(self) -> None:
        with self._lock:
            self._data.clear()

    def __len__(self) -> int:
        with self._lock:
            return len(self._data)
