import assert from "node:assert/strict";
import { test } from "node:test";
import { answer } from "./index.js";
test("answer", () => assert.equal(answer, 42));
