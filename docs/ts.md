# TypeScript projects and compilation

Use Bazel 9.2 with Bzlmod and a prepared pnpm installation. Pin `latticebuild_js` and its declared dependencies in your root MODULE.bazel. Copy the dependency pins from
[MODULE.bazel](../MODULE.bazel); these projects have no BCR release yet.
Declare your installed npm repository through the foundation
[repository rule](https://github.com/latticebuild/rules_js/blob/main/docs/usage.md).

Declare output-affecting compiler options in `js_tsconfig.compiler_options`
and in the JSON configuration. Extend other `js_tsconfig` or SvelteKit targets
through `extends`; their provider identity comes from rules_js. Pass the
installed compiler's executable as `tsc` and its companion package as
`typescript`. Node wrappers and native compiler scripts both run from declared
inputs. Output directories belong to one target; use different directories for
compilation and bundling in the same Bazel package.

```starlark
load("@latticebuild_js//js:defs.bzl", "js_tsconfig", "js_tsc")
js_tsconfig(name = "config", config = "tsconfig.json", srcs = glob(["src/**/*.ts"]), compiler_options = {"rootDir": "src", "outDir": "compiled"})
js_tsc(name = "compile", config = ":config", tsc = "@npm//node_modules/@typescript/native:tsc", typescript = "@npm//node_modules/typescript")
```

Command labels in these examples match the development installation. For other
installations, use each binding's `binaries` map in the version 3 workspace
index rather than assuming an allocated target name.
