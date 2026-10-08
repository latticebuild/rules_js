import assert from "node:assert/strict";
import { test } from "node:test";

import { add } from "./reported.mjs";

test("adds", () => {
  assert.equal(add(1, 2), 3);
});

test("skipped", { skip: "fixture reason" }, () => {});
