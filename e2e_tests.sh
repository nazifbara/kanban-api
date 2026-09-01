#!/usr/bin/env bash
#
# End-to-end edge case tests for the kanban API.
#
# WARNING: this script calls POST /reset at startup, which truncates
# identities (and everything that cascades from them: boards, columns,
# tasks, refresh tokens). Only run this against a disposable/test database.
# Pass --skip-reset to opt out (existing data from a prior run may then
# cause a few tests, like "sign up with duplicate email", to behave
# differently).
#
# Some assertions are marked [INFO] instead of [PASS]/[FAIL]: these cover
# behavior inferred from code paths we couldn't fully verify (e.g. exact
# wording/shape of decodeJSONBody or respondWithError for non-APIErr
# errors, or refresh-token revocation/expiry which can't be triggered
# through the HTTP API alone). They print the actual response so you can
# eyeball it rather than asserting a possibly-wrong expected value.

set -uo pipefail

BASE_URL="${BASE_URL:-http://localhost:8080}"
SKIP_RESET=false

usage() {
  cat <<'EOF'
Usage: ./e2e_tests.sh [--skip-reset]

Runs an end-to-end edge-case test suite against the kanban API.

Environment variable:
    BASE_URL       API base URL (default: http://localhost:8080)

Options:
  --skip-reset   Don't call POST /reset before running (see warning in file header)
  -h, --help     Show this help
EOF
}

while (($# > 0)); do
  case "$1" in
  --skip-reset) SKIP_RESET=true ;;
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

TESTS_RUN=0
TESTS_PASSED=0
TESTS_FAILED=0

# A syntactically valid UUID that will never exist after a fresh reset.
NIL_UUID="00000000-0000-0000-0000-000000000000"
MISSING_UUID="11111111-1111-1111-1111-111111111111"

RESP_STATUS=""
RESP_BODY=""

do_curl() {
  # $1 method  $2 path  $3 token ('' = none)  $4 data ('' = none, otherwise raw JSON)
  local method=$1 path=$2 token=$3 data=$4
  local args=(--silent --write-out '\n%{http_code}' --request "$method" --header 'Content-Type: application/json')
  [[ -n "$token" ]] && args+=(--header "Authorization: Bearer $token")
  [[ -n "$data" ]] && args+=(--data "$data")
  local response
  response=$(curl "${args[@]}" "$BASE_URL$path")
  RESP_STATUS="${response##*$'\n'}"
  RESP_BODY="${response%$'\n'*}"
}

# $1 desc  $2 method  $3 path  $4 token  $5 data  $6 expected_status
run() {
  local desc=$1 method=$2 path=$3 token=$4 data=$5 expected=$6
  do_curl "$method" "$path" "$token" "$data"
  TESTS_RUN=$((TESTS_RUN + 1))
  if [[ "$RESP_STATUS" == "$expected" ]]; then
    TESTS_PASSED=$((TESTS_PASSED + 1))
    printf '  [PASS] %s\n' "$desc"
  else
    TESTS_FAILED=$((TESTS_FAILED + 1))
    printf '  [FAIL] %s (expected %s, got %s)\n' "$desc" "$expected" "$RESP_STATUS"
    printf '         %s\n' "$RESP_BODY"
  fi
}

# Same call shape, but only reports the outcome instead of asserting it.
observe() {
  local desc=$1 method=$2 path=$3 token=$4 data=$5
  do_curl "$method" "$path" "$token" "$data"
  printf '  [INFO] %s -> status %s\n' "$desc" "$RESP_STATUS"
  printf '         %s\n' "$RESP_BODY"
}

section() { printf '\n== %s ==\n' "$1"; }

jid() { printf '%s' "$1" | jq -r '.id'; }
jfield() { printf '%s' "$1" | jq -r ".$2"; }

json_array_has_id() {
  # $1 = json array body, $2 = id to look for
  printf '%s' "$1" | jq -e --arg id "$2" 'map(.id) | index($id) != null' >/dev/null 2>&1
}

# ---------------------------------------------------------------------------
# Setup
# ---------------------------------------------------------------------------

printf 'Waiting for API at %s...\n' "$BASE_URL"
for attempt in {1..30}; do
  if curl --silent --show-error --output /dev/null "$BASE_URL/api/boards" 2>/dev/null; then
    break
  fi
  if ((attempt == 30)); then
    printf 'API did not become ready after 30 attempts\n' >&2
    exit 1
  fi
  sleep 1
done

if [[ "$SKIP_RESET" == false ]]; then
  printf 'Resetting database...\n'
  run 'POST /reset succeeds' POST /reset '' '' 200
fi

# ---------------------------------------------------------------------------
# Sign-up edge cases
# ---------------------------------------------------------------------------

section 'Sign-up'

USER_A_EMAIL='usera@example.com'
USER_A_PASSWORD='correct-password-a'
USER_B_EMAIL='userb@example.com'
USER_B_PASSWORD='correct-password-b'

valid_signup_a=$(jq -n --arg email "$USER_A_EMAIL" --arg password "$USER_A_PASSWORD" \
  '{email: $email, password: $password, first_name: "Ada", last_name: "Alpha"}')
run 'sign up user A with valid data' POST /api/sign-up '' "$valid_signup_a" 201
run 'sign up with duplicate email -> conflict' POST /api/sign-up '' "$valid_signup_a" 409
run 'sign up with missing email' POST /api/sign-up '' \
  '{"password":"x","first_name":"A","last_name":"B"}' 400
run 'sign up with invalid email format' POST /api/sign-up '' \
  '{"email":"not-an-email","password":"x","first_name":"A","last_name":"B"}' 400
run 'sign up with missing password' POST /api/sign-up '' \
  '{"email":"nopass@example.com","first_name":"A","last_name":"B"}' 400
run 'sign up with malformed JSON body' POST /api/sign-up '' '{not json' 400

valid_signup_b=$(jq -n --arg email "$USER_B_EMAIL" --arg password "$USER_B_PASSWORD" \
  '{email: $email, password: $password, first_name: "Bea", last_name: "Beta"}')
run 'sign up user B with valid data' POST /api/sign-up '' "$valid_signup_b" 201

# ---------------------------------------------------------------------------
# Login edge cases
# ---------------------------------------------------------------------------

section 'Login'

login_a_payload=$(jq -n --arg email "$USER_A_EMAIL" --arg password "$USER_A_PASSWORD" '{email: $email, password: $password}')
run 'login with correct credentials' POST /api/login '' "$login_a_payload" 200
TOKEN_A=$(jfield "$RESP_BODY" token)
REFRESH_TOKEN_A=$(jfield "$RESP_BODY" refresh_token)

run 'login with wrong password' POST /api/login '' \
  "$(jq -n --arg email "$USER_A_EMAIL" '{email: $email, password: "totally-wrong"}')" 404
run 'login with non-existent email' POST /api/login '' \
  '{"email":"nobody@example.com","password":"x"}' 404
run 'login with missing password' POST /api/login '' \
  "$(jq -n --arg email "$USER_A_EMAIL" '{email: $email}')" 400
run 'login with invalid email format' POST /api/login '' \
  '{"email":"not-an-email","password":"x"}' 400
run 'login with malformed JSON body' POST /api/login '' '{not json' 400

login_b_payload=$(jq -n --arg email "$USER_B_EMAIL" --arg password "$USER_B_PASSWORD" '{email: $email, password: $password}')
run 'login user B with correct credentials' POST /api/login '' "$login_b_payload" 200
TOKEN_B=$(jfield "$RESP_BODY" token)

# ---------------------------------------------------------------------------
# Refresh edge cases
# ---------------------------------------------------------------------------

section 'Refresh'

run 'refresh with valid refresh token' POST /api/refresh "$REFRESH_TOKEN_A" '' 201
run 'refresh with missing Authorization header' POST /api/refresh '' '' 400
run 'refresh with bogus/unknown refresh token' POST /api/refresh 'not-a-real-refresh-token' '' 404
observe 'refresh with an access (JWT) token instead of a refresh token' POST /api/refresh "$TOKEN_A" ''

# ---------------------------------------------------------------------------
# Auth gate on protected routes
# ---------------------------------------------------------------------------

section 'Auth gate'

run 'GET /api/boards without a token' GET /api/boards '' '' 401
run 'GET /api/boards with a garbage token' GET /api/boards 'garbage.token.value' '' 401
run 'POST /api/boards without a token' POST /api/boards '' '{"name":"X"}' 401

# ---------------------------------------------------------------------------
# Boards
# ---------------------------------------------------------------------------

section 'Boards'

run 'create board with missing name' POST /api/boards "$TOKEN_A" '{"description":"no name"}' 400
run 'create board with malformed JSON' POST /api/boards "$TOKEN_A" '{not json' 400

run 'create board A (owned by user A)' POST /api/boards "$TOKEN_A" \
  '{"name":"Board A","description":"Main test board for user A"}' 201
BOARD_A_ID=$(jid "$RESP_BODY")

run 'create second board A2 (owned by user A, for IDOR checks)' POST /api/boards "$TOKEN_A" \
  '{"name":"Board A2","description":"Second board for user A"}' 201
BOARD_A2_ID=$(jid "$RESP_BODY")

run 'create board B (owned by user B)' POST /api/boards "$TOKEN_B" \
  '{"name":"Board B","description":"Board for user B"}' 201
BOARD_B_ID=$(jid "$RESP_BODY")

run 'get board A as owner' GET "/api/boards/$BOARD_A_ID" "$TOKEN_A" '' 200
run 'get board A as non-owner (user B) -> forbidden' GET "/api/boards/$BOARD_A_ID" "$TOKEN_B" '' 403
run 'get non-existent board' GET "/api/boards/$MISSING_UUID" "$TOKEN_A" '' 404
observe 'get board with a non-UUID path segment' GET '/api/boards/not-a-uuid' "$TOKEN_A" ''

do_curl GET /api/boards "$TOKEN_A" ''
if json_array_has_id "$RESP_BODY" "$BOARD_A_ID" && ! json_array_has_id "$RESP_BODY" "$BOARD_B_ID"; then
  TESTS_RUN=$((TESTS_RUN + 1))
  TESTS_PASSED=$((TESTS_PASSED + 1))
  printf '  [PASS] GET /api/boards for user A only lists boards A owns\n'
else
  TESTS_RUN=$((TESTS_RUN + 1))
  TESTS_FAILED=$((TESTS_FAILED + 1))
  printf '  [FAIL] GET /api/boards for user A only lists boards A owns\n'
  printf '         %s\n' "$RESP_BODY"
fi

run 'update board A with empty name' PUT "/api/boards/$BOARD_A_ID" "$TOKEN_A" '{"name":""}' 400
run 'update board A as non-owner (user B) -> forbidden' PUT "/api/boards/$BOARD_A_ID" "$TOKEN_B" '{"name":"Hijacked"}' 403
# Note: handlerUpdateBoard responds 201 on success even though it's an update, not a create.
run 'update board A with valid data' PUT "/api/boards/$BOARD_A_ID" "$TOKEN_A" '{"name":"Board A Renamed"}' 201

run 'delete board as non-owner (user B) -> forbidden' DELETE "/api/boards/$BOARD_A2_ID" "$TOKEN_B" '' 403
run 'delete non-existent board' DELETE "/api/boards/$MISSING_UUID" "$TOKEN_A" '' 404

# ---------------------------------------------------------------------------
# Columns
# ---------------------------------------------------------------------------

section 'Columns'

run 'create column with missing title' POST "/api/boards/$BOARD_A_ID/columns" "$TOKEN_A" '{"position":0}' 400
run 'create column with negative position' POST "/api/boards/$BOARD_A_ID/columns" "$TOKEN_A" \
  '{"title":"Bad","position":-1}' 400
run 'create column with position beyond current count (0 columns exist)' POST "/api/boards/$BOARD_A_ID/columns" "$TOKEN_A" \
  '{"title":"Bad","position":5}' 400
run 'create column on a board owned by another user -> forbidden' POST "/api/boards/$BOARD_A_ID/columns" "$TOKEN_B" \
  '{"title":"Intruder","position":0}' 403
run 'create column on non-existent board' POST "/api/boards/$MISSING_UUID/columns" "$TOKEN_A" \
  '{"title":"X","position":0}' 404
run 'create column with malformed JSON' POST "/api/boards/$BOARD_A_ID/columns" "$TOKEN_A" '{not json' 400

run 'create column "To Do" at position 0' POST "/api/boards/$BOARD_A_ID/columns" "$TOKEN_A" \
  '{"title":"To Do","position":0}' 201
COLUMN_1_ID=$(jid "$RESP_BODY")

run 'create column "Done" at position 1 (append)' POST "/api/boards/$BOARD_A_ID/columns" "$TOKEN_A" \
  '{"title":"Done","position":1}' 201
COLUMN_2_ID=$(jid "$RESP_BODY")

run 'create column at position beyond count once 2 exist (max valid is 2)' POST "/api/boards/$BOARD_A_ID/columns" "$TOKEN_A" \
  '{"title":"Bad","position":3}' 400

run 'create a column on board B (for cross-board scoping checks)' POST "/api/boards/$BOARD_B_ID/columns" "$TOKEN_B" \
  '{"title":"Board B column","position":0}' 201
BOARD_B_COLUMN_ID=$(jid "$RESP_BODY")

run 'list columns for board A' GET "/api/boards/$BOARD_A_ID/columns" "$TOKEN_A" '' 200
run 'list columns for a board owned by another user -> forbidden' GET "/api/boards/$BOARD_A_ID/columns" "$TOKEN_B" '' 403

run 'patch column with empty title' PATCH "/api/boards/$BOARD_A_ID/columns/$COLUMN_1_ID" "$TOKEN_A" '{"title":""}' 400
run 'patch column with out-of-range position' PATCH "/api/boards/$BOARD_A_ID/columns/$COLUMN_1_ID" "$TOKEN_A" \
  '{"position":99}' 400
run 'patch non-existent column' PATCH "/api/boards/$BOARD_A_ID/columns/$MISSING_UUID" "$TOKEN_A" '{"title":"X"}' 404
run 'patch column using a column ID that belongs to a different board' \
  PATCH "/api/boards/$BOARD_A_ID/columns/$BOARD_B_COLUMN_ID" "$TOKEN_A" '{"title":"X"}' 404
run 'patch column as non-owner (user B) -> forbidden' \
  PATCH "/api/boards/$BOARD_A_ID/columns/$COLUMN_1_ID" "$TOKEN_B" '{"title":"Hijack"}' 403
run 'patch column A1 with valid title change' PATCH "/api/boards/$BOARD_A_ID/columns/$COLUMN_1_ID" "$TOKEN_A" \
  '{"title":"To Do (renamed)"}' 200

run 'delete non-existent column' DELETE "/api/boards/$BOARD_A_ID/columns/$MISSING_UUID" "$TOKEN_A" '' 404
run 'delete column using a column ID that belongs to a different board' \
  DELETE "/api/boards/$BOARD_A_ID/columns/$BOARD_B_COLUMN_ID" "$TOKEN_A" '' 404
run 'delete column as non-owner (user B) -> forbidden' \
  DELETE "/api/boards/$BOARD_A_ID/columns/$COLUMN_1_ID" "$TOKEN_B" '' 403

# ---------------------------------------------------------------------------
# Tasks
# ---------------------------------------------------------------------------

section 'Tasks'

run 'create task with missing title' POST "/api/boards/$BOARD_A_ID/columns/$COLUMN_1_ID/tasks" "$TOKEN_A" \
  '{"description":"no title","position":0}' 400
run 'create task with negative position' POST "/api/boards/$BOARD_A_ID/columns/$COLUMN_1_ID/tasks" "$TOKEN_A" \
  '{"title":"Bad","position":-1}' 400
run 'create task with position beyond current count (0 tasks exist)' POST "/api/boards/$BOARD_A_ID/columns/$COLUMN_1_ID/tasks" "$TOKEN_A" \
  '{"title":"Bad","position":5}' 400

long_title=$(printf 'a%.0s' {1..256})
run 'create task with a 256-character title (limit is 255)' POST "/api/boards/$BOARD_A_ID/columns/$COLUMN_1_ID/tasks" "$TOKEN_A" \
  "$(jq -n --arg title "$long_title" '{title: $title, position: 0}')" 400

run 'create task on a column belonging to a different board' \
  POST "/api/boards/$BOARD_A_ID/columns/$BOARD_B_COLUMN_ID/tasks" "$TOKEN_A" '{"title":"X","position":0}' 404
run 'create task on a board owned by another user -> forbidden' \
  POST "/api/boards/$BOARD_A_ID/columns/$COLUMN_1_ID/tasks" "$TOKEN_B" '{"title":"X","position":0}' 403
run 'create task with malformed JSON' POST "/api/boards/$BOARD_A_ID/columns/$COLUMN_1_ID/tasks" "$TOKEN_A" '{not json' 400

run 'create task "Write tests" at position 0' POST "/api/boards/$BOARD_A_ID/columns/$COLUMN_1_ID/tasks" "$TOKEN_A" \
  '{"title":"Write tests","description":"Cover the edge cases","position":0}' 201
TASK_1_ID=$(jid "$RESP_BODY")

run 'create task "Ship it" at position 1' POST "/api/boards/$BOARD_A_ID/columns/$COLUMN_1_ID/tasks" "$TOKEN_A" \
  '{"title":"Ship it","description":"Deploy to prod","position":1}' 201
TASK_2_ID=$(jid "$RESP_BODY")

run 'list tasks in column 1' GET "/api/boards/$BOARD_A_ID/columns/$COLUMN_1_ID/tasks" "$TOKEN_A" '' 200
run 'list tasks for the whole board' GET "/api/boards/$BOARD_A_ID/tasks" "$TOKEN_A" '' 200
run 'list tasks in a column belonging to a different board' \
  GET "/api/boards/$BOARD_A_ID/columns/$BOARD_B_COLUMN_ID/tasks" "$TOKEN_A" '' 404
run 'list board tasks as non-owner -> forbidden' GET "/api/boards/$BOARD_A_ID/tasks" "$TOKEN_B" '' 403

run 'update task with empty title' PATCH "/api/boards/$BOARD_A_ID/tasks/$TASK_1_ID" "$TOKEN_A" '{"title":""}' 400
run 'update task with a 256-character title' PATCH "/api/boards/$BOARD_A_ID/tasks/$TASK_1_ID" "$TOKEN_A" \
  "$(jq -n --arg title "$long_title" '{title: $title}')" 400
run 'update task with an explicit nil column_id' PATCH "/api/boards/$BOARD_A_ID/tasks/$TASK_1_ID" "$TOKEN_A" \
  "$(jq -n --arg cid "$NIL_UUID" '{column_id: $cid}')" 400
run 'update task position out of range' PATCH "/api/boards/$BOARD_A_ID/tasks/$TASK_1_ID" "$TOKEN_A" '{"position":99}' 400
run 'move task to a column in a different board' PATCH "/api/boards/$BOARD_A_ID/tasks/$TASK_1_ID" "$TOKEN_A" \
  "$(jq -n --arg cid "$BOARD_B_COLUMN_ID" '{column_id: $cid}')" 404
run 'update non-existent task' PATCH "/api/boards/$BOARD_A_ID/tasks/$MISSING_UUID" "$TOKEN_A" '{"title":"X"}' 404
run 'update task owned by another user -> forbidden' \
  PATCH "/api/boards/$BOARD_A_ID/tasks/$TASK_1_ID" "$TOKEN_B" '{"title":"Hijack"}' 403
run 'update task A1 with a valid title change' PATCH "/api/boards/$BOARD_A_ID/tasks/$TASK_1_ID" "$TOKEN_A" \
  '{"title":"Write more tests"}' 200
run 'move task 2 to column 2 at position 0' PATCH "/api/boards/$BOARD_A_ID/tasks/$TASK_2_ID" "$TOKEN_A" \
  "$(jq -n --arg cid "$COLUMN_2_ID" '{column_id: $cid, position: 0}')" 200

run 'delete task owned by another user -> forbidden' \
  DELETE "/api/boards/$BOARD_A_ID/tasks/$TASK_1_ID" "$TOKEN_B" '' 403
run 'delete non-existent task' DELETE "/api/boards/$BOARD_A_ID/tasks/$MISSING_UUID" "$TOKEN_A" '' 404

# ---------------------------------------------------------------------------
# Cross-board IDOR observations
# ---------------------------------------------------------------------------
# handlerUpdateTask / handlerDeleteTask look tasks up purely by taskID and
# never re-check that the task actually belongs to the {boardID} in the
# path (only that the caller owns *some* board matching that boardID).
# These calls document what actually happens when a task ID from one
# board is addressed through a different board the same user owns.

section 'Cross-board task ID scoping (observation only)'

run 're-create task 1 (previous one was moved) to have a fresh task on board A' \
  POST "/api/boards/$BOARD_A_ID/columns/$COLUMN_1_ID/tasks" "$TOKEN_A" \
  '{"title":"Cross-board probe task","position":0}' 201
PROBE_TASK_ID=$(jid "$RESP_BODY")

observe 'PATCH a board-A task while addressing it through board A2 in the path' \
  PATCH "/api/boards/$BOARD_A2_ID/tasks/$PROBE_TASK_ID" "$TOKEN_A" '{"title":"Patched via wrong board path"}'
observe 'DELETE a board-A task while addressing it through board A2 in the path' \
  DELETE "/api/boards/$BOARD_A2_ID/tasks/$PROBE_TASK_ID" "$TOKEN_A" ''

# ---------------------------------------------------------------------------
# Cleanup / column & board deletion side effects
# ---------------------------------------------------------------------------

section 'Deletion side effects'

run 'delete column 1 on board A' DELETE "/api/boards/$BOARD_A_ID/columns/$COLUMN_1_ID" "$TOKEN_A" '' 204

do_curl GET "/api/boards/$BOARD_A_ID/columns" "$TOKEN_A" ''
remaining_position=$(jfield "$RESP_BODY" '[0].position')
TESTS_RUN=$((TESTS_RUN + 1))
if [[ "$remaining_position" == "0" ]]; then
  TESTS_PASSED=$((TESTS_PASSED + 1))
  printf '  [PASS] remaining column shifted down to position 0 after delete\n'
else
  TESTS_FAILED=$((TESTS_FAILED + 1))
  printf '  [FAIL] remaining column shifted down to position 0 after delete (got position=%s)\n' "$remaining_position"
  printf '         %s\n' "$RESP_BODY"
fi

run 'delete board A (cascades columns/tasks)' DELETE "/api/boards/$BOARD_A_ID" "$TOKEN_A" '' 204
run 'get board A after delete -> not found' GET "/api/boards/$BOARD_A_ID" "$TOKEN_A" '' 404
run 'list columns of a deleted board -> not found' GET "/api/boards/$BOARD_A_ID/columns" "$TOKEN_A" '' 404

# ---------------------------------------------------------------------------
# Summary
# ---------------------------------------------------------------------------

section 'Summary'
printf 'Ran: %d  Passed: %d  Failed: %d\n' "$TESTS_RUN" "$TESTS_PASSED" "$TESTS_FAILED"

if ((TESTS_FAILED > 0)); then
  exit 1
fi
exit 0
