#!/usr/bin/env bash
set -euo pipefail
umask 077
project_root="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
if command -v node >/dev/null 2>&1; then
  node "$project_root/scripts/prepare-railway.cjs"
else
  if ! command -v docker >/dev/null 2>&1 || ! docker info >/dev/null 2>&1; then
    echo 'Open Docker Desktop, then run this command again.'; exit 1
  fi
  docker run --rm --mount "type=bind,source=$project_root,target=/workspace" -w /workspace node:24-bookworm-slim node scripts/prepare-railway.cjs
fi
