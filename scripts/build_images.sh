#!/usr/bin/env bash
set -euo pipefail

DOCKER_DIR="assets/docker"

for dir in "$DOCKER_DIR"/*/; do
    name=$(basename "$dir")
    echo "Building movie-reservation-system/$name:latest"
    docker build -t "movie-reservation-system/$name:latest" "$dir"
done
