#!/bin/bash
set -e

echo "Stopping emba-api service..."
sudo systemctl stop emba-api 2>/dev/null || true
sudo systemctl disable emba-api 2>/dev/null || true

echo "Removing service file..."
sudo rm -f /etc/systemd/system/emba-api.service
sudo systemctl daemon-reload

echo "Removing binary..."
sudo rm -f /usr/local/bin/emba-api

echo "EMBA API uninstalled."
