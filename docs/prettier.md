# Prettier checks and fixes

Use Bazel 9.2 with Bzlmod and a prepared pnpm installation. Pin `latticebuild_js` and its declared dependencies in your root MODULE.bazel. Copy the dependency pins from
[MODULE.bazel](../MODULE.bazel); these projects have no BCR release yet.
Declare your installed npm repository through the foundation
[repository rule](https://github.com/latticebuild/rules_js/blob/main/docs/usage.md).

`js_prettier_test` takes declared `srcs`, optional `paths`, and a required
`prettier` executable label. The test name ends in `_test`; the macro creates
its `_fix` twin.

```starlark
load("@latticebuild_js//js:defs.bzl", "js_prettier_test")
js_prettier_test(name = "format_test", srcs = glob(["src/**"]), prettier = "@npm//node_modules/prettier:bin")
```

Include configuration files in `srcs` or `data`, and declare imported plugins
in `deps`. Run `bazel test :format_test` for the staged check and
`bazel run :format_fix` to edit the invoking workspace. Fix commands require the
Bazel run environment.

Command labels in these examples match the development installation. For other
installations, use each binding's `binaries` map in the version 3 workspace
index rather than assuming an allocated target name.
