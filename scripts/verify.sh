#!/usr/bin/env bash
# U2 (scan of every region), U3 (rules suggest), U4 (plan formats and the
# saved plan), U5 (impact analysis) and U6 (apply) checkpoint verification.
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
# U5. Checks with the admin profile that the U5 stack (test/sandbox/u5-fixtures.yaml)
#    is complete and reads its six output ARNs, and that the role R2 of the
#    U1 stack allows iam:GetAccountAuthorizationDetails (the U5 IAM read).
#    Runs the U5 sandbox
#    test (internal/cli/integration_u5_test.go) with the read-only profile; a
#    skipped test, or anything but exactly one PASS line of
#    TestIntegrationU5ImpactSandbox, is a failure. Then runs the same plan
#    with --debug five times and prints, per operation, the number of
#    samples and the 95th percentile (L95), the largest L95 of the IAM and
#    of the resource policy operations, and the number of calls that were
#    retried (NFR1.19). It needs no queue or index, so it runs before the U2
#    wait too.
# 3. Checks the U2 stack and the Resource Explorer index of ap-northeast-1, and
#    records the type of every index.
# 4. Creates an SQS queue without tags in ap-northeast-1 and always deletes it
#    on exit (success, failure or interruption).
# 5. Waits, once a minute for up to 30 minutes, until Resource Explorer lists
#    the queue under tag:none.
# 6. Runs the U2 sandbox tests (internal/cli/integration_u2_test.go) with the
#    read-only profile; a skipped test is a failure.
# 7. Prints the 95th percentile of each call (from five scans and plans of
#    every region), the number of retried calls and the 10-minute estimate.
# U6. With the role R3 of the U6 stack (profile tag-refactor-apply), while
#    the queue of U2 still exists: checks the U6 stack, the profile and that
#    no lock is left; plans the check key tag-refactor-u6-check and checks
#    the range of each plan (scripts/u6check.go: only that key, no impact
#    but the undecided one the U1 fixture P2 gives every key, only
#    fixtures, every target present); adds and removes it five times with
#    apply --yes --allow-impact --debug (the plan check stands in for the
#    stop of --yes alone), the third run keeping its record in S3, and
#    judges every run by its record; checks that no resource and no IAM role
#    keeps the key and no lock is left; does the same once for the
#    never-tagged queue of U2 (its tags before must be empty); checks the
#    S3 record (encrypted, the owner's grant only, a missing key NotFound);
#    and prints the L95 of the tagging calls and the two estimates of
#    NFR5.15.
#
# The temporary folder of U3 and the queue of U2 are removed by one cleanup on
# every exit; it is defined before the first of them is made. While the U6
# check key may be on the fixtures, the cleanup first removes it with R3.
#
# Environment:
#   TAG_REFACTOR_SANDBOX_ACCOUNT_ID  the sandbox account ID (required)
#   TAG_REFACTOR_SANDBOX_PROFILE     admin profile (default tag-refactor-sandbox)
#   TAG_REFACTOR_IT_PROFILE          read-only profile (default tag-refactor-readonly)
#   TAG_REFACTOR_APPLY_PROFILE       profile of the role R3 (default tag-refactor-apply)
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
tool=""
# u6_added is set while the U6 check key may be on the fixtures: the
# cleanup then removes it with R3 before anything else (U6 cicd-pipeline.md).
u6_added=""
cleanup() {
  if [ -n "$u6_added" ] && [ -n "$tool" ] && [ -x "$tool" ]; then
    echo "verify: removing the U6 check key after a failure"
    u6_plan_file="$u3_dir/u6-cleanup-plan.json"
    rm -f -- "$u6_plan_file"
    if "$tool" plan --lang en --profile "$apply_profile" --rules test/sandbox/u6-remove-rules.yaml --out "$u6_plan_file" >/dev/null 2>&1 ||
      [ -s "$u6_plan_file" ]; then
      "$tool" apply --lang en --profile "$apply_profile" --plan "$u6_plan_file" --yes --allow-impact --record-to "$u3_dir/u6-cleanup" \
        >/dev/null 2>"$u3_dir/u6-cleanup-stderr" || true
      if grep -q 'another apply or rollback is running' "$u3_dir/u6-cleanup-stderr"; then
        sed 's/^/  /' "$u3_dir/u6-cleanup-stderr" >&2
        echo "verify: a lock stopped the cleanup; check that no apply runs, then remove it with --force-unlock" >&2
      fi
    fi
    left="$(u6_remaining 2>/dev/null || true)"
    if [ -n "$(printf '%s' "$left" | tr -d ' ')" ]; then
      echo "verify: the check key tag-refactor-u6-check is still on $left; remove it by hand" >&2
    fi
  fi
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
# --lang en: the checks below read the English comments and messages, which
# the language of the machine would otherwise change (U4 BR3.8, BR3.9).
run_tool rules suggest --lang en --profile "$readonly_profile" --region "$region" --output "$draft"
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
run_tool scan --lang en --profile "$readonly_profile" --region "$region" --rules "$draft"
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

# U5. The impact analysis against the U5 stack and the U1 stack.
u5_stack="tag-refactor-u5-fixtures"
u5_status="$(aws_admin cloudformation describe-stacks --stack-name "$u5_stack" --query 'Stacks[0].StackStatus' --output text 2>/dev/null)" ||
  fail "the stack $u5_stack does not exist; deploy test/sandbox/u5-fixtures.yaml as construction/u5-impact/infrastructure-design/cicd-pipeline.md describes"
case "$u5_status" in
  CREATE_COMPLETE | UPDATE_COMPLETE) ;;
  *) fail "the stack $u5_stack is $u5_status; deploy test/sandbox/u5-fixtures.yaml as cicd-pipeline.md describes" ;;
