#!/bin/bash
# Rebuild the isolated demo database and restart the Go server used by the
# Playwright chapters. Requires a local PostgreSQL, the frontend build and a
# server binary; nothing here touches production data.
#
# Override with env vars when your paths differ:
#   FLOW_DIR   working directory that holds ./server, ./env, ./operator.hash
#   FLOW_DB    scratch database name
#   FLOW_BASE  server base URL
set -euo pipefail

DIR=${FLOW_DIR:-/tmp/warwick-flow}
DB=${FLOW_DB:-warwick_flow_demo}
BASE=${FLOW_BASE:-http://127.0.0.1:18081}
SCRIPT_DIR=$(cd "$(dirname "$0")" && pwd)
REPO_ROOT=$(cd "$SCRIPT_DIR/../.." && pwd)

pkill -f "server -env-file $DIR/env" 2>/dev/null || true
sleep 1.2
psql -d postgres -q -c "DROP DATABASE IF EXISTS $DB"
psql -d postgres -q -c "CREATE DATABASE $DB"

if [ ! -f "$DIR/operator.hash" ]; then
  (cd "$REPO_ROOT/backend" && go run "$SCRIPT_DIR/hash.go" 'operator-test-password-123' > "$DIR/operator.hash")
fi

cd "$DIR"
nohup ./server -env-file "$DIR/env" > "$DIR/server.log" 2>&1 &
sleep 2
HASH=$(cat "$DIR/operator.hash")
psql -d "$DB" -q -c "INSERT INTO users(username,password_hash,role) VALUES('warwick-operator','$HASH','operator')"
curl -s -o /dev/null -w "reset health %{http_code}\n" "$BASE/api/health"
