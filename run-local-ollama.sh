#!/usr/bin/env bash
# run-local-ollama.sh — convenience launcher for running the kios Telegram bot
# LOCALLY with a local Ollama model instead of cloud LLMs.
#
# Non-intrusive by design:
#   - adds NO new tracked files beyond this script;
#   - reads secrets from deploy/.env (only the keys it needs, without sourcing
#     the whole file);
#   - renders its runtime config under .picoclaw-dev/run/ (already git-ignored),
#     so it never overwrites the sanitized .picoclaw-dev/config.json;
#   - runs PURE local: cloud API keys are intentionally NOT exported, so no
#     Groq/Gemini/Claude fallback or cost is involved.
#
# Usage:
#   ./run-local-ollama.sh                 # model llama3.2:3b (default)
#   ./run-local-ollama.sh qwen2.5:3b      # override model
#   KIOS_DRYRUN=1 ./run-local-ollama.sh   # show the plan, do NOT start the bot
#
# Prereqs: `ollama serve` running + `ollama pull llama3.2:3b`, and `make build`.
set -eu

ROOT="$(CDPATH= cd "$(dirname "$0")" && pwd)"
ENV_FILE="$ROOT/deploy/.env"
MODEL="${1:-${OLLAMA_MODEL:-llama3.2:3b}}"
OLLAMA_BASE_URL="${OLLAMA_BASE_URL:-http://localhost:11434/v1}"
RUNTIME_HOME="$ROOT/.picoclaw-dev/run"
# Keep the model resident so the bot never pays the (very slow on low-RAM/CPU
# hosts) cold model-load cost mid-request. -1 = never unload while serving.
OLLAMA_KEEP_ALIVE="${OLLAMA_KEEP_ALIVE:--1}"
# HTTP timeout picoclaw waits for a reply; local CPU inference needs headroom.
OLLAMA_REQUEST_TIMEOUT="${OLLAMA_REQUEST_TIMEOUT:-600}"
DRYRUN="${KIOS_DRYRUN:-0}"
[ "${1:-}" = "--dry-run" ] && { DRYRUN=1; MODEL="${OLLAMA_MODEL:-llama3.2:3b}"; }

die() { echo "FATAL: $*" >&2; exit 1; }

# --- secrets: read specific keys from deploy/.env without executing it ---
[ -f "$ENV_FILE" ] || die "$ENV_FILE tidak ada. Isi kredensial dulu (lihat deploy/env.example)."
read_env() {
	# last assignment wins; strip optional surrounding single/double quotes
	sed -n "s/^$1=//p" "$ENV_FILE" | tail -n1 | sed -e 's/^"\(.*\)"$/\1/' -e "s/^'\(.*\)'\$/\1/"
}
TELEGRAM_BOT_TOKEN="$(read_env TELEGRAM_BOT_TOKEN)"
UPSTASH_REDIS_URL="$(read_env UPSTASH_REDIS_URL)"
KIOS_ALLOW_FROM="$(read_env KIOS_ALLOW_FROM)"
KIOS_DEFAULT_ROLE="$(read_env KIOS_DEFAULT_ROLE)"
TZ_ENV="$(read_env TZ)"

miss=""
[ -z "$TELEGRAM_BOT_TOKEN" ] && miss="$miss TELEGRAM_BOT_TOKEN"
[ -z "$UPSTASH_REDIS_URL" ] && miss="$miss UPSTASH_REDIS_URL"
[ -z "$KIOS_ALLOW_FROM" ] && miss="$miss KIOS_ALLOW_FROM"
[ -n "$miss" ] && die "kredensial kosong di deploy/.env:$miss"

# --- binary ---
[ -x "$ROOT/build/picoclaw" ] || die "build/picoclaw belum ada — jalankan 'make build' dulu."
case ":$PATH:" in *":$ROOT/build:"*) : ;; *) PATH="$ROOT/build:$PATH" ;; esac

