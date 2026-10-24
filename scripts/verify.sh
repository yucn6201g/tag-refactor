#!/usr/bin/env bash
# U2 (scan of every region), U3 (rules suggest) and U4 (plan formats and the
# saved plan) checkpoint verification.
# It calls the sandbox AWS account, so it only runs locally, never in CI.
#
# 1. Runs scripts/verify-u1.sh (every test, and the U1 sandbox test).
# 2. Checks with the admin profile that the account is the sandbox account.
# U3. Checks that the U1 stack exists, builds tag-refactor, runs
#    "rules suggest --region ap-northeast-1 --output <file>" with the
#    read-only profile (exit code 0, nothing on standard output), checks the
#    draft (first comment, env and Env in one group), and reads it back with
#    "scan --rules". It uses no queue and no Resource Explorer index, so it
#    runs before the U2 wait.
# U4. Runs the U4 sandbox test (internal/cli/integration_u4_test.go) with
#    the read-only profile: plan in the four formats and with --out against
#    the U1 stack, the counts of every format and of the saved plan agree,
#    and the saved plan reads back. A skipped test, or anything but exactly
#    one PASS line of TestIntegrationU4PlanFormatsAndSavedPlan, is a failure.
#    It needs no queue or index either, so it also runs before the U2 wait.
# 3. Checks the U2 stack and the Resource Explorer index of ap-northeast-1, and
#    records the type of every index.
# 4. Creates an SQS queue without tags in ap-northeast-1 and always deletes it
#    on exit (success, failure or interruption).
# 5. Waits, once a minute for up to 30 minutes, until Resource Explorer lists
#    the queue under tag:none.
# 6. Runs the U2 sandbox tests (internal/cli/integration_u2_test.go) with the
#    read-only profile; a skipped test is a failure.
# 7. Prints the 95th percentile of each call and the 10-minute estimate.
#
# The temporary folder of U3 and the queue of U2 are removed by one cleanup on
# every exit; it is defined before the first of them is made.
#
# Environment:
#   TAG_REFACTOR_SANDBOX_ACCOUNT_ID  the sandbox account ID (required)
#   TAG_REFACTOR_SANDBOX_PROFILE     admin profile (default tag-refactor-sandbox)
#   TAG_REFACTOR_IT_PROFILE          read-only profile (default tag-refactor-readonly)
#
# It prints no credentials: from the AWS CLI it reads ARNs, names and states
# only, through --query.
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

region="ap-northeast-1"
stack="tag-refactor-u2-fixtures"
admin="${TAG_REFACTOR_SANDBOX_PROFILE:-tag-refactor-sandbox}"
readonly_profile="${TAG_REFACTOR_IT_PROFILE:-tag-refactor-readonly}"
wait_minutes=30

fail() {
  echo "verify: $*" >&2
  exit 1
}

# run_go_test runs `go test` and prints its output. Windows Application Control
# sometimes blocks a freshly built test binary; only that case is retried (up
# to 3 attempts), as in verify-u1.sh. A real test failure is never retried.
run_go_test() {
  local attempt status out
  for attempt in 1 2 3; do
    status=0
    out="$(go test "$@" 2>&1)" || status=$?
    printf '%s\n' "$out"
    LAST_OUTPUT="$out"
    if [ "$status" -eq 0 ]; then
      return 0
    fi
    if ! printf '%s\n' "$out" | grep -q 'An Application Control policy has blocked this file'; then
      return "$status"
    fi
    echo "verify: Application Control blocked a test binary; retrying ($attempt of 3)" >&2
  done
  return "$status"
}

aws_admin() {
  aws --profile "$admin" --region "$region" "$@"
}

