# TeamVault Skill for OpenClaw

Seamlessly inject secrets from TeamVault into your OpenClaw agent.

## Setup

1. Set TeamVault server URL and token in `~/.openclaw/.env`:
```
TEAMVAULT_URL=https://vault.example.com:8443
TEAMVAULT_TOKEN=sa.xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
```

2. Reference secrets in your OpenClaw config using `${VAR}` syntax:
```yaml
providers:
  openrouter:
    apiKey: ${OPENROUTER_API_KEY}
channels:
  telegram:
    token: ${TELEGRAM_BOT_TOKEN}
```

3. Add the TeamVault skill to your agent — it will pre-fetch secrets and set them as env vars before the gateway starts.

## How It Works

The skill runs a pre-start hook that:
1. Connects to your TeamVault server using the service account token
2. Fetches all secrets mapped in `teamvault.json`
3. Writes them to a temporary `.env` file
4. OpenClaw's env loader picks them up via `${VAR}` substitution

No secrets are stored on disk permanently — the temp file is deleted after env loading.

## Configuration

Create `teamvault.json` in your workspace:

```json
{
  "server": "${TEAMVAULT_URL}",
  "project": "my-openclaw-agent",
  "mappings": {
    "OPENROUTER_API_KEY": "providers/openrouter/api-key",
    "TELEGRAM_BOT_TOKEN": "channels/telegram/token",
    "BRAVE_API_KEY": "tools/brave-search/api-key",
    "DISCORD_TOKEN": "channels/discord/token"
  }
}
```

## CLI Usage (within OpenClaw)

```bash
# The agent can use teamvault directly via exec tool
teamvault kv get my-openclaw-agent/providers/openrouter/api-key

# Or inject all mapped secrets and run a command
teamvault run \
  --project my-openclaw-agent \
  --map "OPENROUTER_API_KEY=providers/openrouter/api-key" \
  -- openclaw gateway start
```
