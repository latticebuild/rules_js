# Storybook development servers

Use Bazel 9.2 with Bzlmod and a prepared pnpm installation. Pin `latticebuild_js` and its declared dependencies in your root MODULE.bazel. Copy the dependency pins from
[MODULE.bazel](../MODULE.bazel); these projects have no BCR release yet.
Declare your installed npm repository through the foundation
[repository rule](https://github.com/latticebuild/rules_js/blob/main/docs/usage.md).

Declare the Storybook configuration, story sources, and required packages.
`js_storybook` runs the installed `storybook` command on loopback; `port`
defaults to 6006. The executable is suitable for `bazel run` or `ibazel run`.

```starlark
load("@latticebuild_js//js:defs.bzl", "js_storybook")
js_storybook(name = "server", srcs = glob(["src/**", ".storybook/**"]), deps = ["@npm//node_modules/@storybook/svelte-vite", "@npm//node_modules/svelte"], storybook = "@npm//node_modules/storybook:bin", port = 6006)
```

`package` defaults to package.json beside the calling BUILD file. Use
`config_dir` to select another package-relative configuration directory. The
server disables browser opening and telemetry, binds 127.0.0.1, and requires the
exact requested port. Runtime files must be declared; changes restart through
ibazel. The server test starts a real Svelte story, reads its index, stops the
process tree, and verifies the port can be rebound.

Command labels in these examples match the development installation. For other
installations, use each binding's `binaries` map in the version 3 workspace
index rather than assuming an allocated target name.