# One cleanup for every exit (success, failure or interruption). It does
# nothing for what was not made yet: the queue name and the temporary folder
# are empty until then. Only this one EXIT trap is defined, so neither part
# can replace the other.
queue_name=""
queue_url=""
u3_dir=""
cleanup() {
  if [ -n "$u3_dir" ] && [ -d "$u3_dir" ]; then
    rm -rf -- "$u3_dir"
  fi
  if [ -n "$queue_url" ]; then
    if aws_admin sqs delete-queue --queue-url "$queue_url" >/dev/null 2>&1; then
      echo "verify: deleted the queue $queue_name"
    else
      echo "verify: could not delete the queue $queue_name in $region; delete it by hand" >&2
    fi
  fi
}
trap cleanup EXIT
trap 'exit 130' INT TERM

# run_tool runs the built tag-refactor and keeps its exit code, standard
# output and standard error. Application Control may block a freshly built
# binary; only that case is retried (up to 3 attempts).
run_tool() {
  local attempt
  for attempt in 1 2 3; do
    TOOL_STATUS=0
    "$tool" "$@" >"$u3_dir/stdout" 2>"$u3_dir/stderr" || TOOL_STATUS=$?
    if ! grep -q 'An Application Control policy has blocked this file' "$u3_dir/stderr"; then
      return 0
    fi
    echo "verify: Application Control blocked tag-refactor; retrying ($attempt of 3)" >&2
  done
  fail "Application Control blocked tag-refactor 3 times; run the verification again"
}

# 1. The U1 verification: every test and the U1 sandbox test.
echo "verify: running scripts/verify-u1.sh"
TAG_REFACTOR_IT_PROFILE="$readonly_profile" bash scripts/verify-u1.sh || fail "scripts/verify-u1.sh failed"

# 2. The account must be the sandbox account.
[ -n "${TAG_REFACTOR_SANDBOX_ACCOUNT_ID:-}" ] || fail "set TAG_REFACTOR_SANDBOX_ACCOUNT_ID to the sandbox account ID"
account="$(aws_admin sts get-caller-identity --query Account --output text)" ||
  fail "cannot identify the account with the profile $admin; run \"aws sso login --profile $admin\""
[ "$account" = "$TAG_REFACTOR_SANDBOX_ACCOUNT_ID" ] ||
  fail "the profile $admin is account $account, not the sandbox account $TAG_REFACTOR_SANDBOX_ACCOUNT_ID; stopping"

# U3. rules suggest against the U1 fixtures (their env and Env tags).
u1_stack="tag-refactor-u1-fixtures"
u1_status="$(aws_admin cloudformation describe-stacks --stack-name "$u1_stack" --query 'Stacks[0].StackStatus' --output text 2>/dev/null)" ||
  fail "the stack $u1_stack does not exist; deploy the U1 fixtures stack (test/sandbox/u1-fixtures.yaml) as the U1 infrastructure-specification.md describes"
case "$u1_status" in
  CREATE_COMPLETE | UPDATE_COMPLETE) ;;
  *) fail "the stack $u1_stack is $u1_status; deploy the U1 fixtures stack as the U1 infrastructure-specification.md describes" ;;
esac
# rules suggest --output never overwrites a file, so the draft goes to a new
# name inside a temporary folder rather than to a file mktemp creates.
u3_dir="$(mktemp -d)"
draft="$u3_dir/tagrules.yaml"
tool="$u3_dir/tag-refactor$(go env GOEXE)"
echo "verify: building tag-refactor for the U3 check"
go build -mod=readonly -o "$tool" ./cmd/tag-refactor || fail "cannot build tag-refactor"

