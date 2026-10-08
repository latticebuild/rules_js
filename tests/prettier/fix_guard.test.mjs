import assert from "node:assert/strict";
import { spawnSync } from "node:child_process";
import { cpSync, mkdirSync, mkdtempSync, readdirSync, readFileSync, rmSync, statSync } from "node:fs";
import { join } from "node:path";
import { test } from "node:test";
import { fileURLToPath } from "node:url";

import { call, run as runOperation } from "effection";

import { joinTest, onCleanup, lifecycle } from "./lifecycle.mjs";

// A fix twin edits the workspace it was run from, so its guards are the only
// thing between a mistaken invocation and edits in the wrong place.
const fixtures = "testdata/prettier/checks";
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

test("prettier fixes only the invoking workspace and refuses a missing workspace", (t) =>
  joinTest(
    t,
    runOperation(function* () {
      return yield* lifecycle(t.signal, function* () {
        const workspace = yield* call(() => mkdtempSync(join(process.env.TEST_TMPDIR, "prettier-fix-")));
        yield* onCleanup(() => rmSync(workspace, { recursive: true, force: true }));
        const original = join(tree, fixtures, "prettier");
        const directory = join(workspace, fixtures, "prettier");
        const inputs = [".prettierrc.json", "violation.css"];
        const before = yield* call(() => {
          mkdirSync(directory, { recursive: true });
          for (const file of inputs) cpSync(join(original, file), join(directory, file));
          return snapshot(directory);
        });
        const env = { ...process.env };
        delete env.BUILD_WORKSPACE_DIRECTORY;
        const refused = yield* fix("prettier:violation_fix", env);
        assert.notEqual(refused.status, 0, refused.output);
        assert.match(refused.output, /BUILD_WORKSPACE_DIRECTORY is not set/v);
        assert.deepEqual(yield* call(() => snapshot(directory)), before);
        const fixed = yield* fix("prettier:violation_fix", { ...env, BUILD_WORKSPACE_DIRECTORY: workspace });
        assert.equal(fixed.status, 0, fixed.output);
        assert.deepEqual(yield* call(() => snapshot(directory)), {
          ".prettierrc.json": before[".prettierrc.json"],
          "violation.css": "a {\n  color: red;\n}\n",
        });
        for (const file of inputs) {
          assert.equal(yield* call(() => readFileSync(join(original, file), "utf8")), before[file]);
        }
      });
    }),
  ));