# --- ollama daemon + model ---
OLLAMA_PING="${OLLAMA_BASE_URL%/v1}"
if ! curl -fsS "$OLLAMA_PING/api/tags" >/dev/null 2>&1; then
	die "Ollama tidak merespons di $OLLAMA_PING — jalankan 'ollama serve' dulu."
fi
if command -v ollama >/dev/null 2>&1 && ! ollama list 2>/dev/null | awk 'NR>1{print $1}' | grep -qx "$MODEL"; then
	if [ "$DRYRUN" = "1" ]; then
		echo "(dry-run) model '$MODEL' belum ada — normalnya akan: ollama pull $MODEL"
	else
		echo "Model '$MODEL' belum ada. Menarik: ollama pull $MODEL"
		ollama pull "$MODEL"
	fi
fi

# --- compose environment for entrypoint (pure-local ollama) ---
mask() { case "${1:-}" in "") echo "(kosong)";; ?????*) echo "${1%"${1#?????}"}…(${#1} char)";; *) echo "•••";; esac; }
echo "── kios run-local (ollama) ──────────────────────────────"
echo "  model      : $MODEL"
echo "  ollama     : $OLLAMA_BASE_URL"
echo "  home       : $RUNTIME_HOME  (git-ignored)"
echo "  workspace  : $ROOT/workspace"
echo "  telegram   : token $(mask "$TELEGRAM_BOT_TOKEN")"
echo "  redis      : $(mask "$UPSTASH_REDIS_URL")"
echo "  allow_from : $KIOS_ALLOW_FROM"
echo "  role       : ${KIOS_DEFAULT_ROLE:-(default owner)}"
echo "  cloud LLM  : dinonaktifkan (pure local, tanpa fallback/biaya)"
echo "  keep_alive : $OLLAMA_KEEP_ALIVE | request_timeout: ${OLLAMA_REQUEST_TIMEOUT}s"
echo "─────────────────────────────────────────────────────────"

if [ "$DRYRUN" = "1" ]; then
	echo "(dry-run) OK — semua preflight lolos. Bot TIDAK dijalankan."
	exit 0
fi

# Warm up: load the model into RAM and pin it (keep_alive) BEFORE the bot starts,
# so picoclaw's first request hits an already-resident model instead of timing
# out during a slow cold load. On low-RAM hosts this first load can take minutes.
echo "Memuat model '$MODEL' ke memori (pin keep_alive=$OLLAMA_KEEP_ALIVE)… ini bisa lama pada load pertama."
if curl -fsS --max-time "$OLLAMA_REQUEST_TIMEOUT" "${OLLAMA_BASE_URL%/v1}/api/generate" \
	-d "{\"model\":\"$MODEL\",\"prompt\":\"ping\",\"stream\":false,\"keep_alive\":\"$OLLAMA_KEEP_ALIVE\",\"options\":{\"num_predict\":1}}" \
	>/dev/null 2>&1; then
	echo "Model siap (resident di memori)."
else
	echo "WARN: warmup model gagal/timeout — bot tetap lanjut, tapi balasan pertama mungkin lambat." >&2
fi

mkdir -p "$RUNTIME_HOME"
export TELEGRAM_BOT_TOKEN UPSTASH_REDIS_URL KIOS_ALLOW_FROM PATH
[ -n "$KIOS_DEFAULT_ROLE" ] && export KIOS_DEFAULT_ROLE
[ -n "$TZ_ENV" ] && export TZ="$TZ_ENV"
export PICOCLAW_HOME="$RUNTIME_HOME"
export KIOS_WORKSPACE_SRC="$ROOT/workspace"
export OLLAMA_MODEL="$MODEL"
export OLLAMA_BASE_URL OLLAMA_REQUEST_TIMEOUT
# Pure local: ensure no cloud key leaks into the rendered config.
unset GROQ_API_KEY GEMINI_API_KEY ANTHROPIC_API_KEY 2>/dev/null || true

echo "Memulai bot… (Ctrl-C untuk berhenti)"
exec sh "$ROOT/deploy/entrypoint.sh"
