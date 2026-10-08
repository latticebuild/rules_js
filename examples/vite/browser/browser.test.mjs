import { call, ensure, run } from "effection";
import { expect, test } from "vitest";

test("renders in the checksum-pinned Chromium payload", () => run(function* () {
  yield* ensure(() => { document.body.textContent = ""; });
  yield* call(() => { document.body.innerHTML = '<button aria-label="fixture">ready</button>'; });
  expect(document.querySelector("button")?.textContent).toBe("ready");
  expect(navigator.userAgent).toContain("Chrome/");
}));
