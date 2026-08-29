#!/bin/bash
# Autoresearch measure — pinned yardstick.
#   debt          : findings under .auto/lint.ref.yaml (lower is better)  [PRIMARY]
#   nolint_dirs   : raw //nolint directive count (lower is better, floor 0)
#   tests_passed  : go test packages passing (floor, must never drop)
#   test_funcs    : total Test*/Benchmark* funcs (floor, must never drop)
#   arch_viol     : Cordis architecture script violations (floor 0)
#   coverage      : backend statement coverage % (informational)
set -uo pipefail
cd "$(dirname "$0")/../backend"

# Hermetic lint result cache. golangci-lint's default cache is machine-wide, so
# entries written while analysing a different worktree are replayed carrying that
# checkout's absolute paths, which misattributes findings and can serve a stale
# verdict. Key the cache to this directory. Count-neutral: cold and shared-warm
# runs both report the same number of findings.
GOLANGCI_LINT_CACHE="${TMPDIR:-/tmp}/ar-lint-cache-$(pwd | cksum | awk '{print $1}')"
export GOLANGCI_LINT_CACHE

REF_CFG="$(cd .. && pwd)/.auto/lint.ref.yaml"

# Primary: pinned yardstick findings (never the mutable project config).
golangci-lint run -c "${REF_CFG}" > /tmp/ar_debt.txt 2>&1 || true
DEBT=$(grep -cE '\.go:[0-9]+:[0-9]+: ' /tmp/ar_debt.txt || true)

# Suppression reliance — anti-cheat signal.
NOLINT=$(rg '//\s*nolint' --glob '*.go' 2>/dev/null | wc -l | tr -d ' ')

# Regression floors. One covered run feeds both the pass floor and coverage.
go test -cover ./... > /tmp/ar_test.txt 2>&1 || true
TESTS_PASSED=$(grep -c '^ok' /tmp/ar_test.txt || true)
FAILS=$(grep -cE '^(FAIL|--- FAIL)' /tmp/ar_test.txt || true)
TEST_FUNCS=$(rg -c '^(func Test|func Benchmark)' --glob '*_test.go' 2>/dev/null | awk -F: '{s+=$2} END {print s+0}')

# Cordis architecture violations (count of FAIL lines emitted by the gate script).
ARCH_VIOL=$("../scripts/check_cordis_architecture.sh" 2>&1 | grep -c 'FAIL' || true)

COVERAGE=$(grep -oE 'coverage: [0-9.]+%' /tmp/ar_test.txt | awk '{gsub("%","",$2); s+=$2; n++} END {if(n>0) printf "%.2f", s/n; else print "0"}')

echo "METRIC debt=${DEBT}"
echo "METRIC nolint_dirs=${NOLINT}"
echo "METRIC tests_passed=${TESTS_PASSED}"
echo "METRIC test_funcs=${TEST_FUNCS}"
echo "METRIC arch_viol=${ARCH_VIOL}"
echo "METRIC coverage=${COVERAGE}"
echo "INFO test_failures=${FAILS}"
