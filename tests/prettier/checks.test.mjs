import assert from "node:assert/strict";
import { spawnSync } from "node:child_process";
import { join } from "node:path";
import { test } from "node:test";

import { call, run as runOperation } from "effection";

import { joinTest, lifecycle } from "./lifecycle.mjs";

// Every real check test passes, so only these fixtures notice a check that
// stopped refusing: each runs its fixture test, a bound executable in this
// test's tree, and expects a failure that names the violation.
const fixtures = join(import.meta.dirname, "../../testdata/prettier/checks");
const suffix = process.platform === "win32" ? ".exe" : "";

// Bazel's report variables belong to this test, not to the fixtures it runs.
const env = { ...process.env };
for (const name of ["COVERAGE_DIR", "NODE_V8_COVERAGE", "XML_OUTPUT_FILE"]) {
  delete env[name];
}

function* check(target) {
  return yield* call(() => {
    const [pkg, name] = target.split(":");
    const result = spawnSync(join(fixtures, pkg, name + suffix), { encoding: "utf8", env });
    if (result.error) {
      throw result.error;
    }
    return { status: result.status, output: result.stdout + result.stderr };
  });
}

for (const [target, diagnostics] of [
  ["prettier:violation_test", [/violation\.css/v]],
]) {
  test(`${target} rejects its fixture`, (t) =>
    joinTest(
      t,
      runOperation(function* () {
        return yield* lifecycle(t.signal, function* () {
          const { status, output } = yield* check(target);
          assert.notEqual(status, 0, output);
          for (const diagnostic of diagnostics) {
            assert.match(output, diagnostic);
          }
        });
      }),
    ));
}