esac
u5_output() {
  local value
  value="$(aws_admin cloudformation describe-stacks --stack-name "$u5_stack" \
    --query "Stacks[0].Outputs[?OutputKey=='$1'].OutputValue" --output text)" || fail "cannot read the output $1 of $u5_stack"
  { [ -n "$value" ] && [ "$value" != "None" ]; } || fail "the stack $u5_stack has no output $1; deploy test/sandbox/u5-fixtures.yaml again"
  printf '%s' "$value"
}
u5_role="$(u5_output RoleArn)"
u5_bucket="$(u5_output BucketArn)"
u5_topic="$(u5_output TopicArn)"
u5_secret="$(u5_output SecretArn)"
# R2 must carry the U5 reads of docs/iam/readonly-policy.json.
r2_arn="$(aws_admin cloudformation describe-stacks --stack-name "$u1_stack" \
  --query "Stacks[0].Outputs[?OutputKey=='R2Arn'].OutputValue" --output text)" || fail "cannot read the output R2Arn of $u1_stack"
r2_policy="$(aws_admin iam get-role-policy --role-name "${r2_arn##*/}" --policy-name tag-refactor-readonly \
  --query PolicyDocument --output json)" || fail "cannot read the inline policy of R2"
printf '%s\n' "$r2_policy" | grep -q '"iam:GetAccountAuthorizationDetails"' ||
  fail "R2 does not allow iam:GetAccountAuthorizationDetails; update the role R2 of the U1 fixtures stack (test/sandbox/u1-fixtures.yaml) as cicd-pipeline.md describes"

echo "verify: running the U5 sandbox test (profile $readonly_profile)"
LAST_OUTPUT=""
if ! TAG_REFACTOR_IT_PROFILE="$readonly_profile" \
  TAG_REFACTOR_IT_U5_ROLE_ARN="$u5_role" TAG_REFACTOR_IT_U5_BUCKET_ARN="$u5_bucket" \
  TAG_REFACTOR_IT_U5_TOPIC_ARN="$u5_topic" TAG_REFACTOR_IT_U5_SECRET_ARN="$u5_secret" \
  run_go_test -mod=readonly -count=1 -tags integration -run '^TestIntegrationU5' -v ./internal/cli/...; then
  fail "the U5 sandbox test failed"
fi
if printf '%s\n' "$LAST_OUTPUT" | grep -q -- '--- SKIP'; then
  fail "the U5 sandbox test was skipped"
