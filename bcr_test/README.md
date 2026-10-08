# Registry consumer

This separate Bazel module consumes the public `latticebuild_js` API under the
`@subject` alias. Its own-module override resolves to the extracted parent
archive; other Latticebuild modules resolve through the registry.

The BCR presubmit builds `//:artifacts` and runs `//:test`.

Prepare the caller's pinned hoisted tools before Bazel analysis:

```sh
npm exec --yes --package=@pnpm/exe@12.4.2 -- pnpm install --frozen-lockfile
bazel build //:artifacts
bazel test //:test
```

The consumer includes Node/package aliases, runtime/full trees, public scratch-action
composition, TypeScript inheritance/declarations/noEmit, Vite outputs/mode/env,
caller-owned test composition through `TEST_RUNTIME_ATTRS` and `node_executable`,
Vitest, SvelteKit generation/write binary, Storybook server binary, and Ox,
Prettier/Svelte and Knip checks/fix binaries. Native browser/server execution and
disposable writer/fixer cases also run in the owning repository's CI.
