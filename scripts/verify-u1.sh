#!/usr/bin/env bash
# U1 (walking skeleton) checkpoint verification.
#
# 1. Runs the whole U1 test suite (no AWS; the same tests as CI).
# 2. Runs scan and plan end to end against the sandbox AWS account
#    (internal/cli/integration_test.go). This needs the fixture stack from
#    test/sandbox/u1-fixtures.yaml in ap-northeast-1, a valid SSO login, and the
#    read-only profile (default tag-refactor-readonly; override with
#    TAG_REFACTOR_IT_PROFILE).
#
# Unlike a plain `go test`, it fails when the sandbox test is skipped, so a
# missing profile can never look like a pass.
set -euo pipefail

cd "$(dirname "$0")/.."

# A Go installed after the shell started may not be on PATH yet (Windows).
if ! command -v go >/dev/null 2>&1; then
  for dir in "/c/Program Files/Go/bin" "/usr/local/go/bin"; do
    if [ -x "$dir/go" ] || [ -x "$dir/go.exe" ]; then
      export PATH="$dir:$PATH"
      break
    fi
  done
fi
export GOTOOLCHAIN=local

profile="${TAG_REFACTOR_IT_PROFILE:-tag-refactor-readonly}"

echo "verify-u1: running the U1 test suite"
go test -mod=readonly -count=1 ./...

echo "verify-u1: running scan and plan against the sandbox account (profile $profile)"
status=0
out="$(TAG_REFACTOR_IT_PROFILE="$profile" go test -mod=readonly -count=1 -tags integration -run 'TestIntegration' -v ./internal/cli/... 2>&1)" || status=$?
printf '%s\n' "$out"
if [ "$status" -ne 0 ]; then
  echo "verify-u1: the sandbox test failed" >&2
  exit "$status"
fi
if ! printf '%s\n' "$out" | grep -q -- '--- PASS: TestIntegrationSandboxScanAndPlan'; then
  echo "verify-u1: the sandbox test did not run (skipped or not found)" >&2
  exit 1
fi
echo "verify-u1: OK"
