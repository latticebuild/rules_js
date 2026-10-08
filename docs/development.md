# Development

Install Mise, then run `mise trust` and `mise run bootstrap` in this checkout.
Bootstrap installs the exact pnpm lockfile. Keep development caches and build
outputs on your development volume.

```sh
hk check --all --slow
hk validate
hk test
bazel build //:artifacts
bazel test //:test
```

On Linux and macOS, also run `bazel test //:race_test`. Windows runs the portable
Go suites, pnpm analysis cases, and Node lifecycle/reporting tests. Native CI
uses Ubuntu 24.04, macOS 15, and Windows 2025 with the same pinned tools and gates.

The executable collision fixture in `testdata/compiler` exercises the version 3
npm index with an installed alias and occupied command target names. Test runners
use declared runfiles; install dependencies before running Bazel.

Consumer setup and supported APIs live in [usage.md](usage.md). The layout and
provider decisions live in [ARCHITECTURE.md](../ARCHITECTURE.md).
