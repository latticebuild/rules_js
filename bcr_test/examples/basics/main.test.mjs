import assert from "node:assert/strict";
import { test } from "node:test";
import { answer } from "@example/answer";

test("workspace alias resolves", () => assert.equal(answer, 42));
