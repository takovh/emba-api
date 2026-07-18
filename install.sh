#!/bin/bash
set -e

echo "Installing systemd service..."
sudo cp ./build/emba-api /usr/local/bin/emba-api
sudo cp "emba-api.service" /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable emba-api
sudo systemctl restart emba-api

echo "EMBA API installed successfully."
echo "Check status: systemctl status emba-api"