fi
u5_passed="$(printf '%s\n' "$LAST_OUTPUT" | grep -c -- '^--- PASS: TestIntegrationU5ImpactSandbox (' || true)"
[ "$u5_passed" -eq 1 ] || fail "$u5_passed PASS lines of TestIntegrationU5ImpactSandbox, want exactly 1"
printf '%s\n' "$LAST_OUTPUT" | grep -E 'RU5 was|U5 findings:' | sed 's/^ *//' || true

# The latency of each call of plan, from five runs with --debug (the
# account has few resources, so one run calls KMS, SNS and Secrets Manager a
# few times only). The --debug lines name the operation, the region, the
# time and the retries, never the documents (NFR8.22).
echo "verify: measuring the calls of plan with --debug (5 runs)"
debug_lines="$u3_dir/debug-lines"
: >"$debug_lines"
for run in 1 2 3 4 5; do
  run_tool plan --lang en --profile "$readonly_profile" --region "$region" --rules test/sandbox/u5-rules.yaml --format json --debug
  { [ "$TOOL_STATUS" -eq 2 ] || [ "$TOOL_STATUS" -eq 1 ]; } ||
    { sed 's/^/  /' "$u3_dir/stderr" >&2; fail "plan --debug ended with exit code $TOOL_STATUS (run $run)"; }
  grep -E '^debug: [a-z0-9-]+:[A-Za-z]+ ' "$u3_dir/stderr" >>"$debug_lines" || true
done
# Per operation: samples and L95 (the samples sorted by time, the one at
# 95 percent). Under 20 samples the L95 is the largest, which the line says.
awk '
  {
    op = $2; ms = $4; sub(/ms$/, "", ms)
    n[op]++; t[op, n[op]] = ms + 0
    retries = $5; sub(/^retries=/, "", retries)
    if (retries + 0 >= 1) retried++
  }
  END {
    for (op in n) {
      for (i = 2; i <= n[op]; i++) {
        v = t[op, i]
        for (j = i - 1; j >= 1 && t[op, j] > v; j--) t[op, j + 1] = t[op, j]
        t[op, j + 1] = v
      }
      k = int(0.95 * n[op] + 0.999999); if (k < 1) k = 1
      l95 = t[op, k]
      note = (n[op] < 20) ? " (few samples: the largest)" : ""
      printf "  L95 %s: %d ms, %d samples%s\n", op, l95, n[op], note
      if (op ~ /^iam:/ && l95 > iam) iam = l95
      if (op ~ /^(s3|kms|sqs|sns|secretsmanager):/ && l95 > res) res = l95
    }
    printf "  L95(IAM): %d ms\n", iam
    printf "  L95(resource policies): %d ms\n", res
    printf "  retried calls: %d\n", retried
  }
' "$debug_lines" | sort
echo "verify: apply these values to the 10-minute estimate of logical-components.md at the checkpoint"

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
printf '%s\n' "$LAST_OUTPUT" | grep -E 'L95 |NFR1\.8 (estimate|retried)|took ' | sed 's/^ *//'

# U6. apply with the role R3 against the fixtures (U6 cicd-pipeline.md),
# while the never-tagged queue of U2 still exists.
# U6 step 1: the stacks, the profile of R3 and no lock left behind.
u6_stack="tag-refactor-u6-fixtures"
u6_status="$(aws_admin cloudformation describe-stacks --stack-name "$u6_stack" --query 'Stacks[0].StackStatus' --output text 2>/dev/null)" ||
  fail "the stack $u6_stack does not exist; deploy test/sandbox/u6-fixtures.yaml as construction/u6-apply/infrastructure-design/cicd-pipeline.md describes"
case "$u6_status" in
  CREATE_COMPLETE | UPDATE_COMPLETE) ;;
  *) fail "the stack $u6_stack is $u6_status; deploy test/sandbox/u6-fixtures.yaml as cicd-pipeline.md describes" ;;
