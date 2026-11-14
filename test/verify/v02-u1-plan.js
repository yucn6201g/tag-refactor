"use strict";
// Checks the plan of the v0.2.0 U1 verification (test/verify/v02-u1.sh): it
// holds exactly one resource of the stack that renames Env to Environment.
// Usage: node v02-u1-plan.js <plan.json> <stack name>

const fs = require("fs");

const [file, stack] = process.argv.slice(2);
const plan = JSON.parse(fs.readFileSync(file, "utf8"));
const found = plan.resources.filter(
  (r) =>
    r.beforeTags["aws:cloudformation:stack-name"] === stack &&
    r.changes.some(
      (c) =>
        c.kind === "renameKey" &&
        c.fromKey === "Env" &&
        c.toKey === "Environment",
    ),
);
if (found.length !== 1) {
  process.stderr.write(
    "v02-u1: the plan holds " +
      found.length +
      " resources of " +
      stack +
      " that rename Env\n",
  );
  process.exit(1);
}
