#!/usr/bin/env bash
# Install/uninstall the DraftRight backend systemd user service
# Usage: ./install-service.sh [install|uninstall|status]

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
SERVICE_NAME="draftright-backend"
UNIT_DIR="$HOME/.config/systemd/user"
UNIT_FILE="$UNIT_DIR/$SERVICE_NAME.service"
TIMER_FILE="$UNIT_DIR/$SERVICE_NAME.timer"
START_SCRIPT="$SCRIPT_DIR/start-server.sh"

case "${1:-install}" in
    install)
        mkdir -p "$UNIT_DIR"

        # Service unit — one-shot script
        cat > "$UNIT_FILE" <<EOF
[Unit]
Description=DraftRight backend Docker services check

[Service]
Type=oneshot
ExecStart=$START_SCRIPT
Environment=PATH=/usr/local/bin:/usr/bin:/bin
EOF

        # Timer unit — runs at login and every 2 minutes
        cat > "$TIMER_FILE" <<EOF
[Unit]
Description=DraftRight backend health timer

[Timer]
OnBootSec=10s
OnUnitActiveSec=2min
Persistent=true

[Install]
WantedBy=timers.target
EOF

        systemctl --user daemon-reload
        systemctl --user enable --now "$SERVICE_NAME.timer"
        echo "Installed and started $SERVICE_NAME.timer"
        echo "  Runs at boot + every 2 minutes"
        echo "  Logs: journalctl --user -u $SERVICE_NAME"
        ;;
    uninstall)
        systemctl --user disable --now "$SERVICE_NAME.timer" 2>/dev/null || true
        rm -f "$UNIT_FILE" "$TIMER_FILE"
        systemctl --user daemon-reload
        echo "Uninstalled $SERVICE_NAME"
        ;;
    status)
        if systemctl --user is-active "$SERVICE_NAME.timer" &>/dev/null; then
            echo "$SERVICE_NAME.timer is active"
            systemctl --user status "$SERVICE_NAME.timer" --no-pager 2>/dev/null || true
        else
            echo "$SERVICE_NAME.timer is not active"
        fi
        ;;
    *)
        echo "Usage: $0 [install|uninstall|status]"
        exit 1
        ;;
esac
