# Knip unused-code checks and fixes

Use Bazel 9.2 with Bzlmod and a prepared pnpm installation. Pin `latticebuild_js` and its declared dependencies in your root MODULE.bazel. Copy the dependency pins from
[MODULE.bazel](../MODULE.bazel); these projects have no BCR release yet.
Declare your installed npm repository through the foundation
[repository rule](https://github.com/latticebuild/rules_js/blob/main/docs/usage.md).

Pass `knip`, the selected `workspace`, and the package's source, manifest, and
configuration files. A name ending in `_test` creates a `_fix` twin unless you
set `production = True`.

```starlark
load("@latticebuild_js//js:defs.bzl", "js_knip_test")
js_knip_test(name = "unused_test", srcs = glob(["src/**"]) + ["package.json", "knip.json"], knip = "@npm//node_modules/knip:bin", workspace = "@example/package")
```

The check treats configuration hints as errors. A production check has no fix
twin because production-only fixes could remove exports used by tests. Run
`bazel run :unused_fix` for default-mode changes in the invoking workspace.
Configurations that import packages must declare them in `deps`. The development
installation pins Knip's workspace patch in pnpm-workspace.yaml.

Command labels in these examples match the development installation. For other
installations, use each binding's `binaries` map in the version 3 workspace
index rather than assuming an allocated target name.
