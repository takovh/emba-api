#!/bin/bash
set -e

SERVICE_NAME="emba-api"
SERVICE_FILE="/etc/systemd/system/${SERVICE_NAME}.service"
PROJECT_DIR="$(cd "$(dirname "$0")" && pwd)"
VENV_PYTHON="${PROJECT_DIR}/.venv/bin/python"
VENV_UVICORN="${PROJECT_DIR}/.venv/bin/uvicorn"
PORT=8203

echo "=== EMBA API Service Setup ==="

# Check venv exists
if [ ! -f "$VENV_UVICORN" ]; then
    echo "Error: uvicorn not found at $VENV_UVICORN"
    exit 1
fi

# Create service file
sudo tee "$SERVICE_FILE" > /dev/null <<EOF
[Unit]
Description=EMBA Scanner API
After=network.target

[Service]
Type=simple
User=$(whoami)
WorkingDirectory=${PROJECT_DIR}
ExecStart=${VENV_PYTHON} -m uvicorn main:app --host 0.0.0.0 --port ${PORT}
Restart=always
RestartSec=5
Environment=PYTHONUNBUFFERED=1

[Install]
WantedBy=multi-user.target
EOF

# Reload and enable
sudo systemctl daemon-reload
sudo systemctl enable "$SERVICE_NAME"
sudo systemctl start "$SERVICE_NAME"

echo ""
echo "Service created and started."
echo ""
sudo systemctl status "$SERVICE_NAME" --no-pager
