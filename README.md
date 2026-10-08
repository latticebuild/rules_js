# rules_js

Bazel rules for JavaScript packages, Node executables and tests, and runtime trees. Dependencies use an installed, hoisted pnpm structure; build actions work in private scratch directories, using copy-on-write file clones where the filesystem supports them.

## Setup

Use Bazel 9.2 with Bzlmod. These source repositories have no registry release yet.
Pin your chosen revision in your root MODULE.bazel:

```starlark
bazel_dep(name = "latticebuild_js", version = "0.0.0")
git_override(
    module_name = "latticebuild_js",
    remote = "https://github.com/latticebuild/rules_js.git",
    commit = "FULL_COMMIT_SHA",
)
```

Replace FULL_COMMIT_SHA with the full commit hash of that revision. Copy the
Latticebuild dependency overrides from [MODULE.bazel](MODULE.bazel) into the
consuming root too; overrides declared by a dependency do not propagate.

Prepare your caller-owned pnpm installation before Bazel reads it:

```sh
pnpm install --frozen-lockfile
```

Expose it through the foundation’s public repository rule:

```starlark
node_modules = use_repo_rule("@latticebuild_js//js:repositories.bzl", "node_modules")
node_modules(name = "npm", package_json = "//:package.json")
```

## Usage

```starlark
node_modules = use_repo_rule("@latticebuild_js//js:repositories.bzl", "node_modules")
node_modules(name = "npm", package_json = "//:package.json")
```

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

For this example, name the package `@example/library` and set `"type": "module"`
in package.json. An `index.mjs` exporting `const value = 42`, a `cli.mjs`
importing and printing that value, and an `index.test.mjs` checking it with
`node:test` give runnable build, CLI and test targets without npm dependencies.

See [docs/usage.md](docs/usage.md) for attributes, required tool inputs and
consumer setup. The public API lives in [js/](js/);
implementation files under its private/ directory are repository-local.

Use pnpm’s hoisted linker in pnpm-workspace.yaml:

```yaml
nodeLinker: hoisted
packages: ["packages/*"]
```

The repository rule observes installations rather than installing packages.
Its version-3 workspace index records command names and exact allocated Bazel
targets, including command/file collisions. Shared providers and adapter support
live in js/providers.bzl and js/support/. Public Go helpers live under
`github.com/latticebuild/rules_js/go/`.

## Compared with Aspect rules_js

Both projects integrate Node.js tooling with Bazel. The main differences are
dependency layout and where tools do their work:

| Concern | Latticebuild rules_js | Aspect rules_js |
| --- | --- | --- |
| Dependency input | Reads an existing pnpm installation, with the hoisted node_modules layout used by this repository. | Translates a pnpm lockfile into Bazel repositories and lazily fetches packages needed by requested targets. |
| Package layout | Observes physical installations, nested versions, workspace links, aliases and peer contexts; declared targets recreate those import locations. | Builds a pnpm-style package store and links node_modules from Bazel targets. Selected packages can be publicly hoisted. |
| Build actions | Creates a private scratch tree from declared inputs, runs the tool there, and publishes declared outputs after success. | Runs tools using its layout in Bazel’s output tree, with sources and dependencies placed there. |

The Aspect column follows its [dependency documentation](https://github.com/aspect-build/rules_js/blob/main/docs/pnpm.md),
[design explanation](https://github.com/aspect-build/rules_js#design), and
[hoisting options](https://github.com/aspect-build/rules_js/blob/main/docs/troubleshooting.md#its-a-plugin).
Aspect supports hoisting; the difference here is using pnpm’s installed hoisted
graph as the input rather than reproducing installation from the lockfile.

Scratch files are writable copies, so a compiler or bundler can create caches
and intermediate files without changing source inputs. The
[file copier](go/filetree/copy.go) tries filesystem cloning: macOS clonefile,
Linux reflinks, and Windows ReFS duplicate extents. Unsupported cloning falls
back to ordinary copying. Neither path shares writable hard links with inputs.
The [action runner](js/private/tools/run-action/run.go) removes its scratch tree
after success or failure and only publishes declared successful outputs.

This approach gives package installation to pnpm: prepare the frozen installation
before Bazel repository discovery. Aspect’s lazy Bazel downloads can avoid a
whole-workspace installation. Hoisting also exposes packages to normal Node
resolution more broadly, so declare dependencies explicitly even when an import
happens to work in the installed tree. Copy-on-write saves physical copying when
supported; it is not required for correctness and is not a performance guarantee.

## Development

Install [Mise](https://mise.jdx.dev/), then prepare this checkout:

```sh
mise trust
mise run bootstrap
hk validate
hk test
hk check --all --slow
bazel build //:artifacts
bazel test //:test
```

Tools and dependency versions are pinned in [mise.toml](mise.toml) and
[MODULE.bazel](MODULE.bazel). CI runs these gates on native Linux, macOS and
Windows runners. Repositories with a race suite also run it on Linux and macOS.
See [docs/development.md](docs/development.md) for owning checks and platform
constraints, and [ARCHITECTURE.md](ARCHITECTURE.md) for implementation decisions.

## License

[Apache License 2.0](LICENSE).
