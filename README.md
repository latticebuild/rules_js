# rules_js

[![CI](https://github.com/latticebuild/rules_js/actions/workflows/ci.yml/badge.svg)](https://github.com/latticebuild/rules_js/actions/workflows/ci.yml)
[![Bazel](https://img.shields.io/badge/Bazel-9.2.0-43A047?logo=bazel&logoColor=white)](MODULE.bazel)
[![License](https://img.shields.io/badge/License-Apache_2.0-blue.svg)](LICENSE)

Bazel rules for JavaScript packages, Node executables and tests, TypeScript, Vite and Vitest, SvelteKit, Storybook, Oxlint and Oxfmt, Prettier, and Knip. Dependencies use an installed, hoisted pnpm structure; build actions work in private scratch directories, using copy-on-write file clones where the filesystem supports them.

## Setup

Use Bazel 9.2 with Bzlmod. Until the module is registered in the Bazel Central
Registry, pin a source revision in your root MODULE.bazel:

```starlark
bazel_dep(name = "latticebuild_js", version = "0.1.1")
git_override(
    module_name = "latticebuild_js",
    remote = "https://github.com/latticebuild/rules_js.git",
    commit = "FULL_COMMIT_SHA",
)
```

Replace FULL_COMMIT_SHA with the full commit hash of that revision. Also pin any
unregistered Latticebuild modules in the production dependency graph from
[MODULE.bazel](MODULE.bazel); dependency overrides do not propagate. Development
dependencies are ignored when this repository is consumed as a module.

Prepare your caller-owned pnpm installation before Bazel reads it:

```sh
pnpm install --frozen-lockfile
```

Expose it through the public repository rule:

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

All `js_*` rules load from `@latticebuild_js//js:defs.bzl`. Each tool is an explicit label from your own installation.

| Tool or feature | Rules | Guide and example |
| --- | --- | --- |
| Node and packages | `js_package`, `js_binary`, `js_test`, `js_tree` | [Usage](docs/usage.md), [basics](examples/basics/) |
| TypeScript | `js_tsconfig`, `js_tsc` | [Compiler guide](docs/ts.md), [projects](examples/ts/) |
| Vite and Vitest | `js_vite_config`, `js_vite`, `js_vitest` | [Bundler and test guide](docs/vite.md), [node and browser examples](examples/vite/) |
| SvelteKit | `js_svelte_kit`, `js_svelte_kit_write` | [Route and editor guide](docs/svelte.md), [typed routes](examples/svelte/) |
| Storybook | `js_storybook` | [Server guide](docs/storybook.md), [Svelte story](examples/storybook/) |
| Oxlint and Oxfmt | `js_oxlint_test`, `js_oxfmt_test` | [Check and fix guide](docs/ox.md), [checks](examples/ox/) |
| Prettier | `js_prettier_test` | [Formatter guide](docs/prettier.md), [declared plugin](examples/prettier/) |
| Knip | `js_knip_test` | [Unused code guide](docs/knip.md), [workspace checks](examples/knip/) |

The [generated reference](docs/api-rules.md) documents every public rule. Private implementations live under [js/private/](js/private/).

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

| Concern | Latticebuild rules_js | Aspect rules_js 3.5.0 |
| --- | --- | --- |
| Dependency input | Reads an existing pnpm installation, with the hoisted node_modules layout used by this repository. | Translates a pnpm lockfile into Bazel repositories and lazily fetches packages needed by requested targets. |
| Package layout | Observes physical installations, nested versions, workspace links, aliases and peer contexts; declared targets recreate those import locations. | Builds a pnpm-style package store and links node_modules from Bazel targets. Selected packages can be publicly hoisted. |
| Build actions | Creates a private scratch tree from declared inputs, runs the tool there, and publishes declared outputs after success. | Runs tools using its layout in Bazel’s output tree, with sources and dependencies placed there. |
| Node filesystem behavior | Uses native Node filesystem operations in the writable scratch tree; no filesystem patching. | Patches Node’s filesystem API by default on Linux and macOS; its Windows launcher disables these patches. |

The Aspect column follows its [dependency documentation](https://github.com/aspect-build/rules_js/blob/v3.5.0/docs/pnpm.md),
[design explanation](https://github.com/aspect-build/rules_js/tree/v3.5.0#design), and
[hoisting options](https://github.com/aspect-build/rules_js/blob/v3.5.0/docs/troubleshooting.md#its-a-plugin).
Its [filesystem patch defaults](https://github.com/aspect-build/rules_js/blob/v3.5.0/js/private/js_binary.bzl#L149)
have a [Windows launcher exception](https://github.com/aspect-build/rules_js/blob/v3.5.0/js/private/js_binary.bzl#L499).
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

### Action benchmark

Measured on 2026-10-08 at [3cfdea10f885](https://github.com/latticebuild/rules_js/commit/3cfdea10f885b5182d864cf90925107a1032f598) against Aspect rules_js 3.5.0:

| Platform | Application dependencies | Latticebuild median | Aspect median | Ratio (Latticebuild / Aspect) | Paired 95% interval |
| --- | --- | --- | --- | --- | --- |
| Linux x64 | None | 35.5 ms | 41 ms | 0.866× | [0.833, 0.900] |
| Linux x64 | 54 package instances | 338.5 ms | 127 ms | 2.665× | [2.625, 2.717] |
| macOS 27 ARM64 | None | 85.5 ms | 89 ms | 0.961× | [0.863, 1.141] |
| macOS 27 ARM64 | 55 package instances | 1099 ms | 231.5 ms | 4.747× | [4.198, 5.229] |
| Windows x64 | None | 117 ms | 334.5 ms | 0.350× | [0.345, 0.359] |
| Windows x64 | 54 package instances | 1489.5 ms | 549 ms | 2.713× | [2.623, 2.788] |

These are Bazel main-spawn median times from 30 matched pairs per case. Lower ratios favor Latticebuild; the paired bootstrap interval describes uncertainty in that ratio. The macOS no-dependency result overlaps a tie. Many-dependency actions take longer with Latticebuild in this fixture. We accept that overhead to keep writable isolation and Node's native filesystem behavior.

[Complete reports and host details](benchmarks/results/README.md) are checked in. [Native CI evidence](https://github.com/latticebuild/rules_js/actions/runs/37777521987) includes every execution log and dependency inventory. This fixture measures staging and package resolution overhead; compiler and bundler workloads have their own timings.

The [benchmark](benchmarks/main.go) compares the actual scratch runner with
Aspect 3.5.0's `js_binary` and `js_run_binary`. It runs on Linux, macOS 27 and
Windows in CI. Each backend uses Bazel 9.2.0, Node 26.8.2, the same script, and
the same frozen pnpm 12.4.2 lockfile. One case declares no application packages;
the other declares Knip, Prettier, Vite and Vitest and their complete dependencies.
The script resolves and reads package manifests; this measures action overhead
and dependency resolution, rather than compiler or bundler throughput.

Before timing, the benchmark verifies every package instance, dependency and
peer binding, and common payload file's SHA-256. Aspect's normal package
exclusions remain enabled and their file-count difference is reported.
On glibc Linux x64, Aspect also stages four musl-only native leaf packages that
pnpm omits. Their exact versions, platform constraints and payload digests are
checked against the frozen fixture and reported as additional Aspect inputs;
both actions traverse the same ABI-compatible dependency graph.
Both fixtures disable lifecycle scripts. Dependency preparation, first builds
and cached no-op builds are recorded separately from warm action samples.
Neither fixture supplies extra Node flags. Backend defaults, including Aspect's
symlink-main flag and platform-dependent filesystem patch, are retained and recorded separately.

After three warmup pairs, it retains 30 pairs per case, alternates backend order
and supplies a new matching input nonce for each pair. Every sample must contain
one successful, uncached native action with matching output. The headline is
Bazel's main-spawn `totalTime`; execution time, prerequisite actions and command
wall time are also recorded. Qualification requires all correctness, dependency
parity, source identity, native runner and cleanup checks to pass with 30 retained
pairs in each case. The median ratio and its paired bootstrap 95% interval are
reported observations. A slower or inconclusive result does not fail CI: native
Node behavior without filesystem patching is the design priority. The full JSON
execution logs and report are CI artifacts.

Linux qualification requires Bazel's `linux-sandbox`. On Ubuntu 24.04 CI, the
benchmark shares only the pinned Bazel embedded tools between backends and
checks their sandbox executable as the normal runner user. If needed, an
exact-path AppArmor profile grants that executable user-namespace permission,
following [Ubuntu's guidance](https://documentation.ubuntu.com/release-notes/24.04/#unprivileged-user-namespace-restrictions).
Output roots and action caches remain separate. Host preflight diagnostics are
retained with the benchmark evidence; global user-namespace controls stay enabled.

Measure a clean checkout of the source revision, with separate existing work
and new result directories. A detached worktree keeps platform-specific Bazel
lockfile updates in the checkout that builds the benchmark executable:

```sh
git worktree add --detach ../rules-js-benchmark-subject HEAD
bazel run //benchmarks:benchmark -- \
  --source="../rules-js-benchmark-subject" --work-root="/path/to/development-storage" \
  --results="/path/to/new-benchmark-results"
```

In Git Bash on Windows, set `MSYS2_ARG_CONV_EXCL='*'` to preserve Bazel label
arguments.

`--probe` runs one pair per case for harness development and cannot qualify a
release. `--install-base` selects an existing Bazel embedded tool installation,
including the Linux sandbox executable covered by a host profile. The snapshot
above records one complete native CI run; subsequent CI runs retain their own
full evidence.

<details>
<summary>Repository map</summary>

| Area | Location |
| --- | --- |
| Public API | [js/defs.bzl](js/defs.bzl) |
| Implementation | [js/private/](js/private/) |
| Examples | [examples/](examples/) |
| Refusal fixtures | [testdata/](testdata/) |
| Owning checks | [tests/](tests/) |
| Consumer guide | [docs/usage.md](docs/usage.md) |

</details>

## Documentation and examples

See the [generated API reference](docs/README.md) and [runnable examples](examples/README.md).

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

[Sponsor us](https://github.com/mathematic-inc) · [Discuss questions and ideas](https://github.com/latticebuild/rules_js/discussions)

Pull requests are limited to repository collaborators. Use Discussions for bugs,
feature requests and support. Changes merge as squash commits.
