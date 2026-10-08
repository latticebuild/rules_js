import assert from "node:assert/strict";
import { spawnSync } from "node:child_process";
import fs from "node:fs";
import os from "node:os";
import path from "node:path";
import { describe, it } from "node:test";
import { fileURLToPath } from "node:url";

import { call, run as runOperation } from "effection";

import { joinTest, onCleanup, lifecycle } from "./lifecycle.mjs";

const suffix = process.platform === "win32" ? ".exe" : "";
const checker = path.join(
  import.meta.dirname,
  `../../js/private/ts/tools/compile-typescript/compile-typescript_/compile-typescript${suffix}`,
);
const api = path.dirname(fileURLToPath(import.meta.resolve("typescript/package.json")));
const compiler = path.join(api, "bin/tsc");

function* project(t, options = {}, files) {
  files ??= { "src/index.ts": "export const value = 42;" };
  const directory = fs.mkdtempSync(path.join(os.tmpdir(), "tsc-"));
  yield* onCleanup(function* () {
    return yield* call(() => {
      return fs.rmSync(directory, { recursive: true, force: true });
    });
  });
  for (const [name, contents] of Object.entries(files)) {
    const file = path.join(directory, name);
    fs.mkdirSync(path.dirname(file), { recursive: true });
    fs.writeFileSync(file, contents);
  }
  const effective = { rootDir: "src", outDir: "dist", ...options };
  const config = path.join(directory, "tsconfig.json");
  fs.writeFileSync(
    config,
    JSON.stringify({
      compilerOptions: {
        module: "NodeNext",
        target: "ES2022",
        types: [],
        ...effective,
      },
      include: ["src/**/*"],
    }),
  );
  const sourceFiles = Object.keys(files).filter((file) => file.startsWith("src/"));
  return {
    directory,
    config,
    options: effective,
    sources: sourceFiles.map((file) => path.join(directory, file)),
    *run(
      outputs,
      command = compiler,
      declared = effective,
      sources = sourceFiles,
      directories = [],
      validation = api,
    ) {
      return yield* call(() => {
        return spawnSync(
          checker,
          [
            command,
            "tsconfig.json",
            JSON.stringify(declared),
            JSON.stringify(sources),
            JSON.stringify(directories),
            JSON.stringify(outputs),
            validation,
            "validation.check",
          ],
          { cwd: directory, encoding: "utf8", env: { ...process.env, NODE: process.execPath } },
        );
      });
    },
  };
}

function successful(result) {
  assert.equal(result.status, 0, result.stdout + result.stderr);
}

