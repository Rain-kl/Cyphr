#!/bin/bash
# Correctness gate: build + full tests + Cordis architecture checker (errors only)
set -euo pipefail
cd "$(dirname "$0")/../backend"

go build ./... 2>&1 | head -20
go test ./... 2>&1 | grep -vE '^(ok|---|\?|PASS)' | grep -v 'no test files' | head -40 || true
FAILS=$(go test ./... 2>&1 | grep -cE '^(FAIL|--- FAIL)' || true)
if [ "${FAILS}" != "0" ]; then echo "TESTS FAILED (${FAILS})"; exit 1; fi
"$(dirname "$0")/../scripts/check_cordis_architecture.sh" >/dev/null 2>&1 || { echo "CORDIS ARCH CHECK FAILED"; exit 1; }
echo "CHECKS OK"
