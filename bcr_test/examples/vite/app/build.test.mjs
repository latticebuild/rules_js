import assert from "node:assert/strict";
import { readFileSync, readdirSync } from "node:fs";
import { test } from "node:test";
test("Vite publishes the configured bundle", () => {
  const directory = new URL("./web/", import.meta.url);
  assert.match(readFileSync(new URL("index.html", directory), "utf8"), /assets\//);
  assert.ok(readdirSync(new URL("assets/", directory)).some((name) => name.endsWith(".js")));
});
