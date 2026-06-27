#!/usr/bin/env bash
set -euo pipefail

PID_FILE="local/.pids"

trap 'kill $(jobs -p) 2>/dev/null; rm -f "$PID_FILE"' EXIT SIGINT SIGTERM

run() {
  local dir="$1"
  local name
  name=$(grep "^name:" "${dir}configs/local.yaml" | awk '{print $2}')
  (cd "./${dir}" && ENVIRONMENT=local go run .) &
  echo "$name: $!" >> "$PID_FILE"
}

SEARCH_ROOT="${1:-internal}"

while IFS= read -r main_go; do
  dir="$(dirname "$main_go")/"
  [[ -f "${dir}configs/local.yaml" ]] || continue
  run "$dir"
done < <(find "$SEARCH_ROOT" -name main.go -type f | grep -v '^internal/tooling/' | sort)

wait
