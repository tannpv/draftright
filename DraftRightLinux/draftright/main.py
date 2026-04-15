"""DraftRight Linux — entry point."""

import sys

import gi
gi.require_version("Gtk", "4.0")
gi.require_version("Adw", "1")

from draftright.application import DraftRightApplication


def main():
    app = DraftRightApplication()
    app.run(sys.argv)


if __name__ == "__main__":
    main()
