# JavaScript build foundation

The foundation owns the installed npm graph, package layouts, native action and
test runners, and shared provider identities. Language adapters consume these
interfaces so every compiler, framework and checker uses the same
identity, including when consumers rename the module or npm repository.

The public rules in [defs.bzl](js/defs.bzl) describe packages, executable package
scripts, staged trees, and Node tests. [providers.bzl](js/providers.bzl) defines all
shared providers. [support](js/support/BUILD.bazel) exposes the small layout,
action, coverage, execution, and fix primitives used by adapter owners. Tool implementations under js/private/<family> have repository-local visibility.
Selected Go helpers under `go/` are public because native adapters need identical
file copying, process cleanup, reporting, and argument behavior.

The [npm repository rule](js/repositories.bzl) reads a caller-owned package manifest
and its installed pnpm tree. It observes installation symlinks and package bins,
allocates names without colliding with package files or targets, and emits the
exact allocated executable labels in its versioned workspace index. Rendering
uses labels resolved in this module's own repository mapping. A caller may name
its npm repository or this module differently without changing generated loads,
providers, SPDX references, or platform constraints.

Actions construct a fresh declared-input tree, preserve import links, run the
tool there, and publish outputs only after success. Configuration checks remain
validation outputs when a dependency changes between target and execution
configurations. Runtime bundles own their private directories; fix commands
enter the caller's workspace only through the explicit Bazel run environment.
Linux bundled tests return `testing.ExecutionInfo` with `no-sandbox` because
Bound's ancestor-ownership checks cannot identify host root inside a UID
namespace. The execution requirement belongs to test metadata, so it does not
propagate to compilation. Fresh private copies and Bound's checks still apply;
cache and remote eligibility remain available. The public runtime helper exports
`TEST_RUNTIME_ATTRS` so caller-owned tests select the same target-platform contract.
The process helper depends on [graceproc](https://github.com/latticebuild/graceproc)
for bounded descendant cleanup on each native operating system.

All JavaScript rules share one public facade and one Go module. Compiler, bundler,
framework and checker implementations live in separate private family packages.
Consumers select installed tools explicitly; dependencies of this repository's
examples and tests belong to its development installation. Browser payloads come
from [rules_playwright](https://github.com/latticebuild/rules_playwright), a development
dependency for the native examples. The browser module's production closure does
not depend on JavaScript execution.

TypeScript configurations carry inheritance independently of compiler actions;
the compiler validates declared output ownership before publishing. SvelteKit
provides generated route declarations and an explicit editor writer. Vite
reconciles target packages with execution plugins before bundling. Vitest, Node
and Storybook runners supervise native processes and keep reports, caches and
browser profiles in owned locations. Formatters and fix commands enter a caller's
checkout through the explicit Bazel run environment, with staging and path guards.

The graph and layout tests cover rejected inputs, collisions, symlink graphs,
platform selection, and dependency validation. Native tests cover scratch-tree
cleanup and input preservation; Node tests exercise lifecycle, report, filter,
and sharding behavior. External consumers verify repository mapping rather
than relying on the foundation's own apparent name.
