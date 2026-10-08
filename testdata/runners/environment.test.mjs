import { execFileSync } from "node:child_process";
import { realpathSync, statSync } from "node:fs";
import { delimiter, dirname, join } from "node:path";

import { call, run } from "effection";
import { expect, test } from "vitest";

// What bound gives a package executable: its package directory in the tree
// as the working directory, Node's directory first on PATH, and a private
// home and temporary directory in the tree.
const tree = join(process.env.BOUND_ROOT, "tree");

test("runs in its package directory in the tree", () =>
  run(() =>
    call(() => {
      expect(realpathSync(process.cwd())).toBe(
        realpathSync(join(tree, "testdata/runners")),
      );
    }),
  ));

test("finds the executable's node first on PATH", () =>
  run(() =>
    call(() => {
      expect(process.env.PATH.split(delimiter)[0]).toBe(dirname(process.execPath));
      const child = execFileSync("node", ["-p", "process.execPath"], { encoding: "utf8" });
      expect(child.trim()).toBe(process.execPath);
    }),
  ));

test("has a private home and temporary directory in the tree", () =>
  run(() =>
    call(() => {
      for (const name of ["HOME", "USERPROFILE"]) {
        expect(process.env[name]).toBe(join(tree, ".home"));
      }
      for (const name of ["TMPDIR", "TMP", "TEMP"]) {
        expect(process.env[name]).toBe(join(tree, ".tmp"));
      }
      expect(statSync(join(tree, ".git")).isDirectory()).toBe(true);
    }),
  ));
