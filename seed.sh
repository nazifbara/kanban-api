#!/usr/bin/env bash

set -euo pipefail

BASE_URL="${BASE_URL:-http://localhost:8080}"
RESET_DATABASE=false

board_names=(
    "Q3 Product Launch"
    "Website Redesign"
    "Customer Support Operations"
    "Engineering Hiring"
)
board_descriptions=(
    "Coordinate the launch of the mobile checkout experience from discovery through release."
    "Track the redesign of the marketing site, including content, design, development, and analytics."
    "Manage recurring support improvements and high-priority customer issues for the operations team."
    "Move engineering candidates through sourcing, interviews, and the final offer process."
)
board_columns=(
    "Ideas|Planned|In progress|Design review|QA|Released"
    "Backlog|Content|Design|Development|Stakeholder review|Published"
    "New|Triaged|Investigating|Waiting on customer|Ready to close|Closed"
    "Sourced|Recruiter screen|Technical interview|On-site|Offer|Hired"
)

usage() {
    cat <<'EOF'
Usage: ./seed.sh [--reset]

Creates sample Kanban boards and columns through the HTTP API.

Environment variable:
    BASE_URL            API base URL (default: http://localhost:8080)

Options:
  --reset             Delete existing boards before seeding
  -h, --help          Show this help
EOF
}

while (($# > 0)); do
    case "$1" in
        --reset)
            RESET_DATABASE=true
            ;;
        -h|--help)
            usage
            exit 0
            ;;
        *)
            printf 'Unknown option: %s\n\n' "$1" >&2
            usage >&2
            exit 2
            ;;
    esac
    shift
done

require_command() {
    if ! command -v "$1" >/dev/null 2>&1; then
        printf 'Required command not found: %s\n' "$1" >&2
        exit 1
    fi
}

require_command curl
require_command jq

printf 'Waiting for API at %s...\n' "$BASE_URL"
for attempt in {1..30}; do
    if curl --silent --show-error --fail "$BASE_URL/api/boards" >/dev/null 2>&1; then
        break
    fi
    if ((attempt == 30)); then
        printf 'API did not become ready after 30 attempts\n' >&2
        exit 1
    fi
    sleep 1
done

if [[ "$RESET_DATABASE" == true ]]; then
    printf 'Resetting existing boards...\n'
    curl --silent --show-error --fail --request POST "$BASE_URL/reset" >/dev/null
fi

total_columns=0
for ((board_index = 0; board_index < ${#board_names[@]}; board_index++)); do
    board_number=$((board_index + 1))
    board_payload=$(jq -n \
        --arg name "${board_names[$board_index]}" \
        --arg description "${board_descriptions[$board_index]}" \
        '{name: $name, description: $description}')
    board_response=$(curl --silent --show-error --fail \
        --request POST \
        --header 'Content-Type: application/json' \
        --data "$board_payload" \
        "$BASE_URL/api/boards")
    board_id=$(printf '%s' "$board_response" | jq --raw-output --exit-status '.id')

    printf 'Created board %d: %s\n' "$board_number" "$board_id"

    IFS='|' read -r -a columns <<< "${board_columns[$board_index]}"
    for ((column_index = 0; column_index < ${#columns[@]}; column_index++)); do
        column_payload=$(jq -n \
            --arg title "${columns[$column_index]}" \
            --arg board_id "$board_id" \
            --argjson position "$column_index" \
            '{title: $title, board_id: $board_id, position: $position}')
        curl --silent --show-error --fail \
            --request POST \
            --header 'Content-Type: application/json' \
            --data "$column_payload" \
            "$BASE_URL/api/columns" >/dev/null
        total_columns=$((total_columns + 1))
    done

    printf '  Created %d Kanban columns\n' "${#columns[@]}"
done

printf 'Seed complete: %d boards, %d columns\n' "${#board_names[@]}" "$total_columns"