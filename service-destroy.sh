#!/bin/bash
set -e

SERVICE_NAME="emba-api"
SERVICE_FILE="/etc/systemd/system/${SERVICE_NAME}.service"

echo "=== EMBA API Service Cleanup ==="

# Stop and disable
if sudo systemctl is-active --quiet "$SERVICE_NAME"; then
    sudo systemctl stop "$SERVICE_NAME"
    echo "Service stopped."
else
    echo "Service not running."
fi

if sudo systemctl is-enabled --quiet "$SERVICE_NAME" 2>/dev/null; then
    sudo systemctl disable "$SERVICE_NAME"
    echo "Service disabled."
fi

# Remove service file
if [ -f "$SERVICE_FILE" ]; then
    sudo rm "$SERVICE_FILE"
    echo "Service file removed."
fi

sudo systemctl daemon-reload
echo ""
echo "Service cleanup completed."
