#!/usr/bin/env bash
# Keep the Claude Code plugin's two version declarations in lockstep.
#
# Usage:
#   scripts/plugin-version.sh check   Exit non-zero if marketplace.json and plugin.json disagree.
#   scripts/plugin-version.sh sync    Copy plugin.json version into marketplace.json.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
MARKETPLACE="$ROOT/.claude-plugin/marketplace.json"
PLUGIN="$ROOT/claude-plugin/.claude-plugin/plugin.json"

plugin_version() {
  jq -r '.version' "$PLUGIN"
}

marketplace_version() {
  jq -r '.plugins[] | select(.name == "asana-cli") | .version' "$MARKETPLACE"
}

cmd_check() {
  local pv mv
  pv="$(plugin_version)"
  mv="$(marketplace_version)"
  if [[ "$pv" != "$mv" ]]; then
    echo >&2 "Plugin version mismatch:"
    echo >&2 "  plugin.json      = $pv  ($PLUGIN)"
    echo >&2 "  marketplace.json = $mv  ($MARKETPLACE)"
    echo >&2 "Run: scripts/plugin-version.sh sync"
    exit 1
  fi
  echo "Plugin versions match ($pv)."
}

cmd_sync() {
  local pv
  pv="$(plugin_version)"
  local tmp
  tmp="$(mktemp)"
  jq --arg v "$pv" '(.plugins[] | select(.name == "asana-cli") | .version) = $v' "$MARKETPLACE" > "$tmp"
  mv "$tmp" "$MARKETPLACE"
  echo "Set marketplace.json plugin version to $pv."
}

case "${1:-}" in
  check) cmd_check ;;
  sync)  cmd_sync ;;
  *)
    echo "Usage: $0 {check|sync}" >&2
    exit 2
    ;;
esac
