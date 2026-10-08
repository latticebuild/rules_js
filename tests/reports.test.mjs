import assert from "node:assert/strict";
import { spawnSync } from "node:child_process";
import { existsSync, mkdirSync, mkdtempSync, readdirSync, readFileSync, rmSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { test } from "node:test";

import { call, run as runOperation } from "effection";

import { joinTest, onCleanup, lifecycle } from "./lifecycle.mjs";

// Every real test passes without anyone reading its reports, so only these
// fixtures notice an executable that stopped writing them: each runs with its
// own XML_OUTPUT_FILE and COVERAGE_DIR, and the reports it leaves are read back.
const fixtures = join(import.meta.dirname, "../testdata/runners");
const suffix = process.platform === "win32" ? ".exe" : "";

// A directory removed after the test.
function* directory() {
  const created = mkdtempSync(join(process.env.TEST_TMPDIR ?? tmpdir(), "reports-"));
  yield* onCleanup(function* () {
    return yield* call(() => {
      return rmSync(created, { recursive: true, force: true });
    });
  });
  return created;
}

// Runs a fixture with its own reports and the variables `extra` sets. This
// test's own filter, shard and runner variables never reach it: this file runs
// as a test runner's child, whose NODE_TEST_CONTEXT would make node:test skip
// the fixture's files, and the entry's removal of it has a case of its own.
function* run(t, name, extra = {}) {
  const scratch = yield* directory();
  const coverage = join(scratch, "coverage");
  mkdirSync(coverage);
  const xml = join(scratch, "test.xml");
  const env = { ...process.env, XML_OUTPUT_FILE: xml, COVERAGE_DIR: coverage };
  for (const key of [
    "COVERAGE_MANIFEST",
    "NODE_TEST_CONTEXT",
    "NODE_V8_COVERAGE",
    "TESTBRIDGE_TEST_ONLY",
    "TEST_SHARD_INDEX",
    "TEST_SHARD_STATUS_FILE",
    "TEST_TOTAL_SHARDS",
    "TEST_UNDECLARED_OUTPUTS_DIR",
  ]) {
    delete env[key];
  }
  Object.assign(env, extra);
  const result = spawnSync(join(fixtures, name + suffix), { encoding: "utf8", env });
  if (result.error) {
    throw result.error;
  }
  const output = result.stdout + result.stderr;
  const tracefiles = readdirSync(coverage).map((file) =>
    readFileSync(join(coverage, file), "utf8"),
  );
  return { status: result.status, output, xml: readFileSync(xml, "utf8"), tracefiles };
}

const source = "SF:testdata/runners/reported.mjs\n";

test("js_test writes a case per test and the repository path of the covered source", (t) =>
  joinTest(
    t,
    runOperation(function* () {
      return yield* lifecycle(t.signal, function* () {
        const { status, output, xml, tracefiles } = yield* run(t, "node_reports_test");
        assert.equal(status, 0, output);
        assert.match(xml, /<testcase [^>]*name="adds"/v);
        assert.match(xml, /<testcase [^>]*name="skipped"[^>]*>\s*<skipped/v);
        assert.ok(
          tracefiles.some((tracefile) => tracefile.includes(source)),
          tracefiles.join("\n"),
        );
      });
    }),
  ));

// Every real js_test passes, so only this case notices a swallowed failure.
test("js_test fails with its test and reports the failing case", (t) =>
  joinTest(
    t,
    runOperation(function* () {
      return yield* lifecycle(t.signal, function* () {
        const { status, output, xml } = yield* run(t, "node_failure_test");
        assert.notEqual(status, 0, output);
        assert.match(xml, /<testcase [^>]*name="fails"[^>]*>\s*<failure/v);
      });
    }),
  ));

// The names of the cases a JUnit report holds; `classname` also ends in
// `name=`, so the attribute must follow whitespace.
const cases = (xml) =>
  [...xml.matchAll(/<testcase\b[^>]*\sname="([^"]+)"/gv)].map((match) => match[1]);

test("js_test runs only the cases Bazel's test filter names", (t) =>
  joinTest(
    t,
    runOperation(function* () {
      return yield* lifecycle(t.signal, function* () {
        const { status, output, xml } = yield* run(t, "node_reports_test", {
          TESTBRIDGE_TEST_ONLY: "adds",
        });
        assert.equal(status, 0, output);
        assert.ok(cases(xml).includes("adds"), xml);
        assert.ok(!cases(xml).includes("skipped"), xml);
      });
    }),
  ));

// Node shards by test file; Bazel fails a sharded test that never touches the
// status file.
test("js_test shards run a share of the test files each", (t) =>
  joinTest(
    t,
    runOperation(function* () {
      return yield* lifecycle(t.signal, function* () {
        const ran = [];
        for (const index of ["0", "1"]) {
          const statusFile = join(yield* directory(), "shard-status");
          const { status, output, xml } = yield* run(t, "node_shards_test", {
            TEST_TOTAL_SHARDS: "2",
            TEST_SHARD_INDEX: index,
            TEST_SHARD_STATUS_FILE: statusFile,
          });
          assert.equal(status, 0, output);
          assert.ok(existsSync(statusFile), "the shard touched TEST_SHARD_STATUS_FILE");
          const shard = cases(xml).filter((name) => name === "first" || name === "second");
          assert.equal(shard.length, 1, xml);
          ran.push(...shard);
        }
        assert.deepEqual(ran.toSorted(), ["first", "second"]);
      });
    }),
  ));

// Inherited, NODE_TEST_CONTEXT would make node:test skip every file and report
// success; the entry removes it, and the report proves the files ran.
test("js_test runs its files under an enclosing test runner's NODE_TEST_CONTEXT", (t) =>
  joinTest(
    t,
    runOperation(function* () {
      return yield* lifecycle(t.signal, function* () {
        const { status, output, xml } = yield* run(t, "node_reports_test", {
          NODE_TEST_CONTEXT: "child-v8",
        });
        assert.equal(status, 0, output);
        assert.ok(cases(xml).includes("adds"), xml);
      });
    }),
  ));
