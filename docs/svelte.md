# SvelteKit configuration and route types

Use Bazel 9.2 with Bzlmod and a prepared pnpm installation. Pin `latticebuild_js` and its declared dependencies in your root MODULE.bazel. Copy the dependency pins from
[MODULE.bazel](../MODULE.bazel); these projects have no BCR release yet.
Declare your installed npm repository through the foundation
[repository rule](https://github.com/latticebuild/rules_js/blob/main/docs/usage.md).

Declare the Vite configuration containing `sveltekit()`, the package manifest,
routes and static inputs, and the configuration's imported packages. Supply
`kit` with the installed `svelte-kit` command and `typescript` with its package.

```starlark
load("@latticebuild_js//js:defs.bzl", "js_svelte_kit")
js_svelte_kit(name = "kit", config = "vite.config.js", srcs = glob(["src/**"]), kit = "@npm//node_modules/@sveltejs/kit:svelte-kit", typescript = "@npm//node_modules/typescript", deps = ["@npm//node_modules/@sveltejs/kit", "@npm//node_modules/svelte"], tool_deps = ["@npm//node_modules/@sveltejs/vite-plugin-svelte"])
```

The target provides the foundation's `JsTsconfigInfo`. A js_tsconfig project may
extend it when its JSON config extends `$app/tsconfig`. Building writes declared
outputs. `bazel run :kit_write` copies those generated outputs into the owning
workspace for editors. The write command requires a Bazel run workspace and
refuses external-repository targets. Keep secrets and competing configuration
files out of `srcs`; the rule rejects Svelte configs and duplicate Vite configs.

Command labels in these examples match the development installation. For other
installations, use each binding's `binaries` map in the version 3 workspace
index rather than assuming an allocated target name.
