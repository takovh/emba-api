#!/bin/bash
set -e

echo "Stopping emba-api service..."
systemctl stop emba-api 2>/dev/null || true
systemctl disable emba-api 2>/dev/null || true

echo "Removing service file..."
rm -f /etc/systemd/system/emba-api.service
systemctl daemon-reload

echo "Removing binary..."
rm -f /usr/local/bin/emba-api

echo "EMBA API uninstalled."