echo "verify: running rules suggest (profile $readonly_profile)"
run_tool rules suggest --profile "$readonly_profile" --region "$region" --output "$draft"
[ "$TOOL_STATUS" -eq 0 ] || { sed 's/^/  /' "$u3_dir/stderr" >&2; fail "rules suggest ended with exit code $TOOL_STATUS, not 0"; }
[ ! -s "$u3_dir/stdout" ] || fail "rules suggest printed to standard output although --output was given"
[ -s "$draft" ] || fail "rules suggest did not write $draft"
head -n 1 "$draft" | grep -q '^# generated by tag-refactor ' || fail "the draft does not start with \"# generated by tag-refactor\""
# env and Env are one group: a key line and the aliases line under it hold
# both (which spelling is canonical depends on the counts).
group="$(awk 'prev ~ /^  "/ && $0 ~ /^    aliases: / && (prev ~ /"env"/ || $0 ~ /"env"/) && (prev ~ /"Env"/ || $0 ~ /"Env"/) { print prev; print } { prev = $0 }' "$draft")"
if [ -z "$group" ]; then
  sed 's/^/  /' "$draft" >&2
  fail "env and Env are not in one group of the draft"
fi

echo "verify: reading the draft back with scan --rules"
run_tool scan --profile "$readonly_profile" --region "$region" --rules "$draft"
# 0 (no issues) and 2 (issues to review) both mean the draft was read.
if [ "$TOOL_STATUS" -ne 0 ] && [ "$TOOL_STATUS" -ne 2 ] || grep -q '^Error:' "$u3_dir/stderr"; then
  sed 's/^/  /' "$u3_dir/stderr" >&2
  fail "scan could not read the draft of rules suggest"
fi
draft_keys="$(grep -cE '^  "' "$draft" || true)"
echo "verify: U3 draft has $draft_keys keys; the env group:"
printf '%s\n' "$group" | sed 's/^/  /'

# U4. plan in the four formats and the saved plan, against the U1 stack
# (checked above). The test reads the sandbox account ID from
# TAG_REFACTOR_SANDBOX_ACCOUNT_ID.
echo "verify: running the U4 sandbox test (profile $readonly_profile)"
LAST_OUTPUT=""
if ! TAG_REFACTOR_IT_PROFILE="$readonly_profile" \
  run_go_test -mod=readonly -count=1 -tags integration -run '^TestIntegrationU4' -v ./internal/cli/...; then
  fail "the U4 sandbox test failed"
fi
if printf '%s\n' "$LAST_OUTPUT" | grep -q -- '--- SKIP'; then
  fail "the U4 sandbox test was skipped"
fi
# Only the line of the test itself (its name, a space and a parenthesis)
# counts, so a -run pattern that matches nothing cannot pass.
u4_passed="$(printf '%s\n' "$LAST_OUTPUT" | grep -c -- '^--- PASS: TestIntegrationU4PlanFormatsAndSavedPlan (' || true)"
[ "$u4_passed" -eq 1 ] || fail "$u4_passed PASS lines of TestIntegrationU4PlanFormatsAndSavedPlan, want exactly 1"
printf '%s\n' "$LAST_OUTPUT" | grep -E 'U4 counts:' | sed 's/^ *//' || true

# 3. The U2 stack and the Resource Explorer index of ap-northeast-1.
status="$(aws_admin cloudformation describe-stacks --stack-name "$stack" --query 'Stacks[0].StackStatus' --output text 2>/dev/null)" ||
  fail "the stack $stack does not exist; deploy test/sandbox/u2-fixtures.yaml as README.md describes"
case "$status" in
  CREATE_COMPLETE | UPDATE_COMPLETE) ;;
  *) fail "the stack $stack is $status; it must be CREATE_COMPLETE or UPDATE_COMPLETE" ;;
esac
indexes="$(aws_admin resource-explorer-2 list-indexes --query 'Indexes[].[Region,Type]' --output text)" ||
  fail "cannot list the Resource Explorer indexes"
echo "verify: Resource Explorer indexes (region, type):"
printf '%s\n' "$indexes" | sed 's/^/  /'
if printf '%s\n' "$indexes" | grep -q 'AGGREGATOR'; then
  fail "the account has an aggregator index; the U2 checkpoint expects local indexes only (the aggregator path is covered by the unit tests)"
