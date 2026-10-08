import { expect, test } from "vitest";
import { answer } from "./answer.mjs";
test("declared runtime input", () => expect(answer).toBe(42));
