import { expect, test } from "vitest";

import { add } from "./reported.mjs";

test("adds", () => {
  expect(add(1, 2)).toBe(3);
});
