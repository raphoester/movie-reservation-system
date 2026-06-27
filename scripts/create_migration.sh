#!/bin/bash

MIGRATION_DIR="assets/migrations"

# Parse named arguments: db=<instance> name=<migration_name>
for arg in "$@"; do
    case $arg in
        db=*) ARG_DB="${arg#db=}" ;;
        name=*) ARG_NAME="${arg#name=}" ;;
    esac
done

# Determine timestamp command (gdate on Mac, date on Linux)
if command -v gdate &> /dev/null; then
    DATE_CMD="gdate"
elif date +%s%N &> /dev/null && [[ "$(date +%s%N)" != *"N"* ]]; then
    DATE_CMD="date"
else
    echo "Error: requires gdate (Mac: brew install coreutils) or GNU date"
    exit 1
fi

if [ ! -d "$MIGRATION_DIR" ]; then
    echo "Error: Migration directory '$MIGRATION_DIR' does not exist."
    exit 1
fi

# Resolve DB instance: use argument or fall back to interactive fzf
if [ -n "$ARG_DB" ]; then
    INSTANCE="$ARG_DB"
else
    if ! command -v fzf &> /dev/null; then
        echo "Error: interactive mode requires fzf (Mac: brew install fzf, Linux: apt install fzf)"
        echo "Alternatively, pass arguments: $0 db=<instance> name=<migration_name>"
        exit 1
    fi
    INSTANCE=$(
        find "$MIGRATION_DIR" -mindepth 1 -maxdepth 2 -type d | while IFS= read -r dir; do
            [ -z "$(find "$dir" -mindepth 1 -maxdepth 1 -type d 2>/dev/null)" ] && echo "${dir#${MIGRATION_DIR}/}"
        done | fzf --prompt="Select a db instance: "
    )
    if [ -z "$INSTANCE" ]; then
        echo "No instance selected. Exiting."
        exit 1
    fi
fi

# Resolve migration name: use argument or fall back to interactive prompt
if [ -n "$ARG_NAME" ]; then
    MIGRATION_NAME="$ARG_NAME"
else
    echo "Enter migration name (snake_case only):"
    read -r MIGRATION_NAME
fi

if [[ ! "$MIGRATION_NAME" =~ ^[a-z0-9_]+$ ]]; then
    echo "Error: Migration name must be in snake_case format."
    exit 1
fi

TIMESTAMP=$($DATE_CMD +%s%N | cut -b1-13)

MIGRATION_PATH="$MIGRATION_DIR/$INSTANCE"
UP_FILE="$MIGRATION_PATH/${TIMESTAMP}_${MIGRATION_NAME}.up.sql"
DOWN_FILE="$MIGRATION_PATH/${TIMESTAMP}_${MIGRATION_NAME}.down.sql"

mkdir -p "$MIGRATION_PATH"
echo "-- UP" > "$UP_FILE"
echo "-- DOWN" > "$DOWN_FILE"

echo "Migration files created:"
echo "  $UP_FILE"
echo "  $DOWN_FILE"
