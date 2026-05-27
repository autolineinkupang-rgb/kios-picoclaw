#!/bin/sh
# kios-picoclaw container entrypoint.
# Renders config.json from environment variables (picoclaw does NOT expand
# $VARS inside config.json), provisions the workspace, then starts the gateway.
set -e

: "${PICOCLAW_HOME:=/app/.picoclaw}"
export PICOCLAW_HOME
WORKSPACE="$PICOCLAW_HOME/workspace"
CONFIG="$PICOCLAW_HOME/config.json"
PORT="${PORT:-18790}"

mkdir -p "$PICOCLAW_HOME"

# Provision the bundled workspace (persona + skills) if not already present.
if [ ! -d "$WORKSPACE" ]; then
    mkdir -p "$WORKSPACE"
    cp -a /app/workspace/. "$WORKSPACE/"
fi

# Validate required secrets.
missing=""
for v in TELEGRAM_BOT_TOKEN GROQ_API_KEY UPSTASH_REDIS_URL KIOS_ALLOW_FROM; do
    eval val=\$$v
    [ -z "$val" ] && missing="$missing $v"
done
if [ -n "$missing" ]; then
    echo "FATAL: missing required environment variables:$missing" >&2
    exit 1
fi

# Build allow_from as a JSON array from a comma-separated list of Telegram IDs.
ALLOW_JSON=$(printf '%s' "$KIOS_ALLOW_FROM" | awk -F, '{o="";for(i=1;i<=NF;i++){g=$i;gsub(/^[ \t]+|[ \t]+$/,"",g);if(g!=""){if(o!="")o=o",";o=o "\"" g "\""}}print "["o"]"}')

# Optional fallback models (Gemini and/or Claude — only when keys are provided).
GEMINI_ENTRY=""
ANTHROPIC_ENTRY=""
FALLBACK_MODELS=""

if [ -n "$GEMINI_API_KEY" ]; then
    GEMINI_ENTRY=$(printf ',{"model_name":"gemini-flash","model":"gemini/%s","api_keys":["%s"]}' "${GEMINI_MODEL:-gemini-2.0-flash}" "$GEMINI_API_KEY")
    FALLBACK_MODELS="${FALLBACK_MODELS:+$FALLBACK_MODELS,}\"gemini-flash\""
fi

if [ -n "$ANTHROPIC_API_KEY" ]; then
    ANTHROPIC_ENTRY=$(printf ',{"model_name":"claude","model":"anthropic/%s","api_keys":["%s"],"api_base":"https://api.anthropic.com/v1"}' "${ANTHROPIC_MODEL:-claude-sonnet-4-6}" "$ANTHROPIC_API_KEY")
    FALLBACK_MODELS="${FALLBACK_MODELS:+$FALLBACK_MODELS,}\"claude\""
fi

FALLBACKS="[${FALLBACK_MODELS}]"

cat > "$CONFIG" <<EOF
{
  "version": 3,
  "gateway": { "host": "0.0.0.0", "port": $PORT },
  "agents": {
    "defaults": {
      "workspace": "$WORKSPACE",
      "model_name": "groq-llama",
      "model_fallbacks": $FALLBACKS,
      "max_tokens": 4096,
      "max_tool_iterations": 12,
      "temperature": 0.5
    }
  },
  "model_list": [
    {"model_name":"groq-llama","model":"groq/${GROQ_MODEL:-meta-llama/llama-4-scout-17b-16e-instruct}","api_keys":["$GROQ_API_KEY"]}$GEMINI_ENTRY$ANTHROPIC_ENTRY
  ],
  "channel_list": {
    "telegram": {
      "enabled": true,
      "type": "telegram",
      "allow_from": $ALLOW_JSON,
      "settings": { "token": "$TELEGRAM_BOT_TOKEN" }
    }
  }
}
EOF

# Clear any stale PID from a previous (crashed) container.
rm -f "$PICOCLAW_HOME/.picoclaw.pid"

echo "kios-picoclaw: starting gateway on 0.0.0.0:$PORT (workspace: $WORKSPACE)"
exec picoclaw gateway
