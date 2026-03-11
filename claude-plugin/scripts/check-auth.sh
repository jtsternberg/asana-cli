#!/bin/bash
# Verify asana CLI is authenticated and ready
if ! command -v asana &> /dev/null; then
  echo "ERROR: asana CLI not found. Build from ~/Code/asana-cli: go build -o /usr/local/bin/asana ./cmd/asana"
  exit 1
fi
asana auth status 2>&1
