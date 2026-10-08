import { defineConfig } from "vitest/config";
import { playwright } from "@vitest/browser-playwright";
export default defineConfig({ test: {
  include: ["*.test.mjs"],
  browser: { enabled: true, headless: true, provider: playwright({ launchOptions: { channel: "chromium", env: { ...process.env, ...(process.platform === "linux" ? { TMPDIR: "/tmp" } : {}) } } }), instances: [{ browser: "chromium" }] },
} });
