#!/usr/bin/env bash
#
# build-and-run.sh — Build new-api (backend + frontend) and run it
# against the local SQLite database at /root/yaoj/it/new-api/data/one-api.db
#
# Usage:
#   ./build-and-run.sh          # build + run
#   ./build-and-run.sh build    # build only
#   ./build-and-run.sh run      # run only (assumes binary exists)
set -euo pipefail

ROOT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
cd "$ROOT_DIR"

BACKEND_BIN="$ROOT_DIR/new-api"
DB_PATH="$ROOT_DIR/data/one-api.db"
PORT="${NEW_API_PORT:-3000}"

# Default env for the backend process. Users can override any of these by
# exporting them before running the script (the `${VAR:-default}` form keeps
# the existing value when set).
DEFAULT_SQLITE_PATH="$DB_PATH?_busy_timeout=30000"
export SQLITE_PATH="${SQLITE_PATH:-$DEFAULT_SQLITE_PATH}"
export SQL_DSN="${SQL_DSN:-local}"
export PORT="${PORT:-3000}"
export NODE_TYPE="${NODE_TYPE:-master}"

# ----------------------------------------------------------------------------
# 0. Pre-flight: install tooling if missing
# ----------------------------------------------------------------------------
ensure_go() {
  if ! command -v go >/dev/null 2>&1; then
    echo "[*] Go not found; cannot install automatically in this env. Install Go >=1.22 and re-run." >&2
    return 1
  fi
}

ensure_bun() {
  if command -v bun >/dev/null 2>&1; then
    return 0
  fi
  echo "[*] bun not found; installing via official script..."
  curl -fsSL https://bun.sh/install | bash
  export BUN_INSTALL="$HOME/.bun"
  export PATH="$BUN_INSTALL/bin:$PATH"
}

# ----------------------------------------------------------------------------
# 1. Build frontend (default + classic) so embed.FS works
# ----------------------------------------------------------------------------
build_frontend() {
  echo "[*] Building frontend (default + classic)..."
  ensure_bun
  cd "$ROOT_DIR/web"
  bun install
  echo "[*] -> web/default"
  cd "$ROOT_DIR/web/default"
  DISABLE_ESLINT_PLUGIN=true bun run build
  echo "[*] -> web/classic"
  cd "$ROOT_DIR/web/classic"
  # classic build may fail due to pre-existing date-fns-tz export incompat;
  # fall back to placeholder dist so backend embed still compiles.
  if ! bun install || ! bun run build; then
    echo "[!] web/classic build failed; emitting placeholder dist."
    mkdir -p "$ROOT_DIR/web/classic/dist"
    cat >"$ROOT_DIR/web/classic/dist/index.html" <<'EOF'
<!DOCTYPE html><html><head><meta charset="utf-8"><title>new-api classic</title></head><body>Classic frontend not built.</body></html>
EOF
  fi
  cd "$ROOT_DIR"
}

build_placeholder_dist() {
  # Emit minimal dist stubs so backend embed compiles without a full frontend
  # build (useful for backend-only iteration).
  mkdir -p "$ROOT_DIR/web/default/dist" "$ROOT_DIR/web/classic/dist"
  [ -f "$ROOT_DIR/web/default/dist/index.html" ] || \
    cat >"$ROOT_DIR/web/default/dist/index.html" <<'EOF'
<!DOCTYPE html><html><head><meta charset="utf-8"><title>new-api</title></head><body>Frontend not built.</body></html>
EOF
  [ -f "$ROOT_DIR/web/classic/dist/index.html" ] || \
    cat >"$ROOT_DIR/web/classic/dist/index.html" <<'EOF'
<!DOCTYPE html><html><head><meta charset="utf-8"><title>new-api classic</title></head><body>Classic frontend not built.</body></html>
EOF
}

# ----------------------------------------------------------------------------
# 2. Build backend
# ----------------------------------------------------------------------------
build_backend() {
  echo "[*] Building backend..."
  ensure_go
  cd "$ROOT_DIR"
  VERSION="$(git describe --tags 2>/dev/null || echo 'dev')"
  CGO_ENABLED=1 go build \
    -ldflags "-s -w -X 'new-api/common.Version=$VERSION'" \
    -o "$BACKEND_BIN" ./
  echo "[*] Backend binary: $BACKEND_BIN"
}

# ----------------------------------------------------------------------------
# 3. Run backend against $DB_PATH
# ----------------------------------------------------------------------------
run_backend() {
  if [ ! -x "$BACKEND_BIN" ]; then
    echo "[!] Backend binary not found; building first..." >&2
    build_placeholder_dist
    build_backend
  fi
  if [ ! -f "$DB_PATH" ]; then
    echo "[!] Database file not found at $DB_PATH; creating an empty one." >&2
    mkdir -p "$(dirname "$DB_PATH")"
    : >"$DB_PATH"
  fi

  # Kill any stale new-api process that may still hold the port.
  local old_pid
  old_pid="$(ss -tlnp 2>/dev/null | grep -E "[:.]$PORT\b" | grep -oE 'pid=[0-9]+' | head -1 | cut -d= -f2 || true)"
  if [ -n "$old_pid" ]; then
    echo "[*] Stopping stale new-api process (pid=$old_pid) on port $PORT"
    kill -9 "$old_pid" 2>/dev/null || true
    sleep 1
  fi

  echo "[*] Starting new-api on port $PORT using SQLite: $DB_PATH"
  cd "$ROOT_DIR"
  setsid "$BACKEND_BIN" > /tmp/new-api.log 2>&1 < /dev/null &
  disown
  echo "[*] Backend pid=$!, log: /tmp/new-api.log"
}

# ----------------------------------------------------------------------------
# Dispatch
# ----------------------------------------------------------------------------
case "${1:-all}" in
  all)
    build_frontend
    build_backend
    run_backend
    ;;
  fb|full-build)
    build_frontend
    build_backend
    run_backend
    ;;
  frontend|web)
    build_frontend
    ;;
  backend|go)
    build_placeholder_dist
    build_backend
    ;;
  run)
    run_backend
    ;;
  *)
    echo "usage: $0 [all|frontend|backend|run|fb]" >&2
    exit 1
    ;;
esac