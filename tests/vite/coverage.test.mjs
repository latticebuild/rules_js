import assert from "node:assert/strict";
import { spawnSync } from "node:child_process";
import {
  existsSync,
  mkdirSync,
  mkdtempSync,
  readdirSync,
  readFileSync,
  realpathSync,
  rmSync,
  symlinkSync,
  writeFileSync,
} from "node:fs";
import { tmpdir } from "node:os";
import { dirname, join } from "node:path";
import { test } from "node:test";

import { call, run as runOperation } from "effection";

import { joinTest, onCleanup, lifecycle } from "./lifecycle.mjs";

// For js_test and js_vitest the native adapter calls coverage preparation before the
// tool starts. Under `bazel coverage` the tool writes its tracefiles into a
// private COVERAGE_DIR in the tree; after a zero exit, the records of the files
// Bazel lists are translated into Bazel's COVERAGE_DIR.

const suffix = process.platform === "win32" ? ".exe" : "";
const adapter = join(
  import.meta.dirname,
  `../../js/private/vite/tools/run-vitest/run-vitest_/run-vitest${suffix}`,
);
const scratch = () => process.env.TEST_TMPDIR ?? tmpdir();

const record = (source) => `TN:\nSF:${source}\nDA:1,1\nLF:1\nLH:1\nend_of_record\n`;

// The tree files Bazel lists, with the paths it reports them by.
const listed = {
  "javascript/app/src/index.js": "javascript/app/src/index.js",
  "javascript/app/src/gen.js": "bazel-out/k8-fastbuild/bin/javascript/app/src/gen.js",
};

// An extracted bundle under `parent`: the coverage map beside tree/, and a tool
// in the tree that calls coverage preparation as the native adapter does, with the map
// its first argument names, writes the tracefiles TRACEFILES holds into
// COVERAGE_DIR, reports what it saw, and ends as END says: `exit N` with
// process.exit, as Vitest ends, or `code N` with process.exitCode, as the
// node:test runner ends.
function* bundle(t, map, parent = scratch()) {
  const root = mkdtempSync(join(parent, "bundle-"));
  yield* onCleanup(function* () {
    return yield* call(() => {
      return rmSync(root, { recursive: true, force: true });
    });
  });
  const tree = join(root, "tree");
  mkdirSync(join(tree, "javascript/app"), { recursive: true });
  writeFileSync(
    join(tree, "tool.mjs"),
    `import fs from "node:fs";
import path from "node:path";
for (const [name, body] of Object.entries(JSON.parse(process.env.TRACEFILES))) {
  fs.writeFileSync(path.join(process.env.COVERAGE_DIR, name), body);
}
fs.writeFileSync(process.env.RESULT, JSON.stringify({
  coverageDir: process.env.COVERAGE_DIR ?? null,
}));
const [how, status] = process.env.END.split(" ");
if (how === "exit") {
  process.exit(Number(status));
}
process.exitCode = Number(status);
`,
  );
  const mapFile = join(root, "coverage_map.json");
  writeFileSync(mapFile, JSON.stringify(map));
  return { root, tree, mapFile };
}

// Bazel's COVERAGE_DIR for one run.
function* destination() {
  const directory = mkdtempSync(join(scratch(), "published-"));
  yield* onCleanup(function* () {
    return yield* call(() => {
      return rmSync(directory, { recursive: true, force: true });
    });
  });
  return directory;
}

// What a run published into a COVERAGE_DIR.
function published(directory) {
  return Object.fromEntries(
    readdirSync(directory).map((name) => [name, readFileSync(join(directory, name), "utf8")]),
  );
}

// Runs the bundle's tool with node started in its package directory, as bound
// starts it, with Bazel's COVERAGE_DIR set to `coverageDir` (unset when it is
// undefined).
function* run(bundled, { coverageDir, tracefiles = {}, end = "exit 0" } = {}) {
  return yield* call(() => {
    const result = join(bundled.root, "result.json");
    const env = {
      ...process.env,
      RESULT: result,
      TRACEFILES: JSON.stringify(tracefiles),
      END: end,
    };
    for (const key of [
      "COVERAGE_DIR",
      "XML_OUTPUT_FILE",
      "TEST_TOTAL_SHARDS",
      "TEST_SHARD_INDEX",
      "TEST_SHARD_STATUS_FILE",
      "TEST_UNDECLARED_OUTPUTS_DIR",
      "TESTBRIDGE_TEST_ONLY",
    ]) {
      delete env[key];
    }
    if (coverageDir !== undefined) {
      env.COVERAGE_DIR = coverageDir;
    }
    const child = spawnSync(
      adapter,
      [
        "--node",
        process.execPath,
        "--coverage",
        bundled.mapFile,
        "--",
        join(bundled.tree, "tool.mjs"),
      ],
      {
        encoding: "utf8",
        env,
        cwd: join(bundled.tree, "javascript/app"),
      },
    );
    if (child.error) {
      throw child.error;
    }
    let reported;
    try {
      reported = JSON.parse(readFileSync(result, "utf8"));
    } catch {
      // The tool failed before it reported.
    }
    return { status: child.status, output: child.stdout + child.stderr, reported };
  });
}

