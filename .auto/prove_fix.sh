#!/bin/bash
# Mechanically prove a FIX iteration is load-bearing.
#
# Usage: .auto/prove_fix.sh <package> <changed source file> [<more files>...]
#
# Run immediately AFTER committing the fix, with a clean worktree. It reverts
# only the non-test source files to their pre-fix state (keeping the new test),
# runs the package tests, and requires them to FAIL. Then it restores HEAD.
# A fix nobody can break with a revert is not a fix.
set -uo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"

if [ ! -z "$(git -C "${ROOT}" status --porcelain)" ]; then
    echo "PROVE ABORT: worktree must be clean (commit the change first)"
    exit 2
fi

PKG="$1"; shift
SRC_FILES=("$@")
if [ "${#SRC_FILES[@]}" -eq 0 ]; then
    echo "PROVE ABORT: no source files given"
    exit 2
fi

cd "${ROOT}/backend" || exit 2

restore() {
    git -C "${ROOT}" checkout HEAD -- "${SRC_FILES[@]}" 2>/dev/null
}
trap restore EXIT

for f in "${SRC_FILES[@]}"; do
    if git -C "${ROOT}" cat-file -e "HEAD^:${f}" 2>/dev/null; then
        git -C "${ROOT}" checkout "HEAD^" -- "${f}" || { echo "PROVE ABORT: cannot revert ${f}"; exit 2; }
    else
        # File did not exist before this commit — removing it is the revert.
        rm -f "${ROOT}/${f}"
    fi
done

echo "--- tests against pre-fix source ---"
OUT=$(go test -count=1 "${PKG}" 2>&1)
RC=$?
echo "${OUT}" | tail -15
if [ "${RC}" -eq 0 ]; then
    echo "PROVE FAILED: tests still pass without the fix — this is not a real bug fix"
    exit 1
fi
if echo "${OUT}" | grep -qE '^(FAIL|--- FAIL)'; then
    KIND="assertion"
else
    KIND="compile"
fi
echo "PROVED: test fails without the fix (${KIND})"
exit 0
