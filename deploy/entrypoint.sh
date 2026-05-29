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
# Each provider registers multiple named entries so the owner can pick a
# specific model via /model or the dashboard without redeploying.
GEMINI_ENTRY=""
ANTHROPIC_ENTRY=""
FALLBACK_MODELS=""
# Extra Groq variants (always registered alongside the primary groq-llama).
GROQ_EXTRA=""

# Groq: register 3 model variants with stable model_name values.
GROQ_EXTRA=$(printf ',{"model_name":"groq-8b","model":"groq/llama3-8b-8192","api_keys":["%s"]}' "$GROQ_API_KEY")
GROQ_EXTRA="${GROQ_EXTRA}$(printf ',{"model_name":"groq-mixtral","model":"groq/mixtral-8x7b-32768","api_keys":["%s"]}' "$GROQ_API_KEY")"

if [ -n "$GEMINI_API_KEY" ]; then
    # Register 3 Gemini variants; gemini-flash is used by the routing system.
    GEMINI_ENTRY=$(printf ',{"model_name":"gemini-lite","model":"gemini/gemini-2.0-flash-lite","api_keys":["%s"]}' "$GEMINI_API_KEY")
    GEMINI_ENTRY="${GEMINI_ENTRY}$(printf ',{"model_name":"gemini-flash","model":"gemini/gemini-2.0-flash","api_keys":["%s"]}' "$GEMINI_API_KEY")"
    GEMINI_ENTRY="${GEMINI_ENTRY}$(printf ',{"model_name":"gemini-pro","model":"gemini/gemini-1.5-pro","api_keys":["%s"]}' "$GEMINI_API_KEY")"
    FALLBACK_MODELS="${FALLBACK_MODELS:+$FALLBACK_MODELS,}\"gemini-flash\""
fi

if [ -n "$ANTHROPIC_API_KEY" ]; then
    # Register 3 Anthropic variants; claude-sonnet is used by the routing system.
    ANTHROPIC_ENTRY=$(printf ',{"model_name":"claude-haiku","model":"anthropic/claude-haiku-4-5-20251001","api_keys":["%s"],"api_base":"https://api.anthropic.com/v1"}' "$ANTHROPIC_API_KEY")
    ANTHROPIC_ENTRY="${ANTHROPIC_ENTRY}$(printf ',{"model_name":"claude-sonnet","model":"anthropic/claude-sonnet-4-6","api_keys":["%s"],"api_base":"https://api.anthropic.com/v1"}' "$ANTHROPIC_API_KEY")"
    ANTHROPIC_ENTRY="${ANTHROPIC_ENTRY}$(printf ',{"model_name":"claude-opus","model":"anthropic/claude-opus-4-6","api_keys":["%s"],"api_base":"https://api.anthropic.com/v1"}' "$ANTHROPIC_API_KEY")"
    # Alias "claude" → claude-sonnet for backward compat (routing uses "claude").
    ANTHROPIC_ENTRY="${ANTHROPIC_ENTRY}$(printf ',{"model_name":"claude","model":"anthropic/claude-sonnet-4-6","api_keys":["%s"],"api_base":"https://api.anthropic.com/v1"}' "$ANTHROPIC_API_KEY")"
    FALLBACK_MODELS="${FALLBACK_MODELS:+$FALLBACK_MODELS,}\"claude-sonnet\""
fi

FALLBACKS="[${FALLBACK_MODELS}]"

# 3-tier model routing:
#   simple (score < 0.15)        → Gemini  (light)
#   medium (0.15 <= score < 0.50)→ Claude  (medium)
#   complex (score >= 0.50)      → Groq    (primary/heavy)
# Routing is enabled only when both Gemini and Claude keys are present.
ROUTING_ENABLED="false"
ROUTING_LIGHT=""
ROUTING_MEDIUM=""
if [ -n "$GEMINI_API_KEY" ] && [ -n "$ANTHROPIC_API_KEY" ]; then
    ROUTING_ENABLED="true"
    ROUTING_LIGHT="gemini-flash"
    ROUTING_MEDIUM="claude-sonnet"
elif [ -n "$ANTHROPIC_API_KEY" ]; then
    # Only Claude available: simple → Claude Sonnet, complex → Groq (no light tier)
    ROUTING_ENABLED="true"
    ROUTING_LIGHT="claude-sonnet"
    ROUTING_MEDIUM=""
elif [ -n "$GEMINI_API_KEY" ]; then
    # Only Gemini available: simple → Gemini, rest → Groq (no medium tier)
    ROUTING_ENABLED="true"
    ROUTING_LIGHT="gemini-flash"
    ROUTING_MEDIUM=""
fi

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
      "temperature": 0.5,
      "routing": {
        "enabled": $ROUTING_ENABLED,
        "light_model": "$ROUTING_LIGHT",
        "medium_model": "$ROUTING_MEDIUM",
        "threshold": 0.15,
        "heavy_threshold": 0.50
      }
    }
  },
  "model_list": [
    {"model_name":"groq-llama","model":"groq/${GROQ_MODEL:-llama-3.3-70b-versatile}","api_keys":["$GROQ_API_KEY"]}$GROQ_EXTRA$GEMINI_ENTRY$ANTHROPIC_ENTRY
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
