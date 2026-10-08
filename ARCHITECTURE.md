# JavaScript build foundation

The foundation owns the installed npm graph, package layouts, native action and
test runners, and shared provider identities. Language adapters consume these
interfaces so a TypeScript configuration or a Vite configuration keeps the same
identity across independently named Bazel repositories.

The public rules in [defs.bzl](defs.bzl) describe packages, executable package
scripts, staged trees, and Node tests. [providers.bzl](providers.bzl) defines all
shared providers. [support](support/BUILD.bazel) exposes the small layout,
action, coverage, execution, and fix primitives used by adapter owners. Private
implementations have repository-local visibility; adapters cannot import them.
Selected Go helpers under `go/` are public because native adapters need identical
file copying, process cleanup, reporting, and argument behavior.

The [npm repository rule](repositories.bzl) reads a caller-owned package manifest
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
The process helper depends on [graceproc](https://github.com/latticebuild/graceproc)
for bounded descendant cleanup on each native operating system.

Keeping all tool-specific adapters outside this module prevents a foundation
consumer from acquiring TypeScript, Vite, Svelte, or lint tool dependencies.
Consumers supply their installed tools explicitly to those adapters. Splitting
the provider definitions would break identity checks, so they remain here.

The graph and layout tests cover rejected inputs, collisions, symlink graphs,
platform selection, and dependency validation. Native tests cover scratch-tree
cleanup and input preservation; Node tests exercise lifecycle, report, filter,
and sharding behavior. External consumers verify repository mapping rather
than relying on the foundation's own apparent name.
