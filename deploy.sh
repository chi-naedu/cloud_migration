#!/usr/bin/env bash
set -euo pipefail   # stop at the first error

echo "Building for the ARM64 VM..."
GOOS=linux GOARCH=arm64 go build -o inventory-app

echo "Deploying to the VM..."
vagrant ssh -c "sudo systemctl stop inventory-app && \
  sudo cp /vagrant/inventory-app /usr/local/bin/inventory-app && \
  sudo systemctl start inventory-app && \
  systemctl is-active inventory-app"