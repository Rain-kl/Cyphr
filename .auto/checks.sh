#!/bin/bash
# Autoresearch GUARD — hard veto. Every line here protects an invariant that is
# unrelated to the primary metric, plus the anti-cheat red lines.
set -uo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "${ROOT}/backend"
STATUS=0
fail() { echo "GUARD FAIL: $1"; STATUS=1; }

source "${ROOT}/.auto/baseline.env"

# --- 1. Correctness -----------------------------------------------------------
go build ./... || fail "go build failed"
go vet ./...   || fail "go vet failed"

go test ./... > /tmp/ar_guard_test.txt 2>&1 || true
FAILS=$(grep -cE '^(FAIL|--- FAIL)' /tmp/ar_guard_test.txt || true)
[ "${FAILS}" = "0" ] || { grep -E '^(FAIL|--- FAIL)' /tmp/ar_guard_test.txt | head -20; fail "tests failing (${FAILS})"; }

# --- 2. Cordis architecture gate ---------------------------------------------
"${ROOT}/scripts/check_cordis_architecture.sh" > /dev/null 2>&1 || fail "cordis architecture check failed"

# --- 3. Project lint gate must stay clean ------------------------------------
PROJECT_LINT=$(golangci-lint run 2>&1 | grep -cE '\.go:[0-9]+:[0-9]+: ' || true)
[ "${PROJECT_LINT}" = "0" ] || { fail "project golangci-lint reports ${PROJECT_LINT} issues"; }

# --- 4. Anti-cheat: the yardstick itself is immutable ------------------------
REF_SHA_NOW=$(shasum -a 256 "${ROOT}/.auto/lint.ref.yaml" | awk '{print $1}')
[ "${REF_SHA_NOW}" = "${REF_SHA}" ] || fail "pinned yardstick .auto/lint.ref.yaml was modified"

# --- 5. Anti-cheat: the project gate may only ever be STRENGTHENED ------------
WEAK=$(python3 "${ROOT}/.auto/check_gate_weaken.py" 2>&1) || { echo "${WEAK}"; fail "project gate weakened"; }

# --- 6. Anti-cheat: no new suppressions --------------------------------------
NOLINT=$(rg '//\s*nolint' --glob '*.go' 2>/dev/null | wc -l | tr -d ' ')
[ "${NOLINT}" -le "${BASE_NOLINT}" ] || fail "nolint directives grew (${NOLINT} > ${BASE_NOLINT})"

# --- 7. Anti-cheat: no tests deleted, no packages lost -----------------------
TEST_FUNCS=$(rg -c '^(func Test|func Benchmark)' --glob '*_test.go' 2>/dev/null | awk -F: '{s+=$2} END {print s+0}')
[ "${TEST_FUNCS}" -ge "${BASE_TEST_FUNCS}" ] || fail "test funcs shrank (${TEST_FUNCS} < ${BASE_TEST_FUNCS})"
TEST_FILES=$(rg --files --glob '*_test.go' 2>/dev/null | wc -l | tr -d ' ')
[ "${TEST_FILES}" -ge "${BASE_TEST_FILES}" ] || fail "test files deleted (${TEST_FILES} < ${BASE_TEST_FILES})"
PASSED=$(grep -c '^ok' /tmp/ar_guard_test.txt || true)
[ "${PASSED}" -ge "${BASE_TESTS_PASSED}" ] || fail "passing packages shrank (${PASSED} < ${BASE_TESTS_PASSED})"

# --- 8. License headers on Go sources (CI gate) ------------------------------
"${ROOT}/scripts/update_go_license.sh" --check > /dev/null 2>&1 || fail "license header check failed"

if [ "${STATUS}" = "0" ]; then echo "CHECKS OK"; fi
exit ${STATUS}