test("coverage records of listed files are translated and the rest dropped", (t) =>
  joinTest(
    t,
    runOperation(function* () {
      return yield* lifecycle(t.signal, function* () {
        const bundled = yield* bundle(t, listed);
        const root = realpathSync(bundled.tree);
        const coverageDir = yield* destination();
        const { status, output, reported } = yield* run(bundled, {
          coverageDir,
          tracefiles: {
            "vitest.info": [
              // Relative to the tool's working directory.
              record("src/index.js"),
              record(join(root, "javascript/app/src/gen.js")),
              record(join(root, "node_modules/pkg/index.js")),
              record(join(scratch(), "elsewhere.js")),
            ].join(""),
          },
        });
        assert.equal(status, 0, output);
        assert.equal(dirname(reported.coverageDir), bundled.tree);
        assert.deepEqual(published(coverageDir), {
          "js-vitest.dat":
            record("javascript/app/src/index.js") + record(listed["javascript/app/src/gen.js"]),
        });
      });
    }),
  ));

test("coverage from a tree behind a symlink matches the real paths tools report", (t) =>
  joinTest(
    t,
    runOperation(function* () {
      return yield* lifecycle(t.signal, function* () {
        const real = mkdtempSync(join(scratch(), "real-"));
        yield* onCleanup(function* () {
          return yield* call(() => {
            return rmSync(real, { recursive: true, force: true });
          });
        });
        const linked = `${real}-linked`;
        symlinkSync(real, linked, "junction");
        yield* onCleanup(function* () {
          return yield* call(() => {
            return rmSync(linked, { force: true });
          });
        });
        const bundled = yield* bundle(t, listed, linked);
        const coverageDir = yield* destination();
        const { status, output } = yield* run(bundled, {
          coverageDir,
          tracefiles: {
            "node.info": record(join(realpathSync(bundled.tree), "javascript/app/src/index.js")),
          },
        });
        assert.equal(status, 0, output);
        assert.deepEqual(published(coverageDir), {
          "js-node.dat": record("javascript/app/src/index.js"),
        });
      });
    }),
  ));

test(
  "coverage paths in either of Windows' spellings match",
  { skip: process.platform !== "win32" },
  (t) =>
    joinTest(
      t,
      runOperation(function* () {
        return yield* lifecycle(t.signal, function* () {
          const bundled = yield* bundle(t, listed);
          const root = realpathSync(bundled.tree);
          const coverageDir = yield* destination();
          const { status, output } = yield* run(bundled, {
            coverageDir,
            tracefiles: {
              "node.info": [
                record(
                  `${root.replace(/^[A-Z]:/v, (drive) => drive.toLowerCase())}\\javascript\\app\\src\\index.js`,
                ),
                record(`${root.replaceAll("\\", "/")}/javascript/app/src/index.js`),
              ].join(""),
            },
          });
          assert.equal(status, 0, output);
          assert.deepEqual(published(coverageDir), {
            "js-node.dat": record("javascript/app/src/index.js").repeat(2),
          });
        });
      }),
    ),
);

test("a failed run publishes no coverage", (t) =>
  joinTest(
    t,
    runOperation(function* () {
      return yield* lifecycle(t.signal, function* () {
        const coverageDir = yield* destination();
        const { status } = yield* run(yield* bundle(t, listed), {
          coverageDir,
          tracefiles: { "node.info": record("src/index.js") },
          end: "exit 3",
        });
        assert.equal(status, 3);
        assert.deepEqual(published(coverageDir), {});
      });
    }),
  ));

// Vitest ends with process.exit, the node:test runner by setting process.exitCode.
for (const end of ["exit 0", "code 0"]) {
  test(`a truncated tracefile fails the run (${end})`, (t) =>
    joinTest(
      t,
      runOperation(function* () {
        return yield* lifecycle(t.signal, function* () {
          const coverageDir = yield* destination();
          const { status, output } = yield* run(yield* bundle(t, listed), {
            coverageDir,
            tracefiles: { "node.info": "TN:\nSF:src/index.js\nDA:1,1\n" },
            end,
          });
          assert.equal(status, 1);
          assert.match(output, /node\.info: record for src\/index\.js has no end_of_record/v);
          assert.deepEqual(published(coverageDir), {});
        });
      }),
    ));
}

// Otherwise every ordinary run would turn its tool's coverage on.
test("a run without COVERAGE_DIR keeps it unset and creates nothing", (t) =>
  joinTest(
    t,
    runOperation(function* () {
      return yield* lifecycle(t.signal, function* () {
        const bundled = yield* bundle(t, listed);
        const { status, output, reported } = yield* run(bundled);
        assert.equal(status, 0, output);
        assert.equal(reported.coverageDir, null);
        assert.equal(existsSync(join(bundled.tree, ".coverage")), false);
      });
    }),
  ));
