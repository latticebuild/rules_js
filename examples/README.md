# Runnable examples

Prepare the repository with `mise run bootstrap`, then run:

```sh
bazel build //examples:artifacts
bazel test //examples:test
```

| Feature | Source | Command | Expected result |
| --- | --- | --- | --- |
| Package, binary, test and workspace aliases | [basics/BUILD.bazel](basics/BUILD.bazel) | `bazel run //examples/basics:hello` | Prints 42 using an aliased workspace package. |
| Runtime filtering versus full inventories | [runtime_tree/BUILD.bazel](runtime_tree/BUILD.bazel) | `bazel build //examples/runtime_tree:tree //examples/full_tree:tree` | Runtime tree omits declarations/maps; full tree retains them. |
| Custom adapter, providers, scratch actions and declared env | [action/copy.bzl](action/copy.bzl) | `bazel test //examples/action:scratch_test` | Publishes the result while the source input stays unchanged. |
| Hoisted install and npm bin bindings | [../MODULE.bazel](../MODULE.bazel) | `bazel test //tests:index_test` | The explicit installed root generates index v3 and usable npm bin labels. |
| JUnit, LCOV, filters, sharding and cleanup | [../tests/BUILD.bazel](../tests/BUILD.bazel) | `bazel test //tests:reports_test //tests:lifecycle_test` | Existing native cases verify reporting and supervised lifetimes. |

The root artifact/test gates include these examples. Deliberately invalid subjects
remain in test fixtures; their owner tests require the expected refusals. Fix and
editor-write commands modify the invoking checkout only when run explicitly.
Automated mutation cases use disposable invoking workspaces.

## Tool examples

All examples use the same `js/defs.bzl` facade and caller-declared tools. The root
example selectors build and test every feature below.

| Feature | Example guide |
| --- | --- |
| TypeScript inheritance, emissions and noEmit | [TypeScript](ts/README.md) |
| Vite custom output/mode/env; Vitest node, coverage and Chromium | [Vite and Vitest](vite/README.md) |
| SvelteKit route declarations, type checking and editor writer | [SvelteKit](svelte/README.md) |
| Svelte Storybook server, rendering and shutdown | [Storybook](storybook/README.md) |
| Oxlint and Oxfmt checks and guarded fixes | [Ox](ox/README.md) |
| Prettier configuration and Svelte plugin inputs | [Prettier](prettier/README.md) |
| Default and production Knip workspace checks | [Knip](knip/README.md) |