fi
printf '%s\n' "$indexes" | grep -qE "^$region[[:space:]]+LOCAL$" ||
  fail "$region has no local Resource Explorer index; deploy the U2 stack with CreateIndex=true"
index_state="$(aws_admin resource-explorer-2 get-index --query State --output text)" || fail "cannot read the index of $region"
[ "$index_state" = "ACTIVE" ] || fail "the index of $region is $index_state, not ACTIVE; try again later"
view="$(aws_admin resource-explorer-2 get-default-view --query ViewArn --output text)" || fail "cannot read the default view of $region"
{ [ -n "$view" ] && [ "$view" != "None" ]; } || fail "$region has no default view; deploy the U2 stack with CreateDefaultView=true"
filter="$(aws_admin resource-explorer-2 get-view --view-arn "$view" --query 'View.Filters.FilterString' --output text)" ||
  fail "cannot read the default view of $region"
{ [ -z "$filter" ] || [ "$filter" = "None" ]; } ||
  fail "the default view of $region has a filter; the expected results assume none, so a person must decide"
indexed_regions="$(printf '%s\n' "$indexes" | awk 'NF { print $1 }' | paste -sd, -)"

# 4. A queue without tags, deleted on every exit.
suffix="$(od -An -N4 -tx1 /dev/urandom | tr -d ' \n')"
queue_name="tag-refactor-u2-untagged-$suffix"
queue_url="$(aws_admin sqs create-queue --queue-name "$queue_name" --query QueueUrl --output text)" ||
  fail "cannot create the queue $queue_name"
queue_arn="$(aws_admin sqs get-queue-attributes --queue-url "$queue_url" --attribute-names QueueArn --query Attributes.QueueArn --output text)" ||
  fail "cannot read the ARN of the queue $queue_name"
echo "verify: created the queue without tags $queue_arn"

# 5. Wait until Resource Explorer lists the queue under tag:none.
found=""
for minute in $(seq 1 "$wait_minutes"); do
  listed="$(aws_admin resource-explorer-2 list-resources --filters 'FilterString=resourcetype:sqs:queue tag:none' \
    --query 'Resources[].Arn' --output text 2>/dev/null || true)"
  if printf '%s\n' "$listed" | tr '\t' '\n' | grep -qxF "$queue_arn"; then
    found=yes
    break
  fi
  echo "verify: the queue is not in Resource Explorer yet (minute $minute of $wait_minutes)"
  sleep 60
done
[ -n "$found" ] ||
  fail "the queue did not appear in Resource Explorer within $wait_minutes minutes; filling the index can take hours, so run this script again later"

# 6. The U2 sandbox tests with the read-only profile.
echo "verify: running the U2 sandbox tests (profile $readonly_profile)"
LAST_OUTPUT=""
if ! TAG_REFACTOR_IT_PROFILE="$readonly_profile" TAG_REFACTOR_IT_UNTAGGED_ARN="$queue_arn" \
  TAG_REFACTOR_IT_INDEXED_REGIONS="$indexed_regions" \
  run_go_test -mod=readonly -count=1 -tags integration -run '^TestIntegrationU2' -v ./internal/cli/...; then
  fail "the U2 sandbox tests failed"
fi
if printf '%s\n' "$LAST_OUTPUT" | grep -q -- '--- SKIP: TestIntegrationU2'; then
  fail "a U2 sandbox test was skipped"
fi
passed="$(printf '%s\n' "$LAST_OUTPUT" | grep -c -- '--- PASS: TestIntegrationU2' || true)"
[ "$passed" -eq 5 ] || fail "$passed of the 5 U2 sandbox tests passed"

# 7. The 95th percentile of each call and the estimate (NFR1.8).
echo "verify: call latencies and the 10-minute estimate"
printf '%s\n' "$LAST_OUTPUT" | grep -E 'L95 |NFR1\.8 estimate|took ' | sed 's/^ *//'
echo "verify: OK"
