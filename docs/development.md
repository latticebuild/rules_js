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
uses Ubuntu 24.04, macOS 27, and Windows 2025 with the same pinned tools and gates.

The separate BCR consumer runs on all three platforms, including its public
runtime-helper test under a renamed module. Linux qualification records actual
namespace-sandboxed compilation and scratch actions, then executes all bundled
tests with their declared `no-sandbox` requirement. Private runtime copies and
ownership checks stay enabled. The gate retains uncached first-attempt test and
action logs, source hashes, and owned Bazel shutdown results.

The executable collision fixture in `testdata/compiler` exercises the version 3
npm index with an installed alias and occupied command target names. Test runners
use declared runfiles; install dependencies before running Bazel.

Consumer setup and supported APIs live in [usage.md](usage.md). The layout and
provider decisions live in [ARCHITECTURE.md](../ARCHITECTURE.md).

On Windows, use a temporary root with its canonical long path. Vite rejects 8.3
aliases in served paths. CI selects LOCALAPPDATA/Temp/latticebuild before dependency preparation and
forwards TMP/TEMP through Bazel tests; private runtime trees remain inside that
root. Keep this path out of installed source and dependency directories.

CI uses a short Bazel output root on Windows (`D:/b`) so native linkers can
open deeply nested runfiles. Locally, select a short writable root with
`bazel --output_user_root=C:/b test //:test` when needed. Documentation and
example scripts accept the same root through BAZEL_OUTPUT_USER_ROOT.
