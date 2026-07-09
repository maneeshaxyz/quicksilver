#!/usr/bin/env bash
# Dev runner: starts the Go backend (:8080) and the Vite frontend (:3000).
# Auto-generates required secrets into server/.env if they're missing.
# Ctrl-C stops both.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ENV_FILE="$ROOT/server/.env"

for bin in go pnpm openssl; do
  command -v "$bin" >/dev/null || { echo "missing: $bin" >&2; exit 1; }
done

# --- ensure server/.env exists ---
if [[ ! -f "$ENV_FILE" ]]; then
  echo "==> creating server/.env from .env.example"
  cp "$ROOT/server/.env.example" "$ENV_FILE"
fi

# set_secret KEY VALUE — fill in only when the key is empty/placeholder.
set_secret() {
  local key="$1" val="$2" cur
  cur="$(grep -E "^${key}=" "$ENV_FILE" | head -1 | cut -d= -f2-)"
  if [[ -n "$cur" ]]; then return; fi   # already set, leave it
  echo "==> generating $key"
  # portable in-place edit (macOS + GNU sed)
  if grep -qE "^${key}=" "$ENV_FILE"; then
    sed -i.bak -E "s|^${key}=.*|${key}=${val}|" "$ENV_FILE" && rm -f "$ENV_FILE.bak"
  else
    printf '%s=%s\n' "$key" "$val" >> "$ENV_FILE"
  fi
}

set_secret QUICKSILVER_JWT_SECRET      "$(openssl rand -base64 48)"
set_secret QUICKSILVER_SESSION_SEAL_KEY "$(openssl rand -hex 32)"

# --- deps ---
echo "==> go mod tidy"
(cd "$ROOT/server" && go mod tidy)
echo "==> pnpm install"
(cd "$ROOT" && pnpm install --silent)

# --- run both, kill both on exit ---
pids=()
cleanup() {
  echo
  echo "==> stopping…"
  for pid in "${pids[@]}"; do kill "$pid" 2>/dev/null || true; done
  wait 2>/dev/null || true
}
trap cleanup INT TERM EXIT

echo "==> backend  → http://localhost:8080"
(cd "$ROOT/server" && go run ./cmd/server) &
pids+=($!)

echo "==> frontend → http://localhost:3000/quicksilver/"
(cd "$ROOT" && pnpm dev) &
pids+=($!)

wait