esac
u6_output() {
  local value
  value="$(aws_admin cloudformation describe-stacks --stack-name "$u6_stack" \
    --query "Stacks[0].Outputs[?OutputKey=='$1'].OutputValue" --output text)" || fail "cannot read the output $1 of $u6_stack"
  { [ -n "$value" ] && [ "$value" != "None" ]; } || fail "the stack $u6_stack has no output $1; deploy test/sandbox/u6-fixtures.yaml again"
  printf '%s' "$value"
}
r3_arn="$(u6_output ApplyRoleArn)"
u6_sg="$(u6_output SecurityGroupArn)"
u6_bucket="$(u6_output RecordBucket)"
qu5_url="$(aws_admin cloudformation describe-stack-resource --stack-name "$u5_stack" --logical-resource-id QU5 \
  --query StackResourceDetail.PhysicalResourceId --output text)" || fail "cannot read QU5 of $u5_stack"
qu5_arn="$(aws_admin sqs get-queue-attributes --queue-url "$qu5_url" --attribute-names QueueArn --query Attributes.QueueArn --output text)" ||
  fail "cannot read the ARN of QU5"
u6_targets=("$u5_bucket" "$qu5_arn" "$u5_topic" "$u5_secret" "$u5_role" "$u6_sg")
apply_profile="${TAG_REFACTOR_APPLY_PROFILE:-tag-refactor-apply}"
apply_caller="$(aws --profile "$apply_profile" --region "$region" sts get-caller-identity --query Arn --output text)" ||
  fail "cannot use the profile $apply_profile; add a profile that assumes $r3_arn (README.md, apply)"
case "$apply_caller" in
  *":assumed-role/${r3_arn##*/}/"*) ;;
  *) fail "the profile $apply_profile is not the role ${r3_arn##*/} of $u6_stack" ;;
esac
local_lock="$HOME/.tag-refactor/runs/$account/.lock"
if [ -e "$local_lock" ]; then
  sed 's/^/  /' "$local_lock" >&2
  fail "a lock is left at $local_lock; check that no apply runs, then remove it with apply --force-unlock (never automatically)"
fi
u6_dir="$u3_dir/u6"
mkdir -p "$u6_dir"
u6_debug="$u6_dir/debug-lines"
: >"$u6_debug"
u6_plans=0

# u6_plan makes the plan of a rule file with R3 in every region and checks
# its range (U6 step 2). It sets U6_PLAN to the saved plan.
u6_plan() {
  local rules="$1" mode="$2"
  shift 2
  u6_plans=$((u6_plans + 1))
  U6_PLAN="$u6_dir/plan-$u6_plans.json"
  run_tool plan --lang en --profile "$apply_profile" --rules "$rules" --format json --out "$U6_PLAN"
  { [ "$TOOL_STATUS" -eq 0 ] || [ "$TOOL_STATUS" -eq 2 ]; } ||
    { sed 's/^/  /' "$u3_dir/stderr" >&2; fail "plan --rules $rules ended with exit code $TOOL_STATUS"; }
  go run scripts/u6check.go plan "$U6_PLAN" "$mode" "$queue_arn" "$@" || fail "the plan of $rules is out of range; nothing was changed"
}

# u6_apply applies the plan with R3, --yes and --allow-impact, and sets
# U6_RUN to the run ID. --allow-impact is needed for the undecided finding
# that the U1 fixture P2 gives every key; u6_plan has already refused any
# other finding. A run that made no record fails.
u6_apply() {
  local record_to="$1"
  run_tool apply --lang en --profile "$apply_profile" --plan "$U6_PLAN" --yes --allow-impact --debug --record-to "$record_to"
  grep -E '^debug: tag:[A-Za-z]+ ' "$u3_dir/stderr" >>"$u6_debug" || true
  U6_RUN="$(sed -n 's/.*tag-refactor rollback --run \([0-9TZ]*-[0-9a-f]*\).*/\1/p' "$u3_dir/stderr" | head -n 1)"
  if [ -z "$U6_RUN" ] || { [ "$TOOL_STATUS" -ne 0 ] && [ "$TOOL_STATUS" -ne 2 ]; }; then
    sed 's/^/  /' "$u3_dir/stderr" >&2
    if grep -q 'another apply or rollback is running' "$u3_dir/stderr"; then
      fail "apply stopped at a lock; check that no apply runs, then remove it with --force-unlock"
    fi
    fail "apply ended with exit code $TOOL_STATUS without a complete run"
  fi
}

