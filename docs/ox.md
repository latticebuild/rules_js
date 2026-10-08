# Oxlint and Oxfmt checks and fixes

Use Bazel 9.2 with Bzlmod and a prepared pnpm installation. Pin `latticebuild_js` and its declared dependencies in your root MODULE.bazel. Copy the dependency pins from
[MODULE.bazel](../MODULE.bazel); these projects have no BCR release yet.
Declare your installed npm repository through the foundation
[repository rule](https://github.com/latticebuild/rules_js/blob/main/docs/usage.md).

Load `js_oxlint_test` or `js_oxfmt_test` from `defs.bzl`. Pass the installed
`oxlint` or `oxfmt` executable label and declare source/configuration files in
`srcs`. Test names end in `_test`; each macro also creates the corresponding
`_fix` executable.

```starlark
load("@latticebuild_js//js:defs.bzl", "js_oxlint_test", "js_oxfmt_test")
js_oxlint_test(name = "lint_test", srcs = glob(["src/**"]) + [".oxlintrc.json"], oxlint = "@npm//node_modules/oxlint:bin")
js_oxfmt_test(name = "format_test", srcs = glob(["src/**"]), oxfmt = "@npm//node_modules/oxfmt:bin")
```

`bazel test` reads declared staged files. `bazel run :lint_fix` or
`:format_fix` applies fixes in the invoking workspace. A direct execution of a
fix command without the Bazel run environment fails. Pass package dependencies
through `deps` when the configuration imports them.

Command labels in these examples match the development installation. For other
installations, use each binding's `binaries` map in the version 3 workspace
index rather than assuming an allocated target name.