describe("effective configuration failures", () => {
  it("reports declared options that differ from the effective configuration", (t) =>
    joinTest(
      t,
      runOperation(function* () {
        return yield* lifecycle(t.signal, function* () {
          const p = yield* project(t, { sourceMap: true });
          assert.match(
            (yield* p.run([], compiler, { rootDir: "src", outDir: "build" })).stderr,
            /compilerOptions.outDir.*declared "build"/v,
          );
          assert.match(
            (yield* p.run([], compiler, { rootDir: "src", outDir: "dist" })).stderr,
            /compilerOptions.sourceMap is true, declared false/v,
          );
        });
      }),
    ));
  it("reports unresolved installed and missing inherited configurations", (t) =>
    joinTest(
      t,
      runOperation(function* () {
        return yield* lifecycle(t.signal, function* () {
          for (const inherited of ["@tsconfig/strictest/tsconfig.json", "./gone.json"]) {
            const p = yield* project(t);
            fs.writeFileSync(
              p.config,
              JSON.stringify({ extends: inherited, files: ["src/index.ts"] }),
            );
            const failed = yield* p.run([]);
            assert.notEqual(failed.status, 0);
            assert.match(failed.stderr, /TS(?:6053|5083)/v);
          }
        });
      }),
    ));
  it("rejects unknown declared options", (t) =>
    joinTest(
      t,
      runOperation(function* () {
        return yield* lifecycle(t.signal, function* () {
          const p = yield* project(t);
          assert.match(
            (yield* p.run([], compiler, { ...p.options, strcit: true })).stderr,
            /Unknown compiler option 'strcit'/v,
          );
        });
      }),
    ));
  it("normalizes inherited paths, enum spellings and null resets", (t) =>
    joinTest(
      t,
      runOperation(function* () {
        return yield* lifecycle(t.signal, function* () {
          const p = yield* project(t);
          fs.mkdirSync(path.join(p.directory, "config"));
          fs.writeFileSync(
            path.join(p.directory, "config/base.json"),
            JSON.stringify({
              compilerOptions: {
                rootDir: "../src",
                outDir: "../dist",
                sourceMap: true,
                module: "nodenext",
                target: "es2022",
                types: [],
              },
            }),
          );
          fs.writeFileSync(
            p.config,
            JSON.stringify({
              extends: "./config/base.json",
              compilerOptions: { sourceMap: null },
              files: ["src/index.ts"],
            }),
          );
          successful(
            yield* p.run(["dist/index.js"], compiler, {
              ...p.options,
              sourceMap: null,
              module: "NodeNext",
              target: "ES2022",
            }),
          );
        });
      }),
    ));
  it("normalizes a reset inherited rootDir to the compiler default", (t) =>
    joinTest(
      t,
      runOperation(function* () {
        return yield* lifecycle(t.signal, function* () {
          const p = yield* project(t);
          fs.writeFileSync(
            path.join(p.directory, "base.json"),
            JSON.stringify({ compilerOptions: { rootDir: "src", outDir: "dist" } }),
          );
          fs.writeFileSync(
            p.config,
            JSON.stringify({
              extends: "./base.json",
              compilerOptions: { rootDir: null, noEmit: true },
              files: ["src/index.ts"],
            }),
          );
          successful(yield* p.run([], compiler, { outDir: "dist", noEmit: true }));
        });
      }),
    ));
  it("distinguishes inherited null scalar paths from explicit directories", (t) =>
    joinTest(
      t,
      runOperation(function* () {
        return yield* lifecycle(t.signal, function* () {
          const p = yield* project(t);
          fs.writeFileSync(
            path.join(p.directory, "base.json"),
            JSON.stringify({ compilerOptions: { rootDir: "src", outDir: "dist" } }),
          );
          fs.writeFileSync(
            p.config,
            JSON.stringify({
              extends: "./base.json",
              compilerOptions: { outDir: null, noEmit: true },
              files: ["src/index.ts"],
            }),
          );
          successful(yield* p.run([], compiler, { rootDir: "src", noEmit: true }));
          assert.match(
            (yield* p.run([], compiler, { rootDir: "src", outDir: ".", noEmit: true })).stderr,
            /compilerOptions.outDir is null/v,
          );
          assert.equal(
            fs.readdirSync(p.directory).some((name) => name.startsWith(".latticebuild-compiler-")),
            false,
          );
          fs.writeFileSync(
            p.config,
            JSON.stringify({
              compilerOptions: { rootDir: "src", outDir: ".", noEmit: true },
              files: ["src/index.ts"],
            }),
          );
          successful(yield* p.run([], compiler, { rootDir: "src", outDir: ".", noEmit: true }));
        });
      }),
    ));
  it("refuses a project when the selected validation CLI cannot inspect it", (t) =>
    joinTest(
      t,
      runOperation(function* () {
        return yield* lifecycle(t.signal, function* () {
          for (const key of ["rootDirs", "typeRoots"]) {
            const p = yield* project(t, { [key]: null, noEmit: true });
            const failed = yield* p.run([]);
            assert.notEqual(failed.status, 0);
            assert.equal(fs.existsSync(path.join(p.directory, "validation.check")), false);
          }
        });
      }),
    ));
  it("inspects scalar paths when the project uses implicit source discovery", (t) =>
    joinTest(
      t,
      runOperation(function* () {
        return yield* lifecycle(t.signal, function* () {
          const p = yield* project(t, { rootDir: ".", noEmit: true });
          const config = JSON.parse(fs.readFileSync(p.config, "utf8"));
          delete config.include;
          fs.writeFileSync(p.config, JSON.stringify(config));
          successful(yield* p.run([]));
        });
      }),
    ));
  it("removes the inherited probe when its inspection fails", (t) =>
    joinTest(
      t,
      runOperation(function* () {
        return yield* lifecycle(t.signal, function* () {
          const p = yield* project(t, { rootDir: ".", noEmit: true });
          const validation = path.join(p.directory, "validation");
          fs.mkdirSync(validation);
          fs.writeFileSync(
            path.join(validation, "package.json"),
            JSON.stringify({ bin: { tsc: "validator.cjs" } }),
          );
          fs.writeFileSync(
            path.join(validation, "validator.cjs"),
            `if (process.argv.at(-1).includes(".latticebuild-compiler-config-")) {
console.error("inherited inspection failed"); process.exit(1);
}
console.log(JSON.stringify({compilerOptions:{rootDir:"."},files:["src/index.ts"]}));`,
          );
          const result = yield* p.run([], compiler, p.options, undefined, undefined, validation);
          assert.notEqual(result.status, 0);
          assert.match(result.stderr, /inherited inspection failed/v);
          assert.equal(fs.existsSync(path.join(p.directory, "validation.check")), false);
          assert.equal(
            fs.readdirSync(p.directory).some((name) => name.startsWith(".latticebuild-compiler-")),
            false,
          );
        });
      }),
    ));
  it("derives declaration and incremental from composite", (t) =>
    joinTest(
      t,
      runOperation(function* () {
        return yield* lifecycle(t.signal, function* () {
          const p = yield* project(t, { composite: true });
          successful(yield* p.run(["dist/index.js", "dist/index.d.ts", "tsconfig.tsbuildinfo"]));
        });
      }),
    ));
  it("rejects compilation without sources", (t) =>
    joinTest(
      t,
      runOperation(function* () {
        return yield* lifecycle(t.signal, function* () {
          const p = yield* project(t, {}, {});
          const result = yield* p.run([]);
          assert.notEqual(result.status, 0);
          assert.match(result.stderr, /TS18003/v);
        });
      }),
    ));
  it("rejects project references before invoking the selected compiler", (t) =>
    joinTest(
      t,
      runOperation(function* () {
        return yield* lifecycle(t.signal, function* () {
          const p = yield* project(t);
          const config = JSON.parse(fs.readFileSync(p.config, "utf8"));
          config.references = [{ path: "./other" }];
          fs.writeFileSync(p.config, JSON.stringify(config));
          const result = yield* p.run(["dist/index.js"]);
          assert.notEqual(result.status, 0);
          assert.match(result.stderr, /project references are unsupported/v);
          assert.equal(fs.existsSync(path.join(p.directory, "dist")), false);
        });
      }),
    ));
});