# u6_record copies the record of the last run to U6_RECORD.
u6_record() {
  U6_RECORD="$u6_dir/$U6_RUN.json"
  case "$1" in
    s3://*) aws_admin s3api get-object --bucket "$u6_bucket" --key "runs/$account/$U6_RUN.json" "$U6_RECORD" >/dev/null ||
      fail "the record $U6_RUN is not in s3://$u6_bucket/runs/$account/" ;;
    *) cp "$1/$account/$U6_RUN.json" "$U6_RECORD" || fail "the record $U6_RUN is not in $1/$account/" ;;
  esac
}

# U6 step 3: add and remove the check key five times; the third run keeps
# its record in S3. The flag tells the cleanup to remove the key.
s3_run=""
for round in 1 2 3 4 5; do
  record_to="$u6_dir/records"
  [ "$round" -eq 3 ] && record_to="s3://$u6_bucket/runs"
  u6_plan test/sandbox/u6-add-rules.yaml add "${u6_targets[@]}"
  add_plan="$U6_PLAN"
  u6_added=yes
  u6_apply "$record_to"
  u6_record "$record_to"
  go run scripts/u6check.go record "$U6_RECORD" "$add_plan" "${u6_targets[@]}" || fail "the add of round $round is not complete"
  [ "$round" -eq 3 ] && s3_run="$U6_RUN"
  u6_plan test/sandbox/u6-remove-rules.yaml remove "${u6_targets[@]}"
  remove_plan="$U6_PLAN"
  u6_apply "$record_to"
  u6_record "$record_to"
  go run scripts/u6check.go record "$U6_RECORD" "$remove_plan" "${u6_targets[@]}" || fail "the removal of round $round is not complete"
  if grep -q '"status": "skipped"' "$U6_RECORD"; then
    fail "the removal of round $round skipped resources"
  fi
done

# U6 step 4: nothing keeps the check key (eventually consistent: six reads
# 10 seconds apart), and no lock is left.
u6_remaining() {
  local r left=""
  for r in ap-northeast-1 us-east-1; do
    left="$left $(aws --profile "$admin" --region "$r" resourcegroupstaggingapi get-resources \
      --tag-filters Key=tag-refactor-u6-check --query 'ResourceTagMappingList[].ResourceARN' --output text)"
  done
  for role in "${u5_role##*/}" "${r3_arn##*/}" "${r2_arn##*/}"; do
    if aws_admin iam list-role-tags --role-name "$role" --query 'Tags[].Key' --output text | tr '\t' '\n' | grep -qx tag-refactor-u6-check; then
      left="$left role/$role"
    fi
  done
  printf '%s' "$left" | tr -s ' \t' ' ' | sed 's/^ //; s/ $//; s/None//g'
}
u6_check_removed() {
  local attempt left
  for attempt in 1 2 3 4 5 6; do
    left="$(u6_remaining)"
    [ -z "$(printf '%s' "$left" | tr -d ' ')" ] && return 0
    echo "verify: the check key is still on $left (read $attempt of 6)"
    sleep 10
  done
  fail "the check key is left on $left"
}
u6_check_removed
[ ! -e "$local_lock" ] || fail "the local lock $local_lock is left"
if aws_admin s3api head-object --bucket "$u6_bucket" --key "runs/$account/.lock" >/dev/null 2>&1; then
  fail "the S3 lock s3://$u6_bucket/runs/$account/.lock is left"
fi
u6_added=""

# U6 step 5: the never-tagged queue of U2, with its own rules.
if aws_admin resourcegroupstaggingapi get-resources --resource-arn-list "$queue_arn" \
  --query 'ResourceTagMappingList[].ResourceARN' --output text | grep -qF "$queue_arn"; then
  echo "verify: the tagging API returns the never-tagged queue when it is named"
else
  echo "verify: the tagging API does not return the never-tagged queue when it is named (apply treats it as having no tags)"
