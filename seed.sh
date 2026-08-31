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

# Per board: columns separated by ';' (same order as board_columns entries).
# Within a column: tasks separated by '#'. Within a task: "Title~Description".
board_tasks=(
  "Explore one-tap checkout options~Research approaches for reducing checkout friction on mobile#Survey customers about payment preferences~Send a short survey to the customer panel;Define MVP scope for mobile checkout~Document must-have features for v1 launch#Set launch date target~Align with marketing on a target release window;Build payment provider integration~Wire up the new mobile payment SDK#Implement address autofill~Add autofill support for shipping addresses;Review checkout flow mockups~Walk through the updated Figma designs with stakeholders#Finalize error state designs~Confirm styling for failed payment states;Test checkout on iOS devices~Run through checkout flows on supported iOS versions#Load test payment endpoint~Verify the payment service handles peak traffic;Ship mobile checkout v1~Deploy the new checkout experience to production#Monitor launch metrics~Track conversion and error rates post-launch"
  "Audit existing site content~Catalog all pages and content across the marketing site#Gather competitor site examples~Collect reference sites for design inspiration;Write new homepage copy~Draft updated messaging for the homepage hero#Update pricing page copy~Refresh pricing tiers and feature descriptions;Create new homepage mockup~Design the updated homepage layout in Figma#Design mobile navigation~Rework the mobile nav menu for the new site;Build homepage components~Implement the new homepage in the CMS#Set up analytics tracking~Add event tracking for key site interactions;Present redesign to leadership~Walk leadership through the new site design#Collect feedback on new nav~Gather input from the sales team on navigation;Launch new homepage~Push the redesigned homepage live#Archive old site assets~Move outdated assets to storage"
  "Login issue reported by customer~Customer unable to log in after password reset#Billing discrepancy ticket~Customer reports being charged twice;Assign login issue to auth team~Route ticket to the authentication squad#Flag billing ticket as high priority~Escalate due to potential systemic issue;Reproduce login failure~Attempt to reproduce the reported login issue#Check billing logs~Review payment logs for duplicate charges;Request screenshot from customer~Ask customer for a screenshot of the error#Confirm charge details with customer~Ask for the last 4 digits of the card;Confirm login fix resolved issue~Verify the customer can now log in#Refund processed for duplicate charge~Confirm refund was issued;Login issue resolved~Ticket closed after successful password reset#Billing issue resolved~Ticket closed after refund confirmation"
  "Backend engineer candidate found~Sourced via LinkedIn outreach for backend role#Frontend engineer candidate found~Referral from current team member;Schedule recruiter call~Set up initial screening call with candidate#Confirm salary expectations~Discuss compensation range with candidate;Schedule coding interview~Set up technical interview with engineering panel#Review take-home assignment~Evaluate submitted coding exercise;Schedule on-site loop~Coordinate interviews with cross-functional team#Collect interviewer feedback~Gather scorecards from all interviewers;Prepare offer letter~Draft compensation package for approval#Extend offer to candidate~Send formal offer and answer questions;Onboard new backend engineer~Kick off onboarding process for new hire#Send welcome package~Ship laptop and swag to new hire"
)

usage() {
  cat <<'EOF'
Usage: ./seed.sh [--reset]

Creates sample Kanban boards, columns, and tasks through the HTTP API.

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
  -h | --help)
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
total_tasks=0
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

  IFS='|' read -r -a columns <<<"${board_columns[$board_index]}"
  IFS=';' read -r -a column_task_groups <<<"${board_tasks[$board_index]}"

  board_task_count=0
  for ((column_index = 0; column_index < ${#columns[@]}; column_index++)); do
    column_payload=$(jq -n \
      --arg title "${columns[$column_index]}" \
      --arg board_id "$board_id" \
      --argjson position "$column_index" \
      '{title: $title, board_id: $board_id, position: $position}')
    column_response=$(curl --silent --show-error --fail \
      --request POST \
      --header 'Content-Type: application/json' \
      --data "$column_payload" \
      "$BASE_URL/api/columns")
    column_id=$(printf '%s' "$column_response" | jq --raw-output --exit-status '.id')

    IFS='#' read -r -a tasks <<<"${column_task_groups[$column_index]}"
    for ((task_index = 0; task_index < ${#tasks[@]}; task_index++)); do
      task_title="${tasks[$task_index]%%~*}"
      task_description="${tasks[$task_index]#*~}"
      task_payload=$(jq -n \
        --arg title "$task_title" \
        --arg description "$task_description" \
        --arg board_id "$board_id" \
        --arg column_id "$column_id" \
        --argjson position "$task_index" \
        '{title: $title, description: $description, board_id: $board_id, column_id: $column_id, position: $position}')
      curl --silent --show-error --fail \
        --request POST \
        --header 'Content-Type: application/json' \
        --data "$task_payload" \
        "$BASE_URL/api/tasks" >/dev/null
      board_task_count=$((board_task_count + 1))
      total_tasks=$((total_tasks + 1))
    done
  done

  total_columns=$((total_columns + ${#columns[@]}))
  printf '  Created %d Kanban columns\n' "${#columns[@]}"
  printf '  Created %d tasks\n' "$board_task_count"
done

printf 'Seed complete: %d boards, %d columns, %d tasks\n' "${#board_names[@]}" "$total_columns" "$total_tasks"
