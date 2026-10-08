import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { test } from "node:test";
test("scratch output is published and input is unchanged", () => {
  assert.equal(readFileSync(new URL("./result.txt", import.meta.url), "utf8"), "original|42|release\n");
  assert.equal(readFileSync(new URL("./input.txt", import.meta.url), "utf8"), "original\n");
});