fi
u6_plan test/sandbox/u6-untagged-rules.yaml untagged "$queue_arn"
untagged_plan="$U6_PLAN"
u6_added=yes
u6_apply "$u6_dir/records"
u6_record "$u6_dir/records"
go run scripts/u6check.go record "$U6_RECORD" "$untagged_plan" "$queue_arn" || fail "the add on the never-tagged queue is not complete"
go run scripts/u6check.go entry "$U6_RECORD" "$queue_arn" || fail "the never-tagged queue was not written from empty tags"
u6_plan test/sandbox/u6-remove-rules.yaml remove "$queue_arn"
remove_plan="$U6_PLAN"
u6_apply "$u6_dir/records"
u6_record "$u6_dir/records"
go run scripts/u6check.go record "$U6_RECORD" "$remove_plan" "$queue_arn" || fail "the removal on the never-tagged queue is not complete"
u6_check_removed
u6_added=""

# U6 step 6: the record in S3 is encrypted, has no grant but the owner's,
# and R3 reads a missing key as NotFound.
sse="$(aws_admin s3api head-object --bucket "$u6_bucket" --key "runs/$account/$s3_run.json" --query ServerSideEncryption --output text)" ||
  fail "the S3 record $s3_run is missing"
{ [ -n "$sse" ] && [ "$sse" != "None" ]; } || fail "the S3 record $s3_run is not encrypted"
grants="$(aws_admin s3api get-object-acl --bucket "$u6_bucket" --key "runs/$account/$s3_run.json" --query 'length(Grants)' --output text)" ||
  fail "cannot read the ACL of the S3 record"
[ "$grants" = "1" ] || fail "the S3 record has $grants grants; only the owner may have one"
missing_err="$(aws --profile "$apply_profile" --region "$region" s3api get-object --bucket "$u6_bucket" \
  --key "runs/$account/no-such-run.json" "$u6_dir/missing" 2>&1 >/dev/null || true)"
case "$missing_err" in
  *NoSuchKey* | *"Not Found"*) echo "verify: a missing record reads as NotFound with R3" ;;
  *AccessDenied*) fail "a missing record reads as AccessDenied with R3; fix the s3:ListBucket statement of docs/iam/apply-policy.json" ;;
  *) fail "reading a missing record with R3 gave: $missing_err" ;;
esac
echo "verify: the S3 record $s3_run is encrypted ($sse) with the owner's grant only"

# U6 step 7: L95 of the tagging calls of the ten applies and the estimates
# of NFR5.15 for the skewed region (2,000 resources): usual changes 20 first
# reads, 50 reads again and back, 105 writes of each kind; values that
# differ on every resource 20, 50 and 2,000 of each kind.
awk '
  {
    op = $2; ms = $4; sub(/ms$/, "", ms)
    n[op]++; t[op, n[op]] = ms + 0
    retries = $5; sub(/^retries=/, "", retries)
    if (retries + 0 >= 1) retried++
  }
  END {
    for (op in n) {
      for (i = 2; i <= n[op]; i++) {
        v = t[op, i]
        for (j = i - 1; j >= 1 && t[op, j] > v; j--) t[op, j + 1] = t[op, j]
        t[op, j + 1] = v
      }
      k = int(0.95 * n[op] + 0.999999); if (k < 1) k = 1
      l95[op] = t[op, k]
      note = (n[op] < 20) ? " (few samples: the largest)" : ""
      printf "  L95 %s: %d ms, %d samples%s\n", op, l95[op], n[op], note
    }
    g = l95["tag:GetResources"]; tg = l95["tag:TagResources"]; ut = l95["tag:UntagResources"]
    usual = ((20 + 50) * g + 105 * tg + 105 * ut) * 1.2 / 60000
    scattered = ((20 + 50) * g + 2000 * tg + 2000 * ut) * 1.2 / 60000
    printf "  retried calls: %d\n", retried
    printf "  NFR5.15 estimate, usual changes: %.1f minutes (limit 30)\n", usual
    printf "  NFR5.15 estimate, values differing on every resource: %.1f minutes (recorded, not judged)\n", scattered
  }
' "$u6_debug"
echo "verify: record these values and the limits of the small samples at the U6 checkpoint"
echo "verify: OK"
