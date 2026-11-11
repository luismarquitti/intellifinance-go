#!/usr/bin/env bash
# Operates only the explicitly prepared user-owned development environment.
set -euo pipefail
ROOT=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)
cd "$ROOT"
TOOLS_ENV=${TOOLS_ENV:-"$HOME/.local/share/intellifinance/tools/env.sh"}
if [[ -f "$TOOLS_ENV" ]]; then source "$TOOLS_ENV"; fi
LINTER=${GOLANGCI_LINT:-"$HOME/.local/share/intellifinance/tools/golangci-lint-2.15.0-linux-amd64/golangci-lint"}
UNIT=intellifinance-dev.service
ENV_FILE=${ENV_FILE:-"$HOME/.config/intellifinance/development.env"}
PGDATA=${PGDATA:-"$HOME/.local/share/intellifinance/pgdata"}
PGLOG=${PGLOG:-"$HOME/.local/share/intellifinance/postgres.log"}

load_env() {
 [[ -f "$ENV_FILE" ]] || { printf 'Missing private environment file: %s\n' "$ENV_FILE" >&2; exit 1; }
 set -a
 source "$ENV_FILE"
 set +a
 : "${DATABASE_URL:?DATABASE_URL is required}"
 python3 - <<'PY'
import os
from urllib.parse import urlsplit, unquote
u = urlsplit(os.environ['DATABASE_URL'])
protected = (os.environ.get('ENVIRONMENT') == 'personal' or u.port == 55433
             or 'personal' in unquote(u.path).lower()
             or 'personal' in unquote(u.username or '').lower()
             or bool(os.environ.get('PERSONAL_PASSWORD_HASH')))
if protected:
    raise SystemExit('Development/test launcher refuses protected personal target')
PY
}

case "${1:-help}" in
 build-web)
  npm ci --ignore-scripts --prefix web
  npm run build --prefix web
  ;;
 build-api)
  mkdir -p bin
  go build -o bin/api ./cmd/api
  go build -o bin/seed ./cmd/seed
  go build -o bin/import-ledger ./cmd/import-ledger
  ;;
 up)
  load_env
  [[ "${BIND_ADDR:-127.0.0.1}" == 127.0.0.1 ]] || { printf 'Refusing to expose development Admin bypass beyond loopback.\n' >&2; exit 1; }
  [[ "${ENVIRONMENT:-development}" == development ]] || { printf 'This command is for development only.\n' >&2; exit 1; }
  [[ -x "$ROOT/bin/api" ]] || { printf 'Run make build first.\n' >&2; exit 1; }
  [[ "${PORT:-8090}" =~ ^[1-9][0-9]{0,4}$ ]] && (( ${PORT:-8090} <= 65535 )) || { printf 'Invalid development PORT.\n' >&2; exit 1; }
  # The transient unit requires an absolute EnvironmentFile path.
  [[ "$ENV_FILE" == /* && -f "$ENV_FILE" ]] || { printf 'ENV_FILE must be an absolute regular file.\n' >&2; exit 1; }
  # Transient unit properties survive restart: unload before recreating.
  state=$(systemctl --user show "$UNIT" --property=LoadState --value) || [[ "$state" == not-found ]]
  old_pid=0
  if [[ "$state" != not-found ]]; then
   old_pid=$(systemctl --user show "$UNIT" --property=MainPID --value)
  fi
  [[ "$old_pid" =~ ^[0-9]+$ ]] || { printf 'Invalid MainPID.\n' >&2; exit 1; }
  python3 scripts/local-port.py preflight "${PORT:-8090}" "$old_pid"
  if [[ "$state" != not-found ]]; then
   systemctl --user stop "$UNIT"
   for _ in {1..30}; do
    state=$(systemctl --user show "$UNIT" --property=LoadState --value) || [[ "$state" == not-found ]]
    [[ "$state" == not-found ]] && break
    sleep 1
   done
   [[ "$state" == not-found ]] || { printf 'Previous unit did not unload.\n' >&2; exit 1; }
  fi
  python3 scripts/local-port.py preflight "${PORT:-8090}" 0
  systemd-run --user --unit="$UNIT" --collect --property="WorkingDirectory=$ROOT" --property="EnvironmentFile=$ENV_FILE" --property=Restart=on-failure --setenv=BIND_ADDR=127.0.0.1 --setenv=ENVIRONMENT=development --setenv="PORT=${PORT:-8090}" "$ROOT/bin/api"
  for _ in {1..30}; do
   pid=$(systemctl --user show "$UNIT" --property=MainPID --value)
   [[ "$pid" =~ ^[0-9]+$ ]] || { printf 'Invalid MainPID.\n' >&2; exit 1; }
   if [[ "$pid" != 0 ]] && systemctl --user is-active --quiet "$UNIT" &&
      python3 scripts/local-port.py owned "${PORT:-8090}" "$pid" 2>/dev/null; then
    if report=$(curl --noproxy '*' --max-time 3 -fsS "http://127.0.0.1:${PORT:-8090}/readyz" 2>/dev/null) &&
       confirmed_pid=$(systemctl --user show "$UNIT" --property=MainPID --value) &&
       [[ "$confirmed_pid" == "$pid" ]] &&
       systemctl --user is-active --quiet "$UNIT" &&
       python3 scripts/local-port.py owned "${PORT:-8090}" "$pid"; then
     printf '%s\n' "$report"
     exit 0
    fi
   fi
   sleep 1
  done
  printf 'API did not become ready; inspect make mvp-logs.\n' >&2
  exit 1
  ;;
 down) systemctl --user stop "$UNIT" ;;
 status)
  systemctl --user --no-pager status "$UNIT"
  ;;
 logs) journalctl --user -u "$UNIT" -n 100 --no-pager ;;
 seed)
  load_env
  [[ "${ENVIRONMENT:-development}" == development ]] || { printf 'Development seed only.\n' >&2; exit 1; }
  go run ./cmd/seed
  ;;
 pg-up)
  [[ -f "$PGDATA/PG_VERSION" ]] || { printf 'Prepared user-owned PostgreSQL cluster missing: %s\n' "$PGDATA" >&2; exit 1; }
  if ! pg_ctl -D "$PGDATA" status >/dev/null 2>&1; then pg_ctl -D "$PGDATA" -l "$PGLOG" -w start; fi
  ;;
 pg-down) pg_ctl -D "$PGDATA" -w -m fast stop ;;
 test)
  ENV_FILE=${TEST_ENV_FILE:-"$HOME/.config/intellifinance/test.env"}
  load_env
  python3 - <<'PY'
import os
from urllib.parse import urlsplit
url = urlsplit(os.environ['DATABASE_URL'])
if url.hostname not in ('127.0.0.1', 'localhost', '::1') or not url.path.endswith('_test'):
    raise SystemExit('Tests require an isolated local database whose name ends in _test')
PY
  go test -race -count=1 -json ./... | python3 scripts/check-test-results.py
  ;;
 lint)
  if [[ ! -x "$LINTER" ]]; then LINTER=golangci-lint; fi
  "$LINTER" run --timeout=5m
  ;;
 *) printf 'Usage: %s {build-web|build-api|up|down|status|logs|seed|pg-up|pg-down|test|lint}\n' "$0"; exit 2 ;;
esac
