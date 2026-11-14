#!/usr/bin/env bash
# v0.2.0 U1 (walking skeleton) verification: the Construction Verification
# Command of the intent 261107-iac-cdk-pr. Run it by hand only, never in CI.
#
# 1. Checks with the read-only profile that the account is the sandbox account
#    and that the stack tag-refactor-cdk-skeleton exists (deployed beforehand
#    from the test app; test/verify/README-v02-u1.md says how).
# 2. Builds tag-refactor, runs scan, and makes a plan with the rule Env ->
#    Environment (test/verify/v02-u1-rules.yaml); the plan must hold the bucket
#    of the stack with the rename.
# 3. Clones the test-only repository into a temporary folder, runs npm ci
#    there, and runs iac --pr on the clone. cdk synth runs there without any AWS
#    or GitHub credentials (tag-refactor takes them away).
# 4. Checks that a pull request was opened and that its difference is the one
#    line of the key, then closes it and deletes its branch (unless
#    TAG_REFACTOR_KEEP_PR=1, for a person to look at it; then close it by hand).
#
# It needs: AWS credentials of the read-only profile (aws sso login), gh logged
# in with write access to the test-only repository, git, Node.js 18 or later,
# npm and Go. It writes nothing to AWS. It prints no credential.
set -euo pipefail

cd "$(dirname "$0")/../.."

: "${TAG_REFACTOR_SANDBOX_ACCOUNT_ID:?set the ID of the sandbox account}"
: "${TAG_REFACTOR_TEST_REPO:?set owner/name of the test-only repository}"
profile="${TAG_REFACTOR_IT_PROFILE:-tag-refactor-readonly}"
region="ap-northeast-1"
stack="tag-refactor-cdk-skeleton"
rules="test/verify/v02-u1-rules.yaml"
export GOTOOLCHAIN=local CGO_ENABLED=0

fail() {
  echo "v02-u1: $*" >&2
  exit 1
}

for tool in go git gh node npm aws; do
  command -v "$tool" >/dev/null 2>&1 || fail "$tool is needed"
done

work="$(mktemp -d)"
pr_url=""
cleanup() {
  if [ -n "$pr_url" ] && [ "${TAG_REFACTOR_KEEP_PR:-}" != "1" ]; then
    if ! gh pr close "$pr_url" --delete-branch >/dev/null 2>&1; then
      echo "v02-u1: close $pr_url and delete its branch by hand" >&2
    fi
  fi
  rm -rf "$work"
}
trap cleanup EXIT

echo "v02-u1: checking the account and the stack (profile $profile)"
account="$(aws sts get-caller-identity --profile "$profile" --query Account --output text)"
[ "$account" = "$TAG_REFACTOR_SANDBOX_ACCOUNT_ID" ] || fail "the profile $profile is not for the sandbox account"
# The read-only profile may read tags only (no cloudformation:DescribeStacks),
# so the stack is found through the tags CloudFormation puts on its bucket.
found="$(aws resourcegroupstaggingapi get-resources --region "$region" --profile "$profile" \
  --tag-filters "Key=aws:cloudformation:stack-name,Values=$stack" --resource-type-filters s3 \
  --query 'length(ResourceTagMappingList)' --output text)" || fail "could not read the tags of the stack $stack"
[ "$found" = "1" ] || fail "the stack $stack was not found (its bucket has no tags); deploy it first (test/verify/README-v02-u1.md)"

echo "v02-u1: building tag-refactor"
tool="$work/tag-refactor"
case "$(go env GOOS)" in windows) tool="$tool.exe" ;; esac
go build -mod=readonly -o "$tool" ./cmd/tag-refactor

# run_tool runs tag-refactor and accepts the exit codes 0 and 2 only.
run_tool() {
  local code=0
  "$tool" "$@" || code=$?
  if [ "$code" -ne 0 ] && [ "$code" -ne 2 ]; then
    fail "tag-refactor $1 ended with exit code $code"
  fi
}

echo "v02-u1: scan and plan in $region"
run_tool scan --profile "$profile" --region "$region" --rules "$rules" --lang en >"$work/scan.txt"
run_tool plan --profile "$profile" --region "$region" --rules "$rules" --lang en --out "$work/plan.json" >/dev/null
node test/verify/v02-u1-plan.js "$work/plan.json" "$stack"

echo "v02-u1: cloning the test-only repository and installing its app"
gh repo clone "$TAG_REFACTOR_TEST_REPO" "$work/clone" -- -q
npm --prefix "$work/clone" ci --ignore-scripts >/dev/null

echo "v02-u1: iac --pr"
code=0
"$tool" iac --plan "$work/plan.json" --dir "$work/clone" --pr --lang en >"$work/iac.out" || code=$?
cat "$work/iac.out"
if [ "$code" -ne 0 ] && [ "$code" -ne 2 ]; then
  fail "iac --pr ended with exit code $code"
fi
pr_url="$(grep '^https://github.com/' "$work/iac.out" | tail -n 1 || true)"
[ -n "$pr_url" ] || fail "no pull request was opened"

echo "v02-u1: checking the difference of $pr_url"
diff="$(gh pr diff "$pr_url")"
removed="$(printf '%s\n' "$diff" | grep -E '^-[^-]' || true)"
added="$(printf '%s\n' "$diff" | grep -E '^\+[^+]' || true)"
if [ "$(printf '%s\n' "$removed" | grep -c .)" -ne 1 ] || [ "$(printf '%s\n' "$added" | grep -c .)" -ne 1 ]; then
  printf '%s\n' "$diff" >&2
  fail "the difference is not one line"
fi
case "$removed$added" in
  *"'Env'"*"'Environment'"*) ;;
  *)
    printf '%s\n' "$diff" >&2
    fail "the line does not rename Env to Environment"
    ;;
esac

if [ "${TAG_REFACTOR_KEEP_PR:-}" = "1" ]; then
  echo "v02-u1: OK; the pull request $pr_url is kept: close it and delete its branch after looking"
else
  echo "v02-u1: OK; the pull request $pr_url is closed and its branch deleted"
fi
