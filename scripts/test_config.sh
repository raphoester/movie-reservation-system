#!/usr/bin/env bash

set -e

[ -z "${APP}" ] && echo "please, set APP env variable (file path)" && exit 1

if [ ! -f "${APP}/main.go" ]; then
    echo "File ${APP}/main.go not found, skipping"
    exit 0
fi

for config in "${APP}/configs"/*.yaml; do
    # Skip generated *.resolved.yaml files — they are derived outputs, not source configs,
    # and would produce an invalid ENVIRONMENT value (e.g. "dev.resolved") if included.
    [[ "$config" == *.resolved.yaml ]] && continue
    env=$(basename "$config" .yaml)
    echo "Testing configuration for ${APP} in ${env}"
    (cd "${APP}" && \
      VALIDATE_CONFIG_ONLY=1 \
      ENVIRONMENT="$env" \
      go run .)
done