describe("compiler input and emission failures", () => {
  it("includes imported implementation and JSON files even outside include patterns", (t) =>
    joinTest(
      t,
      runOperation(function* () {
        return yield* lifecycle(t.signal, function* () {
          const p = yield* project(
            t,
            { resolveJsonModule: true },
            {
              "src/index.ts":
                "import data from './value.json'; export { data }; export { extra } from './extra.js';",
              "src/extra.ts": "export const extra = 42;",
              "src/value.json": '{"value":42}',
            },
          );
          const config = JSON.parse(fs.readFileSync(p.config, "utf8"));
          config.include = ["src/index.ts"];
          fs.writeFileSync(p.config, JSON.stringify(config));
          const result = yield* p.run(["dist/index.js"], compiler, p.options, ["src/index.ts"]);
          assert.notEqual(result.status, 0);
          assert.match(result.stderr, /extra.ts is not a declared emitting source/v);
          assert.match(result.stderr, /value.json is not a declared emitting source/v);
          successful(yield* p.run(["dist/index.js", "dist/extra.js", "dist/value.json"]));
        });
      }),
    ));
  it("accepts undeclared inputs only inside a generated directory, which must not emit", (t) =>
    joinTest(
      t,
      runOperation(function* () {
        return yield* lifecycle(t.signal, function* () {
          // SvelteKit's route types hold proxies a project imports without declaring.
          const proxy = { "types/proxy.ts": "export const load = 1;" };
          const sources = ["src/index.ts"];
          const inside = yield* project(
            t,
            { rootDir: ".", noEmit: true },
            { "src/index.ts": "export { load } from '../types/proxy.js';", ...proxy },
          );
          successful(yield* inside.run([], compiler, inside.options, sources, ["types"]));
          assert.match(
            (yield* inside.run([], compiler, inside.options, sources)).stderr,
            /proxy\.ts is not a declared emitting source/v,
          );
          // A sibling whose name starts with the directory's is outside it.
          const sibling = yield* project(
            t,
            { rootDir: ".", noEmit: true },
            {
              "src/index.ts": "export { other } from '../types2/other.js';",
              "types2/other.ts": "export const other = 2;",
            },
          );
          assert.match(
            (yield* sibling.run([], compiler, sibling.options, sources, ["types"])).stderr,
            /other\.ts is not a declared emitting source/v,
          );
          // Emission stays declared: a generated input may not emit.
          const emitting = yield* project(
            t,
            { rootDir: "." },
            { "src/index.ts": "export { load } from '../types/proxy.js';", ...proxy },
          );
          const result = yield* emitting.run(
            ["dist/src/index.js"],
            compiler,
            emitting.options,
            sources,
            ["types"],
          );
          assert.notEqual(result.status, 0);
          assert.match(
            result.stderr,
            /selected compiler emitted.*proxy\.js.*but the target declares/v,
          );
        });
      }),
    ));
  it("rejects excluded sources, extra outputs, missing outputs and collisions", (t) =>
    joinTest(
      t,
      runOperation(function* () {
        return yield* lifecycle(t.signal, function* () {
          const p = yield* project(t);
          const result = yield* p.run(["extra.js"], compiler, p.options, [
            "src/index.ts",
            "excluded.ts",
          ]);
          assert.notEqual(result.status, 0);
          assert.match(result.stderr, /excluded.ts is not a compiler input/v);
          assert.match(
            result.stderr,
            /selected compiler emitted.*index.js.*target declares.*extra.js/v,
          );
          const collision = yield* project(
            t,
            { jsx: "react" },
            { "src/index.ts": "export {};", "src/index.tsx": "export {};" },
          );
          const config = JSON.parse(fs.readFileSync(collision.config, "utf8"));
          config.files = ["src/index.ts", "src/index.tsx"];
          fs.writeFileSync(collision.config, JSON.stringify(config));
          const failed = yield* collision.run([]);
          assert.notEqual(failed.status, 0);
          assert.match(failed.stdout, /TS5056/v);
        });
      }),
    ));
  it("rejects excluded declaration sources", (t) =>
    joinTest(
      t,
      runOperation(function* () {
        return yield* lifecycle(t.signal, function* () {
          const p = yield* project(
            t,
            { noEmit: true },
            { "src/included.d.ts": "export declare const included: number;" },
          );
          const result = yield* p.run([], compiler, p.options, [
            "src/included.d.ts",
            "excluded.d.mts",
          ]);
          assert.notEqual(result.status, 0);
          assert.match(result.stderr, /declared source .*excluded.d.mts.*not a compiler input/v);
        });
      }),
    ));
  it("does not satisfy an excluded source through an external package symlink", (t) =>
    joinTest(
      t,
      runOperation(function* () {
        return yield* lifecycle(t.signal, function* () {
          const p = yield* project(
            t,
            { noEmit: true },
            {
              "src/index.ts": "import { value } from 'linked'; export { value };",
              "linked/index.ts": "export const value = 42;",
              "linked/package.json": '{"name":"linked","types":"index.ts"}',
            },
          );
          fs.mkdirSync(path.join(p.directory, "node_modules"));
          fs.symlinkSync(
            path.join(p.directory, "linked"),
            path.join(p.directory, "node_modules/linked"),
            "junction",
          );
          successful(yield* p.run([]));
          const result = yield* p.run([], compiler, p.options, ["src/index.ts", "linked/index.ts"]);
          assert.notEqual(result.status, 0);
          assert.ok(
            result.stderr.includes(`${path.join("linked", "index.ts")} is not a compiler input`),
          );
        });
      }),
    ));
  it("checks invalid noEmit projects and writes validation only on success", (t) =>
    joinTest(
      t,
      runOperation(function* () {
        return yield* lifecycle(t.signal, function* () {
          for (const noCheck of [false, true]) {
            const p = yield* project(
              t,
              { noEmit: true, noCheck },
              { "src/index.ts": 'export const value: number = "wrong";' },
            );
            const result = yield* p.run([]);
            assert.equal(result.status === 0, noCheck, result.stdout + result.stderr);
            assert.equal(fs.existsSync(path.join(p.directory, "validation.check")), noCheck);
            assert.equal(fs.existsSync(path.join(p.directory, "dist/index.js")), false);
            if (!noCheck) {
              assert.match(result.stdout, /TS2322/v);
            }
          }
        });
      }),
    ));
  it("invokes the compiler for declaration-only programs that emit no files", (t) =>
    joinTest(
      t,
      runOperation(function* () {
        return yield* lifecycle(t.signal, function* () {
          const p = yield* project(
            t,
            { rootDir: "." },
            { "src/index.d.ts": "declare const value: number;" },
          );
          const command = path.join(p.directory, "compiler.cjs");
          fs.writeFileSync(
            command,
            `
const fs = require("node:fs");
if (fs.existsSync("invoked")) throw new Error("compiler invoked twice");
if (fs.readdirSync(".").some(name => name.startsWith(".latticebuild-compiler-"))) throw new Error("temporary configuration leaked into compilation");
fs.writeFileSync("invoked", "");
console.log("compiler invoked");
console.log(require("node:path").resolve("src/index.d.ts"));`,
          );
          const result = yield* p.run([], command);
          successful(result);
          assert.equal(result.stdout, "compiler invoked\n");
        });
      }),
    ));
  it("rejects commands whose emission differs from Bazel outputs", (t) =>
    joinTest(
      t,
      runOperation(function* () {
        return yield* lifecycle(t.signal, function* () {
          const p = yield* project(t);
          const command = path.join(p.directory, "compiler.cjs");
          fs.writeFileSync(command, 'console.log("TSFILE: unexpected.js")');
          const result = yield* p.run(["dist/index.js"], command);
          assert.notEqual(result.status, 0);
          assert.match(result.stderr, /selected compiler emitted.*target declares/v);
        });
      }),
    ));
});
