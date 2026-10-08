import assert from "node:assert/strict";
import { call, run } from "effection";
import { chromium } from "playwright-core";

import { lifecycle, onCleanup } from "./lifecycle.mjs";

run(function* () {
  return yield* lifecycle(undefined, function* () {
    const server = yield* call(() => chromium.launchServer({
      channel: "chromium",
      headless: true,
      // Linux Unix sockets must fit inside Chromium's 108-byte path limit.
      env: { ...process.env, ...(process.platform === "linux" ? { TMPDIR: "/tmp" } : {}) },
    }));
    yield* onCleanup(() => server.kill());
    const browser = yield* call(() => chromium.connect(server.wsEndpoint()));
    yield* onCleanup(() => browser.close());
    const page = yield* call(() => browser.newPage());
    yield* call(() => page.goto(process.argv[2] + "/iframe.html?id=fixture-button--ready&viewMode=story", { waitUntil: "domcontentloaded" }));
    const button = page.getByRole("button", { name: "ready", exact: true });
    yield* call(() => button.waitFor({ state: "visible" }));
    assert.equal(yield* call(() => button.textContent()), "ready");
    yield* call(() => button.click());
  });
}).catch((error) => {
  console.error(error);
  process.exitCode = 1;
});
