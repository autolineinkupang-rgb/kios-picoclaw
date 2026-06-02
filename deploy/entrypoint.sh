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

# Resolve the workspace source (persona + skills). In the Docker image this is
# /app/workspace; for a raw local run it falls back to the repo's workspace dir
# (script lives in deploy/, so ../workspace). Override with KIOS_WORKSPACE_SRC.
if [ -z "$KIOS_WORKSPACE_SRC" ]; then
    if [ -d /app/workspace ]; then
        KIOS_WORKSPACE_SRC="/app/workspace"
    else
        SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
        KIOS_WORKSPACE_SRC="$SCRIPT_DIR/../workspace"
    fi
fi

# Provision the bundled workspace (persona + skills) if not already present.
if [ ! -d "$WORKSPACE" ]; then
    mkdir -p "$WORKSPACE"
    if [ -d "$KIOS_WORKSPACE_SRC" ]; then
        cp -a "$KIOS_WORKSPACE_SRC/." "$WORKSPACE/"
    else
        echo "WARN: workspace source not found at $KIOS_WORKSPACE_SRC; starting with empty workspace" >&2
    fi
fi

# LLM mode: Ollama (local, offline) when OLLAMA_MODEL is set or KIOS_LLM=ollama.
# Otherwise the default Groq-primary cloud setup (unchanged from production).
LLM_MODE="cloud"
if [ -n "$OLLAMA_MODEL" ] || [ "$KIOS_LLM" = "ollama" ]; then
    LLM_MODE="ollama"
    : "${OLLAMA_MODEL:=llama3.2:3b}"
    : "${OLLAMA_BASE_URL:=http://localhost:11434/v1}"
    # Local CPU inference (esp. cold model load on low-RAM hosts) can take far
    # longer than the default 120s HTTP timeout. Give it generous headroom.
    : "${OLLAMA_REQUEST_TIMEOUT:=600}"
fi

# Validate required secrets. GROQ_API_KEY is only required in cloud mode; in
# ollama mode the model runs locally so no Groq key is needed.
REQUIRED_VARS="TELEGRAM_BOT_TOKEN UPSTASH_REDIS_URL KIOS_ALLOW_FROM"
if [ "$LLM_MODE" = "cloud" ]; then
    REQUIRED_VARS="$REQUIRED_VARS GROQ_API_KEY"
fi
missing=""
for v in $REQUIRED_VARS; do
    eval val=\$$v
    [ -z "$val" ] && missing="$missing $v"
done
if [ -n "$missing" ]; then
    echo "FATAL: missing required environment variables:$missing" >&2
    exit 1
fi

# Build allow_from as a JSON array from a comma-separated list of Telegram IDs.
ALLOW_JSON=$(printf '%s' "$KIOS_ALLOW_FROM" | awk -F, '{o="";for(i=1;i<=NF;i++){g=$i;gsub(/^[ \t]+|[ \t]+$/,"",g);if(g!=""){if(o!="")o=o",";o=o "\"" g "\""}}print "["o"]"}')

# Build the model_list entries and decide which model_name is the agent default.
#
# Cloud mode (default): Groq is primary; Gemini/Claude added as fallbacks when
# their keys are present. Behaviour is identical to the original production setup.
#
# Ollama mode: a local Ollama model becomes primary (model_name "ollama-local")
# via an OpenAI-compatible endpoint. picoclaw allows an empty api_key for the
# ollama protocol when api_base is set, so api_keys is left empty. If GROQ_API_KEY
# is also provided in ollama mode, Groq is added as an optional cloud fallback.
PRIMARY_MODEL_NAME=""   # agent default model_name
MODEL_ENTRIES=""        # comma-joined JSON objects for model_list
FALLBACK_MODELS=""      # comma-joined quoted model_names for model_fallbacks

# Append a JSON object to MODEL_ENTRIES (handles the leading comma).
append_model() {
    MODEL_ENTRIES="${MODEL_ENTRIES:+$MODEL_ENTRIES,}$1"
}
# Append a quoted model_name to FALLBACK_MODELS.
append_fallback() {
    FALLBACK_MODELS="${FALLBACK_MODELS:+$FALLBACK_MODELS,}\"$1\""
}

if [ "$LLM_MODE" = "ollama" ]; then
    # Primary: local Ollama (empty api_key is allowed when api_base is set).
    PRIMARY_MODEL_NAME="ollama-local"
    append_model "$(printf '{"model_name":"ollama-local","model":"ollama/%s","api_base":"%s","api_keys":[],"request_timeout":%d}' "$OLLAMA_MODEL" "$OLLAMA_BASE_URL" "$OLLAMA_REQUEST_TIMEOUT")"
    # Optional cloud fallback to Groq if a key is provided.
    if [ -n "$GROQ_API_KEY" ]; then
        append_model "$(printf '{"model_name":"groq-llama","model":"groq/%s","api_keys":["%s"]}' "${GROQ_MODEL:-meta-llama/llama-4-scout-17b-16e-instruct}" "$GROQ_API_KEY")"
        append_fallback "groq-llama"
    fi
else
    # Cloud mode: Groq primary (GROQ_API_KEY is required and already validated).
    PRIMARY_MODEL_NAME="groq-llama"
    append_model "$(printf '{"model_name":"groq-llama","model":"groq/%s","api_keys":["%s"]}' "${GROQ_MODEL:-meta-llama/llama-4-scout-17b-16e-instruct}" "$GROQ_API_KEY")"
fi

# Optional fallback models (Gemini and/or Claude — only when keys are provided).
# These also feed the 3-tier router below.
if [ -n "$GEMINI_API_KEY" ]; then
    append_model "$(printf '{"model_name":"gemini-flash","model":"gemini/%s","api_keys":["%s"]}' "${GEMINI_MODEL:-gemini-2.0-flash}" "$GEMINI_API_KEY")"
    append_fallback "gemini-flash"
fi

if [ -n "$ANTHROPIC_API_KEY" ]; then
    append_model "$(printf '{"model_name":"claude","model":"anthropic/%s","api_keys":["%s"],"api_base":"https://api.anthropic.com/v1"}' "${ANTHROPIC_MODEL:-claude-sonnet-4-6}" "$ANTHROPIC_API_KEY")"
    append_fallback "claude"
fi

FALLBACKS="[${FALLBACK_MODELS}]"

# 3-tier model routing:
#   simple (score < 0.15)        → Gemini  (light)
#   medium (0.15 <= score < 0.50)→ Claude  (medium)
#   complex (score >= 0.50)      → primary (heavy: Groq, or Ollama in local mode)
# Routing is enabled only when Gemini and/or Claude cloud keys are present. In a
# pure Ollama run (no cloud keys) routing stays off so everything hits the local
# model.
ROUTING_ENABLED="false"
ROUTING_LIGHT=""
ROUTING_MEDIUM=""
if [ -n "$GEMINI_API_KEY" ] && [ -n "$ANTHROPIC_API_KEY" ]; then
    ROUTING_ENABLED="true"
    ROUTING_LIGHT="gemini-flash"
    ROUTING_MEDIUM="claude"
elif [ -n "$ANTHROPIC_API_KEY" ]; then
    # Only Claude available: simple → Claude, complex → primary (no light tier)
    ROUTING_ENABLED="true"
    ROUTING_LIGHT="claude"
    ROUTING_MEDIUM=""
elif [ -n "$GEMINI_API_KEY" ]; then
    # Only Gemini available: simple → Gemini, rest → primary (no medium tier)
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
      "model_name": "$PRIMARY_MODEL_NAME",
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
    $MODEL_ENTRIES
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
