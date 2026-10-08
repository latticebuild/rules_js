# Using rules_js

Use Bazel 9.2 with Bzlmod. Add `latticebuild_js` and pin this repository with
`git_override`; root modules must also pin `latticebuild_graceproc` to a commit
from [graceproc](https://github.com/latticebuild/graceproc). These modules have
no BCR release yet. Select an immutable commit rather than a moving branch.

Prepare your own pnpm installation before Bazel reads it:

```sh
pnpm install --frozen-lockfile
```

Declare the repository from your own package manifest in MODULE.bazel:

```starlark
node_modules = use_repo_rule("@latticebuild_js//js:repositories.bzl", "node_modules")
node_modules(name = "npm", package_json = "//:package.json")
```

The repository reads the installed tree, including peer contexts, aliases,
optional platform packages, package metadata, and command scripts. It does not
install packages. Keep package.json, pnpm-lock.yaml, and pnpm-workspace.yaml
under your control. Use the Node and bound toolchains declared in this module;
a root module choosing different versions owns that choice.

Load rules through the public facade:

```starlark
load("@latticebuild_js//js:defs.bzl", "js_binary", "js_package", "js_test", "js_tree")

js_package(
    name = "library",
    package_name = "@example/library",
    srcs = ["index.mjs"],
    package = "package.json",
)
js_binary(name = "cli", bin = "cli.mjs", data = [":library"])
js_test(name = "test", srcs = ["index.test.mjs"], data = [":library"])
js_tree(name = "runtime", deps = [":library"])
```

`js_package` reads package.json beside the calling BUILD file unless you pass
`package`. `aliases` maps additional import names to package targets. Include
scripts and fixtures in `data`; executables stage only declared inputs.

`js_tree` writes one `app/` tree in its Bazel package. It drops declarations,
source maps, build info, and type-only packages by default. Set
`runtime_only = False` for tools that need those files. Give other generators
in that package distinct output directories.

The generated `@npm//:workspace.json` index has version 3. Each package binding
contains a `binaries` map from the published command name to its allocated
Bazel executable label. Read that map when composing an adapter. A package
with one command often has `:bin`, but a file or target collision can allocate
another name. The index records the label that works.

[providers.bzl](../js/providers.bzl) owns `JsPackageInfo`, `JsBinaryInfo`,
`JsTsconfigInfo`, and `JsViteConfigInfo`. Adapter authors may load the public
`js/support/*.bzl` facades and the `//js/support` executable aliases. Import Go helpers
through `github.com/latticebuild/rules_js/go/...`. Keep private implementation
imports inside this repository.
