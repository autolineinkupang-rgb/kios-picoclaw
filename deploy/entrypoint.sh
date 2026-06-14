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
# In cloud mode a primary model is required. Groq is the default preset, so
# GROQ_API_KEY is required UNLESS you supply a custom primary via KIOS_MODEL
# (any provider — see the model section below).
if [ "$LLM_MODE" = "cloud" ] && [ -z "$KIOS_MODEL" ]; then
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
# A custom primary (KIOS_MODEL) needs either an API key or a (keyless local) base.
if [ "$LLM_MODE" = "cloud" ] && [ -n "$KIOS_MODEL" ] && [ -z "$KIOS_MODEL_KEY" ] && [ -z "$KIOS_MODEL_BASE" ]; then
    echo "FATAL: KIOS_MODEL is set but neither KIOS_MODEL_KEY nor KIOS_MODEL_BASE is provided" >&2
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
# Build a generic model_list JSON object for ANY provider.
# Args: name, model_id ("protocol/model"), api_key (may be empty), api_base (may be empty).
# api_keys is omitted when the key is empty (e.g. keyless local endpoint with a base);
# api_base is omitted when empty.
build_model_json() {
    _name="$1"; _model="$2"; _key="$3"; _base="$4"
    _json="{\"model_name\":\"$_name\",\"model\":\"$_model\""
    [ -n "$_key" ]  && _json="$_json,\"api_keys\":[\"$_key\"]"
    [ -n "$_base" ] && _json="$_json,\"api_base\":\"$_base\""
    printf '%s}' "$_json"
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
elif [ -n "$KIOS_MODEL" ]; then
    # Cloud mode, GENERIC primary — works with ANY provider, no file edit needed.
    # Switch the whole bot to another model purely via Railway env:
    #   KIOS_MODEL       (required) full "protocol/model-id", e.g.
    #                    "openai/gpt-4o-mini", "deepseek/deepseek-chat",
    #                    "moonshot/kimi-k2", "mistral/mistral-large-latest",
    #                    "openrouter/anthropic/claude-3.5-sonnet"
    #   KIOS_MODEL_KEY   (required unless KIOS_MODEL_BASE is a keyless local endpoint)
    #   KIOS_MODEL_BASE  (optional) custom api_base / OpenAI-compatible endpoint
    #   KIOS_MODEL_NAME  (optional) alias, default "primary"
    PRIMARY_MODEL_NAME="${KIOS_MODEL_NAME:-primary}"
    append_model "$(build_model_json "$PRIMARY_MODEL_NAME" "$KIOS_MODEL" "$KIOS_MODEL_KEY" "$KIOS_MODEL_BASE")"
else
    # Cloud mode, DEFAULT preset: Groq primary (GROQ_API_KEY required & validated).
    #
    # ── GANTI MODEL GROQ: set env GROQ_MODEL di Railway (mis. "llama-3.3-70b-versatile").
    #    Daftar model: https://console.groq.com/docs/models
    #    Atau pakai provider lain sepenuhnya lewat KIOS_MODEL (lihat cabang di atas).
    PRIMARY_MODEL_NAME="groq-llama"
    append_model "$(printf '{"model_name":"groq-llama","model":"groq/%s","api_keys":["%s"]}' "${GROQ_MODEL:-meta-llama/llama-4-scout-17b-16e-instruct}" "$GROQ_API_KEY")"
fi

# Optional FALLBACK models (Gemini and/or Claude — hanya dibuat kalau key-nya ada).
# Fallback = dipakai SAAT primary (Groq) gagal, BUKAN untuk trafik normal.
# Routing (bagi trafik ke model ini per kompleksitas) hanya aktif jika
# KIOS_ROUTING=on — lihat blok routing di bawah.
#
# ── GANTI MODEL GEMINI: set env GEMINI_MODEL (mis. "gemini-2.0-flash", "gemini-1.5-flash").
#    Catatan: free tier punya kuota harian kecil → gampang kena 429.
if [ -n "$GEMINI_API_KEY" ]; then
    append_model "$(printf '{"model_name":"gemini-flash","model":"gemini/%s","api_keys":["%s"]}' "${GEMINI_MODEL:-gemini-2.0-flash}" "$GEMINI_API_KEY")"
    append_fallback "gemini-flash"
fi

# ── GANTI MODEL CLAUDE: set env ANTHROPIC_MODEL (mis. "claude-sonnet-4-6", "claude-haiku-4-5-20251001").
if [ -n "$ANTHROPIC_API_KEY" ]; then
    append_model "$(printf '{"model_name":"claude","model":"anthropic/%s","api_keys":["%s"],"api_base":"https://api.anthropic.com/v1"}' "${ANTHROPIC_MODEL:-claude-sonnet-4-6}" "$ANTHROPIC_API_KEY")"
    append_fallback "claude"
fi

# Generic EXTRA models — add ANY number of models from ANY provider via env,
# no file edit needed. KIOS_EXTRA_MODELS is a JSON array of model_list entries:
#   KIOS_EXTRA_MODELS='[{"model_name":"kimi","model":"moonshot/kimi-k2","api_keys":["sk-..."]},
#                       {"model_name":"gpt","model":"openai/gpt-4o-mini","api_keys":["sk-..."]}]'
# To use them as failover, list their model_name(s) in KIOS_EXTRA_FALLBACKS (comma-separated),
# e.g. KIOS_EXTRA_FALLBACKS="kimi,gpt". They can also be targeted by routing
# via KIOS_ROUTING_LIGHT / KIOS_ROUTING_MEDIUM (see routing block below).
if [ -n "$KIOS_EXTRA_MODELS" ]; then
    # Strip the outer [ ] and surrounding whitespace, then splice the inner objects in.
    EXTRA_INNER=$(printf '%s' "$KIOS_EXTRA_MODELS" | sed 's/^[[:space:]]*\[//; s/\][[:space:]]*$//; s/^[[:space:]]*//; s/[[:space:]]*$//')
    [ -n "$EXTRA_INNER" ] && append_model "$EXTRA_INNER"
fi
if [ -n "$KIOS_EXTRA_FALLBACKS" ]; then
    OLDIFS="$IFS"; IFS=','
    for fb in $KIOS_EXTRA_FALLBACKS; do
        fb=$(printf '%s' "$fb" | sed 's/^[[:space:]]*//; s/[[:space:]]*$//')
        [ -n "$fb" ] && append_fallback "$fb"
    done
    IFS="$OLDIFS"
fi

FALLBACKS="[${FALLBACK_MODELS}]"

# 3-tier model routing — OPT-IN, DEFAULT MATI.
#
# DEFAULT (KIOS_ROUTING tidak diisi / "off"):
#   Routing dimatikan. SEMUA pesan ditangani primary (Groq di cloud mode).
#   Gemini/Claude tetap ada HANYA sebagai fallback saat Groq gagal.
#   → Ini mencegah 429 Gemini free-tier karena trafik normal tidak lagi
#     dilempar ke Gemini setiap pesan sederhana.
#
# AKTIFKAN ROUTING (kalau nanti dibutuhkan): set env KIOS_ROUTING=on di Railway.
#   Saat aktif, pesan dibagi per kompleksitas:
#     simple  (score < 0.15)        → light_model
#     medium  (0.15 ≤ score < 0.50) → medium_model
#     complex (score ≥ 0.50)        → primary
#   Default light/medium memakai preset (Gemini=light, Claude=medium) bila key-nya
#   ada, TAPI bisa diarahkan ke model APA SAJA (termasuk KIOS_EXTRA_MODELS) lewat:
#     KIOS_ROUTING_LIGHT="<model_name>"   KIOS_ROUTING_MEDIUM="<model_name>"
ROUTING_ENABLED="false"
ROUTING_LIGHT=""
ROUTING_MEDIUM=""
if [ "$KIOS_ROUTING" = "on" ] || [ "$KIOS_ROUTING" = "true" ]; then
    # Sensible defaults from whichever preset keys are present.
    if [ -n "$GEMINI_API_KEY" ] && [ -n "$ANTHROPIC_API_KEY" ]; then
        ROUTING_LIGHT="gemini-flash"; ROUTING_MEDIUM="claude"
    elif [ -n "$ANTHROPIC_API_KEY" ]; then
        ROUTING_LIGHT="claude"
    elif [ -n "$GEMINI_API_KEY" ]; then
        ROUTING_LIGHT="gemini-flash"
    fi
    # Override to ANY model_name (custom/extra models supported).
    [ -n "$KIOS_ROUTING_LIGHT" ]  && ROUTING_LIGHT="$KIOS_ROUTING_LIGHT"
    [ -n "$KIOS_ROUTING_MEDIUM" ] && ROUTING_MEDIUM="$KIOS_ROUTING_MEDIUM"
    # Enable only once at least a light model is resolved.
    [ -n "$ROUTING_LIGHT" ] && ROUTING_ENABLED="true"
fi

# Context window guard — paksa picoclaw mengompres riwayat LEBIH AWAL agar ukuran
# request tidak pernah melebihi batas model free-tier (cegah error 413
# "Request too large", mis. Groq/Cerebras free tier).
#   KIOS_CONTEXT_WINDOW    = batas token konteks (default 12000; aman utk free tier).
#                            Naikkan jika model Anda berkonteks besar & tier-nya lapang.
#   KIOS_SUMMARIZE_PERCENT = mulai ringkas saat konteks capai % ini (default 70).
CONTEXT_WINDOW="${KIOS_CONTEXT_WINDOW:-12000}"
SUMMARIZE_PERCENT="${KIOS_SUMMARIZE_PERCENT:-70}"

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
      "context_window": $CONTEXT_WINDOW,
      "summarize_token_percent": $SUMMARIZE_PERCENT,
      "summarize_message_threshold": 16,
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
echo "kios-picoclaw: LLM primary=$PRIMARY_MODEL_NAME, fallbacks=$FALLBACKS, routing=$ROUTING_ENABLED (light=$ROUTING_LIGHT, medium=$ROUTING_MEDIUM)"
exec picoclaw gateway
