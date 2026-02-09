#!/bin/bash
# TeamVault Pre-Start Hook for OpenClaw
# Place this in your OpenClaw hooks directory or run before gateway start.
#
# Usage: source teamvault-env-hook.sh
#
# Requires:
#   TEAMVAULT_URL  — TeamVault server URL
#   TEAMVAULT_TOKEN — Service account token
#   teamvault CLI binary in PATH

set -euo pipefail

TEAMVAULT_CONFIG="${OPENCLAW_STATE_DIR:-$HOME/.openclaw}/workspace/teamvault.json"

if [ ! -f "$TEAMVAULT_CONFIG" ]; then
  echo "[teamvault] No teamvault.json found, skipping secret injection."
  exit 0
fi

if [ -z "${TEAMVAULT_URL:-}" ] || [ -z "${TEAMVAULT_TOKEN:-}" ]; then
  echo "[teamvault] TEAMVAULT_URL or TEAMVAULT_TOKEN not set, skipping."
  exit 0
fi

echo "[teamvault] Fetching secrets from TeamVault..."

PROJECT=$(python3 -c "import json; print(json.load(open('$TEAMVAULT_CONFIG'))['project'])" 2>/dev/null || echo "")
if [ -z "$PROJECT" ]; then
  echo "[teamvault] No project specified in teamvault.json"
  exit 1
fi

# Parse mappings and fetch each secret
python3 -c "
import json, subprocess, os, sys

config = json.load(open('$TEAMVAULT_CONFIG'))
server = os.environ.get('TEAMVAULT_URL', config.get('server', ''))
token = os.environ.get('TEAMVAULT_TOKEN', '')
project = config['project']
mappings = config.get('mappings', {})

env_lines = []
for env_var, secret_path in mappings.items():
    try:
        result = subprocess.run(
            ['teamvault', 'kv', 'get', f'{project}/{secret_path}'],
            capture_output=True, text=True,
            env={**os.environ, 'TEAMVAULT_SERVER': server, 'TEAMVAULT_TOKEN': token}
        )
        if result.returncode == 0:
            env_lines.append(f'{env_var}={result.stdout}')
            print(f'[teamvault] Loaded: {env_var}', file=sys.stderr)
        else:
            print(f'[teamvault] Failed to fetch {env_var}: {result.stderr.strip()}', file=sys.stderr)
    except Exception as e:
        print(f'[teamvault] Error fetching {env_var}: {e}', file=sys.stderr)

# Write to temporary env file
env_file = os.path.join(os.environ.get('OPENCLAW_STATE_DIR', os.path.expanduser('~/.openclaw')), '.env.teamvault')
with open(env_file, 'w') as f:
    f.write('\n'.join(env_lines) + '\n')
os.chmod(env_file, 0o600)
print(f'[teamvault] {len(env_lines)} secrets loaded to {env_file}', file=sys.stderr)
"

# Source the temporary env file
ENV_FILE="${OPENCLAW_STATE_DIR:-$HOME/.openclaw}/.env.teamvault"
if [ -f "$ENV_FILE" ]; then
  set -a
  source "$ENV_FILE"
  set +a
  # Clean up immediately
  rm -f "$ENV_FILE"
  echo "[teamvault] Secrets injected into environment."
fi
