import assert from "node:assert/strict";
import { spawnSync } from "node:child_process";
import { getEventListeners } from "node:events";
import { mkdtempSync, rmSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { test } from "node:test";

import { call, run as runOperation, spawn as spawnOperation, suspend } from "effection";

import { joinTest, lifecycle, onCleanup, settled } from "./lifecycle.mjs";

function* nativeHarness(source, reporter = "tap") {
  const scratch = mkdtempSync(join(process.env.TEST_TMPDIR ?? tmpdir(), "native-lifecycle-"));
  yield* onCleanup(() => rmSync(scratch, { recursive: true, force: true }));
  const environment = { ...process.env, TMPDIR: scratch, TMP: scratch, TEMP: scratch };
  delete environment.NODE_TEST_CONTEXT;
  delete environment.NODE_V8_COVERAGE;
  const program = `
    import assert from "node:assert/strict";
    import { test } from "node:test";
    const { call, run, sleep, suspend } = await import(process.argv[1]);
    const { joinTest, lifecycle, onCleanup } = await import(process.argv[2]);
    const events = [];
    ${source}
    test("next case observes completed cleanup", t => joinTest(t, run(function* () {
      return yield* call(() => {
        assert.deepEqual(events, ["cleanup started", "cleanup completed"]);
        events.push("next case started");
        console.log("NATIVE_LIFECYCLE", JSON.stringify(events));
      });
    })));
  `;
  return yield* call(() =>
    spawnSync(
      process.execPath,
      [
        `--test-reporter=${reporter}`,
        "--input-type=module",
        "-e",
        program,
        import.meta.resolve("effection"),
        new URL("./lifecycle.mjs", import.meta.url).href,
      ],
      { encoding: "utf8", timeout: 5000, killSignal: "SIGKILL", cwd: scratch, env: environment },
    ),
  );
}

test("cleanup retains the body cause and attempts every disposal in registration order", (t) =>
  joinTest(
    t,
    runOperation(function* () {
      return yield* lifecycle(t.signal, function* () {
        const primary = new Error("body failed");
        const first = new Error("first cleanup failed");
        const second = new Error("second cleanup failed");
        const order = [];
        const pending = yield* settled(
          lifecycle(undefined, function* () {
            yield* onCleanup(() => {
              order.push("first");
              throw first;
            });
            yield* onCleanup(() => {
              order.push("second");
              throw second;
            });
            yield* onCleanup(() => {
              order.push("last");
            });
            throw primary;
          }),
        );
        yield* call(() =>
          assert.rejects(pending, (error) => {
            assert.ok(error instanceof AggregateError);
            assert.equal(error.cause, primary);
            assert.deepEqual(error.errors, [primary, first, second]);
            return true;
          }),
        );
        assert.deepEqual(order, ["first", "second", "last"]);
      });
    }),
  ));

test("cleanup failures fail a successful body and still release later resources", (t) =>
  joinTest(
    t,
    runOperation(function* () {
      return yield* lifecycle(t.signal, function* () {
        const failure = new Error("cleanup failed");
        let released = false;
        const pending = yield* settled(
          lifecycle(undefined, function* () {
            yield* onCleanup(() => {
              throw failure;
            });
            yield* onCleanup(() => {
              released = true;
            });
            return 42;
          }),
        );
        yield* call(() =>
          assert.rejects(pending, (error) => {
            assert.ok(error instanceof AggregateError);
            assert.deepEqual(error.errors, [failure]);
            assert.equal(error.cause, undefined);
            return true;
          }),
        );
        assert.equal(released, true);
      });
    }),
  ));

test("native cancellation waits for cleanup and removes its abort listener", (t) =>
  joinTest(
    t,
    runOperation(function* () {
      return yield* lifecycle(t.signal, function* () {
        const controller = new AbortController();
        const reason = new Error("native test cancelled");
        const listeners = getEventListeners(controller.signal, "abort").length;
        let released = false;
        const pending = yield* settled(
          lifecycle(controller.signal, function* () {
            yield* onCleanup(function* () {
              yield* call(() => Promise.resolve());
              released = true;
            });
            yield* spawnOperation(function* () {
              return yield* call(() => {
                controller.abort(reason);
              });
            });
            yield* suspend();
          }),
        );
        yield* call(() => assert.rejects(pending, (error) => error === reason));
        assert.equal(released, true);
        assert.equal(getEventListeners(controller.signal, "abort").length, listeners);
      });
    }),
  ));

test("native cancellation retains its cause when nested cleanup fails", (t) =>
  joinTest(
    t,
    runOperation(function* () {
      return yield* lifecycle(t.signal, function* () {
        const controller = new AbortController();
        const reason = new Error("native test cancelled");
        const failure = new Error("nested cleanup failed");
        let released = false;
        const pending = yield* settled(
          lifecycle(controller.signal, function* () {
            yield* lifecycle(undefined, function* () {
              yield* onCleanup(() => {
                throw failure;
              });
              yield* onCleanup(() => {
                released = true;
              });
              yield* spawnOperation(function* () {
                return yield* call(() => {
                  controller.abort(reason);
                });
              });
              yield* suspend();
            });
          }),
        );
        yield* call(() =>
          assert.rejects(pending, (error) => {
            assert.ok(error instanceof AggregateError);
            assert.equal(error.cause, reason);
            assert.deepEqual(error.errors, [reason, failure]);
            return true;
          }),
        );
        assert.equal(released, true);
      });
    }),
  ));

test("native timeout joins asynchronous cleanup before the next case starts", (t) =>
  joinTest(
    t,
    runOperation(function* () {
      return yield* lifecycle(t.signal, function* () {
        const result = yield* nativeHarness(`
        test("intentional timeout", { timeout: 20 }, t => joinTest(t, run(function* () {
          return yield* lifecycle(t.signal, function* () {
            yield* onCleanup(function* () {
              events.push("cleanup started");
              yield* sleep(80);
              events.push("cleanup completed");
            });
            yield* suspend();
          });
        })));
      `);
        assert.equal(result.error, undefined);
        assert.equal(result.status, 1);
        assert.equal(result.signal, null);
        assert.match(
          result.stdout,
          /NATIVE_LIFECYCLE \["cleanup started","cleanup completed","next case started"\]/v,
        );
        assert.match(result.stdout, /# pass 1/v);
        assert.match(result.stdout, /# fail 0/v);
        assert.match(result.stdout, /# cancelled 1/v);
        assert.match(result.stdout, /test timed out after 20ms/v);
        assert.doesNotMatch(result.stdout, /AggregateError/v);
      });
    }),
  ));

test("native timeout reports cleanup failure while retaining its timeout cause", (t) =>
  joinTest(
    t,
    runOperation(function* () {
      return yield* lifecycle(t.signal, function* () {
        const source = `
        test("intentional timeout with disposal failure", { timeout: 20 }, t => joinTest(t, run(function* () {
          return yield* lifecycle(t.signal, function* () {
            yield* onCleanup(function* () {
              events.push("cleanup started");
              yield* sleep(80);
              events.push("cleanup completed");
              throw new Error("DISPOSAL_MARKER");
            });
            yield* suspend();
          });
        })));
      `;
        for (const [reporter, passingCase, timeoutClassification] of [
          ["tap", /# pass 1/v, /# fail 0\n# cancelled 1/v],
          ["spec", /✔ next case observes completed cleanup/v, /ℹ fail 0\nℹ cancelled 1/v],
          [
            "junit",
            /<testcase name="next case observes completed cleanup"[^>]*\/>/v,
            /type="testTimeoutFailure"/v,
          ],
        ]) {
          const result = yield* nativeHarness(source, reporter);
          assert.equal(result.error, undefined);
          assert.equal(result.status, 1);
          assert.equal(result.signal, null);
          assert.match(
            result.stdout,
            /NATIVE_LIFECYCLE \["cleanup started","cleanup completed","next case started"\]/v,
          );
          assert.match(result.stdout, passingCase);
          assert.match(result.stdout, timeoutClassification);
          assert.match(result.stdout, /test timed out after 20ms/v);
          assert.match(result.stdout, /AggregateError: Native test body and cleanup failed/v);
          assert.match(result.stdout, /DISPOSAL_MARKER/v);
          assert.match(result.stdout, /\[cause\]: DOMException \[AbortError\]/v);
        }
      });
    }),
  ));

test("native completion reports an ordinary body failure once after cleanup", (t) =>
  joinTest(
    t,
    runOperation(function* () {
      return yield* lifecycle(t.signal, function* () {
        const result = yield* nativeHarness(`
        test("intentional body failure", t => joinTest(t, run(function* () {
          return yield* lifecycle(t.signal, function* () {
            yield* onCleanup(function* () {
              events.push("cleanup started");
              yield* sleep(80);
              events.push("cleanup completed");
            });
            throw new Error("PRIMARY_MARKER");
          });
        })));
      `);
        assert.equal(result.error, undefined);
        assert.equal(result.status, 1);
        assert.equal(result.signal, null);
        assert.match(
          result.stdout,
          /NATIVE_LIFECYCLE \["cleanup started","cleanup completed","next case started"\]/v,
        );
        assert.match(result.stdout, /# pass 1/v);
        assert.match(result.stdout, /# fail 1/v);
        assert.match(result.stdout, /# cancelled 0/v);
        assert.equal(result.stdout.match(/failureType: 'testCodeFailure'/gv)?.length, 1);
        assert.match(result.stdout, /error: 'PRIMARY_MARKER'/v);
        assert.doesNotMatch(result.stdout, /AggregateError/v);
      });
    }),
  ));

test("native cancellation during cleanup fails a successful body after releasing every resource", (t) =>
  joinTest(
    t,
    runOperation(function* () {
      return yield* lifecycle(t.signal, function* () {
        const controller = new AbortController();
        const reason = new Error("cancelled during cleanup");
        const order = [];
        const pending = yield* settled(
          lifecycle(controller.signal, function* () {
            yield* onCleanup(function* () {
              order.push("started");
              controller.abort(reason);
              yield* call(() => Promise.resolve());
              order.push("completed");
            });
            yield* onCleanup(() => order.push("later"));
            return 42;
          }),
        );
        yield* call(() => assert.rejects(pending, (error) => error === reason));
        assert.deepEqual(order, ["started", "completed", "later"]);
        assert.equal(getEventListeners(controller.signal, "abort").length, 0);
      });
    }),
  ));

test("native cancellation during cleanup retains its cause and all disposal failures", (t) =>
  joinTest(
    t,
    runOperation(function* () {
      return yield* lifecycle(t.signal, function* () {
        const controller = new AbortController();
        const reason = new Error("cancelled during cleanup");
        const first = new Error("first disposal failed");
        const second = new Error("second disposal failed");
        const order = [];
        const pending = yield* settled(
          lifecycle(controller.signal, function* () {
            yield* onCleanup(function* () {
              controller.abort(reason);
              yield* call(() => Promise.resolve());
              order.push("first");
              throw first;
            });
            yield* onCleanup(() => {
              order.push("second");
              throw second;
            });
            yield* onCleanup(() => order.push("later"));
            return 42;
          }),
        );
        yield* call(() =>
          assert.rejects(pending, (error) => {
            assert.ok(error instanceof AggregateError);
            assert.equal(error.cause, reason);
            assert.deepEqual(error.errors, [reason, first, second]);
            return true;
          }),
        );
        assert.deepEqual(order, ["first", "second", "later"]);
        assert.equal(getEventListeners(controller.signal, "abort").length, 0);
      });
    }),
  ));

test("native timeout during top-level and nested cleanup completes every disposal before the next case", (t) =>
  joinTest(
    t,
    runOperation(function* () {
      return yield* lifecycle(t.signal, function* () {
        const cleanup = `
          yield* onCleanup(function* () {
            events.push("cleanup started");
            yield* sleep(80);
            events.push("cleanup completed");
          });
          yield* onCleanup(() => console.log("LATER_DISPOSAL_COMPLETED"));
          return 42;
        `;
        for (const body of [
          cleanup,
          `return yield* lifecycle(undefined, function* () { ${cleanup} });`,
        ]) {
          const source = `
            test("intentional timeout during cleanup", { timeout: 20 }, t => joinTest(t, run(function* () {
              return yield* lifecycle(t.signal, function* () { ${body} });
            })));
          `;
          for (const [reporter, passingCase, timeoutClassification] of [
            ["tap", /# pass 1/v, /# fail 0\n# cancelled 1/v],
            ["spec", /✔ next case observes completed cleanup/v, /ℹ fail 0\nℹ cancelled 1/v],
            [
              "junit",
              /<testcase name="next case observes completed cleanup"[^>]*\/>/v,
              /type="testTimeoutFailure"/v,
            ],
          ]) {
            const result = yield* nativeHarness(source, reporter);
            assert.equal(result.error, undefined);
            assert.equal(result.status, 1);
            assert.equal(result.signal, null);
            assert.match(
              result.stdout,
              /NATIVE_LIFECYCLE \["cleanup started","cleanup completed","next case started"\]/v,
            );
            assert.match(result.stdout, /LATER_DISPOSAL_COMPLETED/v);
            assert.match(result.stdout, passingCase);
            assert.match(result.stdout, timeoutClassification);
            assert.match(result.stdout, /test timed out after 20ms/v);
            assert.doesNotMatch(result.stdout, /AggregateError/v);
          }
        }
      });
    }),
  ));

test("native timeout during top-level and nested cleanup retains its cause and disposal diagnostics", (t) =>
  joinTest(
    t,
    runOperation(function* () {
      return yield* lifecycle(t.signal, function* () {
        const cleanup = `
          yield* onCleanup(function* () {
            events.push("cleanup started");
            yield* sleep(80);
            events.push("cleanup completed");
            throw new Error("DURING_CLEANUP_MARKER");
          });
          yield* onCleanup(() => console.log("LATER_DISPOSAL_COMPLETED"));
          return 42;
        `;
        for (const body of [
          cleanup,
          `return yield* lifecycle(undefined, function* () { ${cleanup} });`,
        ]) {
          const source = `
            test("intentional timeout during cleanup failure", { timeout: 20 }, t => joinTest(t, run(function* () {
              return yield* lifecycle(t.signal, function* () { ${body} });
            })));
          `;
          for (const [reporter, passingCase, timeoutClassification] of [
            ["tap", /# pass 1/v, /# fail 0\n# cancelled 1/v],
            ["spec", /✔ next case observes completed cleanup/v, /ℹ fail 0\nℹ cancelled 1/v],
            [
              "junit",
              /<testcase name="next case observes completed cleanup"[^>]*\/>/v,
              /type="testTimeoutFailure"/v,
            ],
          ]) {
            const result = yield* nativeHarness(source, reporter);
            assert.equal(result.error, undefined);
            assert.equal(result.status, 1);
            assert.equal(result.signal, null);
            assert.match(
              result.stdout,
              /NATIVE_LIFECYCLE \["cleanup started","cleanup completed","next case started"\]/v,
            );
            assert.match(result.stdout, /LATER_DISPOSAL_COMPLETED/v);
            assert.match(result.stdout, passingCase);
            assert.match(result.stdout, timeoutClassification);
            assert.match(result.stdout, /test timed out after 20ms/v);
            assert.match(result.stdout, /AggregateError: Native test body and cleanup failed/v);
            assert.match(result.stdout, /DURING_CLEANUP_MARKER/v);
            assert.match(result.stdout, /\[cause\]: DOMException \[AbortError\]/v);
          }
        }
      });
    }),
  ));

test("nested failures retain their exact values when cancellation arrives during failing cleanup", (t) =>
  joinTest(
    t,
    runOperation(function* () {
      return yield* lifecycle(t.signal, function* () {
        for (const primary of [undefined, null, new Error("nested body failed")]) {
          const controller = new AbortController();
          const reason = new Error("cancelled during nested cleanup");
          const failure = new Error("nested cleanup failed");
          const order = [];
          const pending = yield* settled(
            lifecycle(controller.signal, function* () {
              yield* lifecycle(undefined, function* () {
                yield* onCleanup(function* () {
                  controller.abort(reason);
                  yield* call(() => Promise.resolve());
                  order.push("completed");
                  throw failure;
                });
                yield* onCleanup(() => order.push("later"));
                throw primary;
              });
            }),
          );
          yield* call(() =>
            assert.rejects(pending, (error) => {
              assert.ok(error instanceof AggregateError);
              assert.equal(error.cause, reason);
              assert.deepEqual(error.errors, [reason, primary, failure]);
              return true;
            }),
          );
          assert.deepEqual(order, ["completed", "later"]);
        }
      });
    }),
  ));

test("nested body failure survives cancellation during successful cleanup", (t) =>
  joinTest(
    t,
    runOperation(function* () {
      return yield* lifecycle(t.signal, function* () {
        const controller = new AbortController();
        const reason = new Error("cancelled during nested cleanup");
        const primary = new Error("nested body failed");
        const order = [];
        const pending = yield* settled(
          lifecycle(controller.signal, function* () {
            yield* lifecycle(undefined, function* () {
              yield* onCleanup(function* () {
                controller.abort(reason);
                yield* call(() => Promise.resolve());
                order.push("completed");
              });
              yield* onCleanup(() => order.push("later"));
              throw primary;
            });
          }),
        );
        yield* call(() =>
          assert.rejects(pending, (error) => {
            assert.ok(error instanceof AggregateError);
            assert.equal(error.cause, reason);
            assert.deepEqual(error.errors, [reason, primary]);
            return true;
          }),
        );
        assert.deepEqual(order, ["completed", "later"]);
      });
    }),
  ));

test("native timeout during successful nested cleanup reports the earlier body failure", (t) =>
  joinTest(
    t,
    runOperation(function* () {
      return yield* lifecycle(t.signal, function* () {
        const source = `
          test("intentional nested failure before cleanup timeout", { timeout: 20 }, t => joinTest(t, run(function* () {
            return yield* lifecycle(t.signal, function* () {
              yield* lifecycle(undefined, function* () {
                yield* onCleanup(function* () {
                  events.push("cleanup started");
                  yield* sleep(80);
                  events.push("cleanup completed");
                });
                yield* onCleanup(() => console.log("LATER_DISPOSAL_COMPLETED"));
                throw new Error("NESTED_BODY_MARKER");
              });
            });
          })));
        `;
        for (const [reporter, passingCase, timeoutClassification] of [
          ["tap", /# pass 1/v, /# fail 0\n# cancelled 1/v],
          ["spec", /✔ next case observes completed cleanup/v, /ℹ fail 0\nℹ cancelled 1/v],
          [
            "junit",
            /<testcase name="next case observes completed cleanup"[^>]*\/>/v,
            /type="testTimeoutFailure"/v,
          ],
        ]) {
          const result = yield* nativeHarness(source, reporter);
          assert.equal(result.error, undefined);
          assert.equal(result.status, 1);
          assert.equal(result.signal, null);
          assert.match(
            result.stdout,
            /NATIVE_LIFECYCLE \["cleanup started","cleanup completed","next case started"\]/v,
          );
          assert.match(result.stdout, /LATER_DISPOSAL_COMPLETED/v);
          assert.match(result.stdout, passingCase);
          assert.match(result.stdout, timeoutClassification);
          assert.match(result.stdout, /test timed out after 20ms/v);
          assert.match(result.stdout, /AggregateError: Native test body and cleanup failed/v);
          assert.match(result.stdout, /NESTED_BODY_MARKER/v);
          assert.match(result.stdout, /\[cause\]: DOMException \[AbortError\]/v);
        }
      });
    }),
  ));

test("native timeout during failing nested cleanup reports both earlier body and disposal failures", (t) =>
  joinTest(
    t,
    runOperation(function* () {
      return yield* lifecycle(t.signal, function* () {
        const source = `
          test("intentional nested failure before failing cleanup timeout", { timeout: 20 }, t => joinTest(t, run(function* () {
            return yield* lifecycle(t.signal, function* () {
              yield* lifecycle(undefined, function* () {
                yield* onCleanup(function* () {
                  events.push("cleanup started");
                  yield* sleep(80);
                  events.push("cleanup completed");
                  throw new Error("NESTED_DISPOSAL_MARKER");
                });
                yield* onCleanup(() => console.log("LATER_DISPOSAL_COMPLETED"));
                throw new Error("NESTED_BODY_MARKER");
              });
            });
          })));
        `;
        for (const [reporter, passingCase, timeoutClassification] of [
          ["tap", /# pass 1/v, /# fail 0\n# cancelled 1/v],
          ["spec", /✔ next case observes completed cleanup/v, /ℹ fail 0\nℹ cancelled 1/v],
          [
            "junit",
            /<testcase name="next case observes completed cleanup"[^>]*\/>/v,
            /type="testTimeoutFailure"/v,
          ],
        ]) {
          const result = yield* nativeHarness(source, reporter);
          assert.equal(result.error, undefined);
          assert.equal(result.status, 1);
          assert.equal(result.signal, null);
          assert.match(
            result.stdout,
            /NATIVE_LIFECYCLE \["cleanup started","cleanup completed","next case started"\]/v,
          );
          assert.match(result.stdout, /LATER_DISPOSAL_COMPLETED/v);
          assert.match(result.stdout, passingCase);
          assert.match(result.stdout, timeoutClassification);
          assert.match(result.stdout, /test timed out after 20ms/v);
          assert.match(result.stdout, /AggregateError: Native test body and cleanup failed/v);
          assert.match(result.stdout, /NESTED_BODY_MARKER/v);
          assert.match(result.stdout, /NESTED_DISPOSAL_MARKER/v);
          assert.match(result.stdout, /\[cause\]: DOMException \[AbortError\]/v);
        }
      });
    }),
  ));
