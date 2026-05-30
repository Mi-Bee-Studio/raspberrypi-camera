#!/bin/bash
set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"
REMOTE="user@rpi-host"
REMOTE_DIR="~/rpi-cam"

echo "Building for arm64..."
cd "$PROJECT_DIR"
make build

echo "Deploying to $REMOTE..."
ssh "$REMOTE" "mkdir -p $REMOTE_DIR"
scp build/rpi-cam "$REMOTE:$REMOTE_DIR/"
scp configs/config.yaml "$REMOTE:$REMOTE_DIR/"

# Install service
scp deploy/rpi-cam.service "$REMOTE:/tmp/"
ssh "$REMOTE" "sudo mv /tmp/rpi-cam.service /etc/systemd/system/ && sudo systemctl daemon-reload && sudo systemctl enable rpi-cam"

echo "Deploy complete. Run 'make service-restart' to start."
