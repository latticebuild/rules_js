# Vite builds and Vitest tests

Use Bazel 9.2 with Bzlmod and a prepared pnpm installation. Pin `latticebuild_js` and its declared dependencies in your root MODULE.bazel. Copy the dependency pins from
[MODULE.bazel](../MODULE.bazel); these projects have no BCR release yet.
Declare your installed npm repository through the foundation
[repository rule](https://github.com/latticebuild/rules_js/blob/main/docs/usage.md).

Use `js_vite_config` to declare a configuration and the packages or local files
it imports. Pass `vite` to a build and `vitest` plus `coverage_provider` to a
Vitest test. Those labels come from your installed npm graph.

```starlark
load("@latticebuild_js//js:defs.bzl", "js_vite_config", "js_vite", "js_vitest")
js_vite_config(name = "config", config = "vite.config.ts", deps = ["@npm//node_modules/vite"])
js_vite(name = "build", config = ":config", srcs = ["index.html"] + glob(["src/**"]), vite = "@npm//node_modules/vite:bin")
js_vite_config(name = "test_config", testonly = True, config = "vitest.config.ts", deps = ["@npm//node_modules/vitest"])
js_vitest(name = "test", config = ":test_config", srcs = glob(["src/**/*.test.ts"]), vitest = "@npm//node_modules/vitest:bin", coverage_provider = "@npm//node_modules/@vitest/coverage-v8")
```

Vitest writes Bazel JUnit and LCOV reports, including failure and sharding
results. Include runtime files in `data`, package targets in `deps`, and paths
in `env_paths` when they must follow the staged tree. `browser` accepts a
checksum-pinned Chromium payload from rules_playwright. It owns
`PLAYWRIGHT_BROWSERS_PATH`; remove competing declarations of that variable.
Native payloads keep their runfiles locations while the runner creates private
browser validation metadata. Execution uses no browser download cache.

Command labels in these examples match the development installation. For other
installations, use each binding's `binaries` map in the version 3 workspace
index rather than assuming an allocated target name.

On Linux, Chromium creates Unix sockets beneath TMPDIR. If your Bazel temporary
path is long, give the browser launch a short temporary root:

```javascript
const launchOptions = {
  env: { ...process.env, ...(process.platform === "linux" ? { TMPDIR: "/tmp" } : {}) },
};
```

Playwright creates a private profile there and removes it when the browser is
closed. Workspace actions continue to use their own scratch directories.
