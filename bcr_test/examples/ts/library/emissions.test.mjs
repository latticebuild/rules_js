import assert from "node:assert/strict";
import { existsSync } from "node:fs";
import { test } from "node:test";
import { answer } from "./dist/index.js";
test("compiler publishes executable JS and declarations", () => {
  assert.equal(answer, 42);
  assert.ok(existsSync(new URL("./dist/index.d.ts", import.meta.url)));
});
