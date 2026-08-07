import re
from pathlib import Path

from setuptools import setup, find_packages

# Read the version from draftright/__version__.py rather than duplicating it:
# the two had already drifted (setup.py said 1.0.0, the app said 2.4.1), so a
# built package advertised the wrong version.  Parsed, not imported, because
# importing the package here would pull in PyGObject before it is installed.
_VERSION = re.search(
    r'^__version__\s*=\s*["\']([^"\']+)["\']',
    Path(__file__).parent.joinpath("draftright", "__version__.py").read_text(),
    re.MULTILINE,
).group(1)

setup(
    name="draftright",
    version=_VERSION,
    description="DraftRight — AI-powered text rewriting for Linux",
    author="Tan Nguyen",
    packages=find_packages(),
    python_requires=">=3.10",
    install_requires=[
        "PyGObject>=3.44.0",
        "pycairo>=1.24.0",
        "requests>=2.31.0",
        # X11 global hotkey (XGrabKey). Omitting it left every pip install
        # silently without a hotkey on X11 — `_X11Listener` logs one line and
        # returns. Unused on Wayland, which binds through the portal.
        "python-xlib>=0.33",
    ],
    entry_points={
        "console_scripts": [
            "draftright=draftright.main:main",
        ],
    },
    package_data={
        "draftright": ["resources/*.css", "resources/*.xml", "resources/icons/*"],
    },
)
