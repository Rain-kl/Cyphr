#!/bin/bash
# Autoresearch measure: golangci-lint issue count (primary) + dupl subset + test packages
set -euo pipefail
cd "$(dirname "$0")/../backend"

# Primary metric: total golangci-lint issues (non-test code)
golangci-lint run > /tmp/ar_lint.txt 2>&1 || true
LINT_ISSUES=$(grep -cE '(^|[/\\])[^/\\:]+\.go:[0-9]+:' /tmp/ar_lint.txt || true)

# Secondary: dupl-specific issues
DUP_ISSUES=$(grep -cE '\(dupl\)$' /tmp/ar_lint.txt || true)

# Secondary: passing test packages (regression guard)
TESTS_PASSED=$(go test ./... 2>&1 | grep -c '^ok' || true)

echo "METRIC lint_issues=${LINT_ISSUES}"
echo "METRIC dup_issues=${DUP_ISSUES}"
echo "METRIC tests_passed=${TESTS_PASSED}"
