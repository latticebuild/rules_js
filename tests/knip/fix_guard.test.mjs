import assert from "node:assert/strict";
import { spawnSync } from "node:child_process";
import { cpSync, mkdtempSync, readdirSync, readFileSync, rmSync, statSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { test } from "node:test";
import { fileURLToPath } from "node:url";

import { call, run as runOperation } from "effection";

import { joinTest, onCleanup, lifecycle } from "./lifecycle.mjs";

// A fix twin edits the workspace it was run from, so its guards are the only
// thing between a mistaken invocation and edits in the wrong place.
const fixtures = "testdata/knip/checks";
// This test's tree, which holds the fixtures at that path too.
const tree = fileURLToPath(new URL("../../", import.meta.url));
const suffix = process.platform === "win32" ? ".exe" : "";

// Runs a fix twin, a bound executable in this test's tree.
function* fix(target, env) {
  return yield* call(() => {
    const [pkg, name] = target.split(":");
    const executable = join(tree, fixtures, pkg, name + suffix);
    const result = spawnSync(executable, { encoding: "utf8", env });
    if (result.error) {
      throw result.error;
    }
    return { status: result.status, output: result.stdout + result.stderr };
  });
}

// Every file under a directory with its contents.
function snapshot(directory) {
  return Object.fromEntries(
    readdirSync(directory, { recursive: true, encoding: "utf8" })
      .filter((file) => statSync(join(directory, file)).isFile())
      .toSorted()
      .map((file) => [file, readFileSync(join(directory, file), "utf8")]),
  );
}

// The Knip guard fixture copied into a workspace of its own, where the twin
// runs, so that its edits, or their absence, show in that copy alone.
function* guardCopy() {
  const workspace = mkdtempSync(join(process.env.TEST_TMPDIR ?? tmpdir(), "knip-guard-"));
  yield* onCleanup(function* () {
    return yield* call(() => {
      return rmSync(workspace, { recursive: true, force: true });
    });
  });
  const pkg = join(workspace, fixtures, "knip/guard");
  cpSync(join(tree, fixtures, "knip/guard"), pkg, {
    recursive: true,
    dereference: true,
  });
  return { workspace, pkg };
}

// The managed Knip patch skips fixes when a configuration did not load;
// stock Knip applies them first and removes code and dependencies in use.
test("the Knip fix twin changes nothing when a configuration does not load", (t) =>
  joinTest(
    t,
    runOperation(function* () {
      return yield* lifecycle(t.signal, function* () {
        const { workspace, pkg } = yield* guardCopy();
        const before = snapshot(pkg);
        const { status, output } = yield* fix("knip/guard:guard_fix", {
          ...process.env,
          BUILD_WORKSPACE_DIRECTORY: workspace,
        });
        assert.notEqual(status, 0, output);
        assert.deepEqual(snapshot(pkg), before);
      });
    }),
  ));
