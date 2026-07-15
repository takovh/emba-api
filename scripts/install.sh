#!/bin/bash
set -e

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"

echo "Building emba-api..."
cd "$PROJECT_DIR"
go build -ldflags="-s -w" -o /usr/local/bin/emba-api .

echo "Installing systemd service..."
cp "$PROJECT_DIR/emba-api.service" /etc/systemd/system/
systemctl daemon-reload
systemctl enable emba-api
systemctl restart emba-api

echo "EMBA API installed successfully."
echo "Check status: systemctl status emba-api"
