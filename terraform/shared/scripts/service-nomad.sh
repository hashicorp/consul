#!/usr/bin/env bash
set -e

echo "Starting Nomad..."
if [ -x "$(command -v systemctl)" ]; then
  echo "using systemctl"
  sudo systemctl enable nomad.service
  sudo systemctl start nomad
else 
  echo "using upstart"
  sudo start nomad
fi
