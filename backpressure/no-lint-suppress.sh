#!/bin/bash
# Prevents new lint suppression directives from being added to the codebase.
set -eou pipefail

# Patterns to block:
#   //nolint, // nolint - golangci-lint suppression
#   #nosec, // #nosec  - gosec suppression
#   //lint:ignore     - staticcheck/golint ignore
#   /*nolint*/, /* nolint */ - block comment style
RG_PATTERN='//\s*nolint|#nosec|//\s*lint:ignore|/\*\s*nolint'

FOUND=""

# Check changed lines in tracked files
CHANGED=$(git diff HEAD -U0 -- '*.go' | rg "^\+\+\+ b/|^@@|^\+.*($RG_PATTERN)" | awk '
    /^\+\+\+ b\//{file=substr($0,7)}
    /^@@/{split($3,a,","); gsub(/\+/,"",a[1]); line=a[1]}
    /^\+.*(\/\/.*nolint|#nosec|\/\/.*lint:ignore|\/\*.*nolint)/{print file":"line": "$0}
' || true)

# Check untracked .go files
UNTRACKED=$(git ls-files --others --exclude-standard '*.go' | \
    xargs -r rg -n "$RG_PATTERN" 2>/dev/null || true)

FOUND="${CHANGED}${CHANGED:+$'\n'}${UNTRACKED}"
FOUND=$(echo "$FOUND" | sed '/^$/d')  # Remove empty lines

if [ -n "$FOUND" ]; then
    echo "Error: Lint suppression directives are forbidden (nolint, nosec, lint:ignore)."
    echo "Fix the underlying issue instead of suppressing the warning."
    echo "$FOUND"
    exit 1
fi
