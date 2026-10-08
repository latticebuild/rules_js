import assert from "node:assert/strict";
import {spawnSync} from "node:child_process";
import {readFileSync} from "node:fs";
import {test} from "node:test";
import {fileURLToPath} from "node:url";

test("the index preserves an aliased command whose files occupy both target names", () => {
  const index = JSON.parse(readFileSync(new URL("../workspace.json", import.meta.url), "utf8"));
  assert.equal(index.version, 3);
  const binding = index.packages[""].bindings["@fixture/compiler"];
  assert.equal(binding.name, "@fixture/compiler");
  assert.equal(binding.label, "@pnpm//node_modules/@fixture/compiler");
  assert.deepEqual(binding.binaries, {tsc: "@pnpm//node_modules/@fixture/compiler:bin_"});
  assert.ok(binding.platforms.every((label) => label.includes("//js/platforms:") && !label.startsWith("@pnpm")));
  const suffix = process.platform === "win32" ? ".exe" : "";
  const command = fileURLToPath(new URL(`../node_modules/@fixture/compiler/bin_${suffix}`, import.meta.url));
  const result = spawnSync(command, [], {encoding:"utf8"});
  assert.equal(result.status, 0, result.stderr);
  assert.equal(result.stdout.trim(), "allocated compiler executed");
});
