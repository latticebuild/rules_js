import assert from "node:assert/strict";
import { spawn, spawnSync } from "node:child_process";
import { mkdirSync, mkdtempSync, readFileSync, realpathSync, rmSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { test } from "node:test";
import { fileURLToPath } from "node:url";

import { call, run as runOperation, race, sleep } from "effection";

import { joinTest, disposeAll, lifecycle, onCleanup } from "./lifecycle.mjs";

// The native adapters supply runtime behavior that bound cannot configure.
// A real executable shows a broken step only when its tool happens to depend
// on the broken part, so these cases pin each step, including its refusals.
// node:test's reports and filters are proven end to end by reports_test.

const suffix = process.platform === "win32" ? ".exe" : "";
const executable = (name) =>
  fileURLToPath(new URL(`../../js/private/${name === "run-fix" ? "" : "vite/"}tools/${name}/${name}_/${name}${suffix}`, import.meta.url));
const runnerProbe = fileURLToPath(new URL("../../testdata/vite/runners/probe.mjs", import.meta.url));
const scratch = () => process.env.TEST_TMPDIR ?? tmpdir();

// This test's own Bazel variables, which the adapter under test must not see
// unless a case sets them.
const baseEnv = { ...process.env };
for (const name of [
  "COVERAGE_DIR",
  "NODE_V8_COVERAGE",
  "TESTBRIDGE_TEST_ONLY",
  "TEST_SHARD_INDEX",
  "TEST_SHARD_STATUS_FILE",
  "TEST_TOTAL_SHARDS",
  "TEST_UNDECLARED_OUTPUTS_DIR",
  "XML_OUTPUT_FILE",
]) {
  delete baseEnv[name];
}

// A directory removed after the test.
function* directory(t, prefix) {
  const created = mkdtempSync(join(scratch(), prefix));
  yield* onCleanup(function* () {
    return yield* call(() => {
      return rmSync(created, { recursive: true, force: true });
    });
  });
  return created;
}

// Runs the adapter with `args` from `cwd`. A variable set to undefined is
// removed from the environment.
function* run(args, { env = {}, cwd, cli = "run-fix" } = {}) {
  return yield* call(() => {
    const environment = { ...baseEnv, ...env };
    for (const [name, value] of Object.entries(environment)) {
      if (value === undefined) {
        delete environment[name];
      }
    }
    const child = spawnSync(executable(cli), ["--node", process.execPath, ...args], {
      encoding: "utf8",
      env: environment,
      cwd,
    });
    if (child.error) {
      throw child.error;
    }
    return { status: child.status, stdout: child.stdout, stderr: child.stderr };
  });
}

// A workspace holding the package javascript/app, and tools outside it: an ES
// module and a CommonJS script, as Oxfmt's and Prettier's are. Each records
// where it runs and the arguments it sees, then exits with EXIT.
function* fixture(t) {
  const workspace = yield* directory(t, "workspace-");
  mkdirSync(join(workspace, "javascript/app"), { recursive: true });
  const tools = yield* directory(t, "tools-");
  const report = `fs.writeFileSync(process.env.RESULT, JSON.stringify({ cwd: process.cwd(), argv: process.argv.slice(1) }));
process.exitCode = Number(process.env.EXIT ?? 0);
`;
  writeFileSync(join(tools, "tool.mjs"), `import fs from "node:fs";\n${report}`);
  writeFileSync(join(tools, "tool.cjs"), `const fs = require("node:fs");\n${report}`);
  return { workspace, tools, result: join(tools, "result.json") };
}

// Runs a fix twin's command line: the package to enter, then the tool.
function* fix(probe, workspacePackage, { tool = "tool.mjs", env = {} } = {}) {
  const script = join(probe.tools, tool);
  const result = yield* run(["--workspace", workspacePackage, "--", script, "--write", "."], {
    env: { RESULT: probe.result, ...env },
    cwd: probe.tools,
  });
  let reported;
  try {
    reported = JSON.parse(readFileSync(probe.result, "utf8"));
  } catch {
    // The tool never ran.
  }
  return { ...result, script, reported };
}

test("a fix twin runs its tool in its package of the invoking workspace", (t) =>
  joinTest(
    t,
    runOperation(function* () {
      return yield* lifecycle(t.signal, function* () {
        const probe = yield* fixture(t);
        const { status, stderr, script, reported } = yield* fix(probe, "javascript/app", {
          env: { BUILD_WORKSPACE_DIRECTORY: probe.workspace },
        });
        assert.equal(status, 0, stderr);
        assert.equal(
          realpathSync(reported.cwd),
          realpathSync(join(probe.workspace, "javascript/app")),
        );
        // The tool sees the arguments it would if node had started it directly.
        assert.deepEqual(reported.argv, [script, "--write", "."]);
      });
    }),
  ));

test("a fix twin of the root package runs in the workspace itself", (t) =>
  joinTest(
    t,
    runOperation(function* () {
      return yield* lifecycle(t.signal, function* () {
        const probe = yield* fixture(t);
        const { status, stderr, reported } = yield* fix(probe, ".", {
          env: { BUILD_WORKSPACE_DIRECTORY: probe.workspace },
        });
        assert.equal(status, 0, stderr);
        assert.equal(realpathSync(reported.cwd), realpathSync(probe.workspace));
      });
    }),
  ));

test("a CommonJS tool runs the same way", (t) =>
  joinTest(
    t,
    runOperation(function* () {
      return yield* lifecycle(t.signal, function* () {
        const probe = yield* fixture(t);
        const { status, stderr, script, reported } = yield* fix(probe, "javascript/app", {
          tool: "tool.cjs",
          env: { BUILD_WORKSPACE_DIRECTORY: probe.workspace },
        });
        assert.equal(status, 0, stderr);
        assert.equal(
          realpathSync(reported.cwd),
          realpathSync(join(probe.workspace, "javascript/app")),
        );
        assert.deepEqual(reported.argv, [script, "--write", "."]);
      });
    }),
  ));

test("the tool's exit status is the adapter's", (t) =>
  joinTest(
    t,
    runOperation(function* () {
      return yield* lifecycle(t.signal, function* () {
        const probe = yield* fixture(t);
        const { status, stderr, reported } = yield* fix(probe, "javascript/app", {
          env: { BUILD_WORKSPACE_DIRECTORY: probe.workspace, EXIT: "130" },
        });
        assert.equal(status, 130, stderr);
        assert.notEqual(reported, undefined);
      });
    }),
  ));

// fix_guard_test runs a real fix twin, but only where Bazel runs every action
// on the client.
test("a fix twin refuses to start outside bazel run", (t) =>
  joinTest(
    t,
    runOperation(function* () {
      return yield* lifecycle(t.signal, function* () {
        const probe = yield* fixture(t);
        const { status, stderr, reported } = yield* fix(probe, "javascript/app", {
          env: { BUILD_WORKSPACE_DIRECTORY: undefined },
        });
        assert.equal(status, 1);
        assert.match(stderr, /BUILD_WORKSPACE_DIRECTORY is not set/v);
        assert.equal(reported, undefined);
      });
    }),
  ));

test("a fix twin refuses a package the workspace does not have", (t) =>
  joinTest(
    t,
    runOperation(function* () {
      return yield* lifecycle(t.signal, function* () {
        const probe = yield* fixture(t);
        const { status, stderr, reported } = yield* fix(probe, "javascript/gone", {
          env: { BUILD_WORKSPACE_DIRECTORY: probe.workspace },
        });
        assert.equal(status, 1);
        assert.match(stderr, /cannot enter .*gone/v);
        assert.equal(reported, undefined);
      });
    }),
  ));

// Starlark writes the options; a lenient parser would drop a mistyped one, and
// with it a step such as coverage translation, without a word.
test("an unknown option is refused before the tool starts", (t) =>
  joinTest(
    t,
    runOperation(function* () {
      return yield* lifecycle(t.signal, function* () {
        const { status, stdout, stderr } = yield* run([
          "--coverage-map",
          "map.json",
          "--",
          runnerProbe,
        ]);
        assert.equal(status, 1);
        assert.equal(stdout, "");
        assert.match(stderr, /run-fix: .*coverage-map/v);
      });
    }),
  ));

// A browser binding must be validated before starting the framework.
test("an invalid browser layout is refused before Vitest starts", (t) =>
  joinTest(
    t,
    runOperation(function* () {
      return yield* lifecycle(t.signal, function* () {
        const root = yield* directory(t, "browser-layout-");
        const layout = join(root, "browser.json");
        writeFileSync(layout, JSON.stringify({ marker: "../outside", files: ["native/browser"] }));
        const { status, stdout, stderr } = yield* run(
          ["--browser-layout", layout, "--", runnerProbe],
          {
            cli: "run-vitest",
            env: { BOUND_ROOT: root },
          },
        );
        assert.equal(status, 1);
        assert.equal(stdout, "");
        assert.match(stderr, /invalid declared browser layout/v);
      });
    }),
  ));

// Every real Vitest target passes, so only this case notices a swallowed failure.
test("Vitest's failing exit status passes through", (t) =>
  joinTest(
    t,
    runOperation(function* () {
      return yield* lifecycle(t.signal, function* () {
        const root = yield* directory(t, "vitest-");
        const { status, stderr } = yield* run(["--", runnerProbe, "run", "--fail"], {
          cli: "run-vitest",
          env: { XML_OUTPUT_FILE: join(root, "result.xml"), TEST_UNDECLARED_OUTPUTS_DIR: root },
        });
        assert.equal(status, 9, stderr);
      });
    }),
  ));

// The adapter reads coverage only from the directory it gave the tool.
test("Vitest's report directory is refused under Bazel coverage before the CLI starts", (t) =>
  joinTest(
    t,
    runOperation(function* () {
      return yield* lifecycle(t.signal, function* () {
        const root = yield* directory(t, "vitest-");
        const { status, stdout, stderr } = yield* run(
          ["--", runnerProbe, "run", "--coverage.reportsDirectory=elsewhere"],
          { cli: "run-vitest", env: { COVERAGE_DIR: root } },
        );
        assert.equal(status, 1);
        assert.equal(stdout, "");
        assert.match(stderr, /Bazel coverage chooses --coverage\.reportsDirectory/v);
      });
    }),
  ));

test("invalid Bazel shards are refused before the test runner starts", (t) =>
  joinTest(
    t,
    runOperation(function* () {
      return yield* lifecycle(t.signal, function* () {
        for (const index of ["-1", "3", "not-a-number"]) {
          const { status, stdout, stderr } = yield* run(["--", runnerProbe], {
            cli: "run-vitest",
            env: { TEST_TOTAL_SHARDS: "3", TEST_SHARD_INDEX: index },
          });
          assert.equal(status, 1);
          assert.equal(stdout, "");
          assert.match(stderr, /invalid Bazel test shard/v);
        }
      });
    }),
  ));

// Interruption is not an ordinary numeric failure: discard unfinished reports
// after terminating the framework and its children, then preserve the signal.
test(
  "interrupted adapters terminate descendants and discard reports",
  { skip: process.platform === "win32" },
  (t) =>
    joinTest(
      t,
      runOperation(function* () {
        return yield* lifecycle(t.signal, function* () {
          const root = yield* directory(t, "interrupted-");
          const script = join(root, "framework.cjs");
          const ready = join(root, "ready");
          const report = join(root, "result.xml");
          writeFileSync(
            script,
            `
const fs = require("node:fs");
const child = require("node:child_process").spawn(process.execPath, ["-e", "setInterval(()=>{},1000)"], { stdio: "ignore" });
fs.writeFileSync(process.argv.find(arg => arg.startsWith("--outputFile.junit=")).split("=")[1], "<testsuites>");
child.on("spawn", () => {
  fs.writeFileSync(process.env.READY + ".pending", String(child.pid));
  fs.renameSync(process.env.READY + ".pending", process.env.READY);
});
setInterval(()=>{},1000);
`,
          );
          yield* lifecycle(undefined, function* () {
            const child = spawn(
              executable("run-vitest"),
              ["--node", process.execPath, "--", script],
              {
                env: { ...baseEnv, XML_OUTPUT_FILE: report, READY: ready },
                stdio: "ignore",
              },
            );
            const finished = new Promise((resolve, reject) => {
              child.once("error", reject);
              child.once("exit", (code, signal) => resolve({ code, signal }));
            });
            yield* onCleanup(function* () {
              yield* disposeAll([
                () => child.kill("SIGTERM"),
                () =>
                  race([
                    call(() => finished),
                    (function* () {
                      yield* sleep(3000);
                      throw new Error("Interrupted adapter did not exit");
                    })(),
                  ]),
              ]);
            });
            let pid;
            let observation;
            for (let attempt = 0; attempt < 1000; attempt++) {
              try {
                pid = Number(readFileSync(ready, "utf8"));
                break;
              } catch (error) {
                observation = error;
                yield* sleep(10);
              }
            }
            assert.ok(pid, "framework did not start; last observation: " + String(observation));
            child.kill("SIGTERM");
            const result = yield* call(() => finished);
            assert.equal(result.signal, "SIGTERM");
            assert.equal(result.code, null);
            assert.throws(() => readFileSync(report), { code: "ENOENT" });
            assert.throws(() => process.kill(pid, 0), { code: "ESRCH" });
          });
        });
      }),
    ),
);
