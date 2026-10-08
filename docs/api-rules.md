<!-- Generated with Stardoc: http://skydoc.bazel.build -->

JavaScript package, execution, compiler, framework and check rules.

<a id="js_binary"></a>

## js_binary

<pre>
load("@latticebuild_js//js:defs.bzl", "js_binary")

js_binary(<a href="#js_binary-name">name</a>, <a href="#js_binary-data">data</a>, <a href="#js_binary-bin">bin</a>)
</pre>

A package.json bin script.

bazel run, a test, or an action that names it as a tool runs its
executable: one file holding node, the script and its packages, which runs
anywhere. The build rules here read JsBinaryInfo instead and run node on
the script in the tree they lay out, so the tool and the package's own
plugins resolve one node_modules/.

**ATTRIBUTES**


| Name  | Description | Type | Mandatory | Default |
| :------------- | :------------- | :------------- | :------------- | :------------- |
| <a id="js_binary-name"></a>name |  A unique name for this target.   | <a href="https://bazel.build/concepts/labels#target-names">Name</a> | required |  |
| <a id="js_binary-data"></a>data |  What the script needs at run time: its js_package, which brings its dependencies.   | <a href="https://bazel.build/concepts/labels">List of labels</a> | optional |  `[]`  |
| <a id="js_binary-bin"></a>bin |  The script, a file of this package.   | <a href="https://bazel.build/concepts/labels">Label</a> | required |  |


<a id="js_svelte_kit_write"></a>

## js_svelte_kit_write

<pre>
load("@latticebuild_js//js:defs.bzl", "js_svelte_kit_write")

js_svelte_kit_write(<a href="#js_svelte_kit_write-name">name</a>, <a href="#js_svelte_kit_write-src">src</a>)
</pre>

Writes generated SvelteKit configuration and types to the owning workspace package.

**ATTRIBUTES**


| Name  | Description | Type | Mandatory | Default |
| :------------- | :------------- | :------------- | :------------- | :------------- |
| <a id="js_svelte_kit_write-name"></a>name |  A unique name for this target.   | <a href="https://bazel.build/concepts/labels#target-names">Name</a> | required |  |
| <a id="js_svelte_kit_write-src"></a>src |  -   | <a href="https://bazel.build/concepts/labels">Label</a> | required |  |


<a id="js_test"></a>

## js_test

<pre>
load("@latticebuild_js//js:defs.bzl", "js_test")

js_test(<a href="#js_test-name">name</a>, <a href="#js_test-srcs">srcs</a>, <a href="#js_test-data">data</a>)
</pre>

Runs srcs with node:test, writing Bazel's JUnit and LCOV reports.

The executable lays out the declared files with package import links.
Relative imports, npm dependencies, and workspace packages resolve in that
tree.

**ATTRIBUTES**


| Name  | Description | Type | Mandatory | Default |
| :------------- | :------------- | :------------- | :------------- | :------------- |
| <a id="js_test-name"></a>name |  A unique name for this target.   | <a href="https://bazel.build/concepts/labels#target-names">Name</a> | required |  |
| <a id="js_test-srcs"></a>srcs |  The test files.   | <a href="https://bazel.build/concepts/labels">List of labels</a> | required |  |
| <a id="js_test-data"></a>data |  What the tests read or import: the files under test, fixtures, and js_package targets.   | <a href="https://bazel.build/concepts/labels">List of labels</a> | optional |  `[]`  |


<a id="js_tree"></a>

## js_tree

<pre>
load("@latticebuild_js//js:defs.bzl", "js_tree")

js_tree(<a href="#js_tree-name">name</a>, <a href="#js_tree-deps">deps</a>, <a href="#js_tree-runtime_only">runtime_only</a>)
</pre>

The part of a monorepo checkout that running a JavaScript program needs.

A local install and build would leave the same layout: each js_package in
deps and their closure lands at its repository path (workspace packages,
with only the files Node loads by default) or its node_modules/ path (installed npm
packages: those npm installs on the target platform, which the generated
edges select, excluding the types-only @types/* by default); workspace packages' links
become relative node_modules links; each other target's files land at their
package paths. Express a program's own package as a js_package whose deps
give its workspace links. The tree is declared under app/ in this package,
for image_layer(srcs = {"/": ":tree"}), so a package holds one js_tree and
no other target named app. An image for a musl distribution builds for a
platform that names //js/platforms:musl. Set runtime_only = False
when the program typechecks or otherwise reads declarations or source maps.

**ATTRIBUTES**


| Name  | Description | Type | Mandatory | Default |
| :------------- | :------------- | :------------- | :------------- | :------------- |
| <a id="js_tree-name"></a>name |  A unique name for this target.   | <a href="https://bazel.build/concepts/labels#target-names">Name</a> | required |  |
| <a id="js_tree-deps"></a>deps |  The js_package targets the program needs, and plain file targets to place at their package paths.   | <a href="https://bazel.build/concepts/labels">List of labels</a> | optional |  `[]`  |
| <a id="js_tree-runtime_only"></a>runtime_only |  Omit declarations, source maps, build info, and @types packages; disable for programs that typecheck.   | Boolean | optional |  `True`  |


<a id="js_tsc"></a>

## js_tsc

<pre>
load("@latticebuild_js//js:defs.bzl", "js_tsc")

js_tsc(<a href="#js_tsc-name">name</a>, <a href="#js_tsc-config">config</a>, <a href="#js_tsc-tsc">tsc</a>, <a href="#js_tsc-typescript">typescript</a>)
</pre>

Compiles or checks a js_tsconfig project during builds, publishing only standard compiler emissions.

**ATTRIBUTES**


| Name  | Description | Type | Mandatory | Default |
| :------------- | :------------- | :------------- | :------------- | :------------- |
| <a id="js_tsc-name"></a>name |  A unique name for this target.   | <a href="https://bazel.build/concepts/labels#target-names">Name</a> | required |  |
| <a id="js_tsc-config"></a>config |  -   | <a href="https://bazel.build/concepts/labels">Label</a> | required |  |
| <a id="js_tsc-tsc"></a>tsc |  -   | <a href="https://bazel.build/concepts/labels">Label</a> | required |  |
| <a id="js_tsc-typescript"></a>typescript |  -   | <a href="https://bazel.build/concepts/labels">Label</a> | required |  |


<a id="js_vite"></a>

## js_vite

<pre>
load("@latticebuild_js//js:defs.bzl", "js_vite")

js_vite(<a href="#js_vite-name">name</a>, <a href="#js_vite-deps">deps</a>, <a href="#js_vite-srcs">srcs</a>, <a href="#js_vite-out">out</a>, <a href="#js_vite-aliases">aliases</a>, <a href="#js_vite-config">config</a>, <a href="#js_vite-env">env</a>, <a href="#js_vite-mode">mode</a>, <a href="#js_vite-tool_data">tool_data</a>, <a href="#js_vite-tool_deps">tool_deps</a>, <a href="#js_vite-vite">vite</a>)
</pre>

Runs vite build and publishes the directory selected by its configuration.

**ATTRIBUTES**


| Name  | Description | Type | Mandatory | Default |
| :------------- | :------------- | :------------- | :------------- | :------------- |
| <a id="js_vite-name"></a>name |  A unique name for this target.   | <a href="https://bazel.build/concepts/labels#target-names">Name</a> | required |  |
| <a id="js_vite-deps"></a>deps |  -   | <a href="https://bazel.build/concepts/labels">List of labels</a> | optional |  `[]`  |
| <a id="js_vite-srcs"></a>srcs |  File inputs in the target configuration: application files, project file inventories and configuration files, without implicit package bindings.   | <a href="https://bazel.build/concepts/labels">List of labels</a> | optional |  `[]`  |
| <a id="js_vite-out"></a>out |  -   | String | optional |  `"dist"`  |
| <a id="js_vite-aliases"></a>aliases |  -   | Dictionary: String -> Label | optional |  `{}`  |
| <a id="js_vite-config"></a>config |  -   | <a href="https://bazel.build/concepts/labels">Label</a> | required |  |
| <a id="js_vite-env"></a>env |  -   | <a href="https://bazel.build/rules/lib/core/dict">Dictionary: String -> String</a> | optional |  `{}`  |
| <a id="js_vite-mode"></a>mode |  -   | String | optional |  `"production"`  |
| <a id="js_vite-tool_data"></a>tool_data |  Additional file inputs in the execution configuration, such as executables or generated files the build tooling reads, without package bindings.   | <a href="https://bazel.build/concepts/labels">List of labels</a> | optional |  `[]`  |
| <a id="js_vite-tool_deps"></a>tool_deps |  -   | <a href="https://bazel.build/concepts/labels">List of labels</a> | optional |  `[]`  |
| <a id="js_vite-vite"></a>vite |  -   | <a href="https://bazel.build/concepts/labels">Label</a> | required |  |


<a id="js_vitest"></a>

## js_vitest

<pre>
load("@latticebuild_js//js:defs.bzl", "js_vitest")

js_vitest(<a href="#js_vitest-name">name</a>, <a href="#js_vitest-deps">deps</a>, <a href="#js_vitest-srcs">srcs</a>, <a href="#js_vitest-data">data</a>, <a href="#js_vitest-aliases">aliases</a>, <a href="#js_vitest-browser">browser</a>, <a href="#js_vitest-config">config</a>, <a href="#js_vitest-coverage_provider">coverage_provider</a>, <a href="#js_vitest-data_paths">data_paths</a>, <a href="#js_vitest-env">env</a>,
          <a href="#js_vitest-env_inherit">env_inherit</a>, <a href="#js_vitest-env_paths">env_paths</a>, <a href="#js_vitest-mode">mode</a>, <a href="#js_vitest-vitest">vitest</a>)
</pre>

Creates a Vitest test with Bazel arguments, filtering, sharding and reporting.

**ATTRIBUTES**


| Name  | Description | Type | Mandatory | Default |
| :------------- | :------------- | :------------- | :------------- | :------------- |
| <a id="js_vitest-name"></a>name |  A unique name for this target.   | <a href="https://bazel.build/concepts/labels#target-names">Name</a> | required |  |
| <a id="js_vitest-deps"></a>deps |  Imported files, packages and configuration providers.   | <a href="https://bazel.build/concepts/labels">List of labels</a> | optional |  `[]`  |
| <a id="js_vitest-srcs"></a>srcs |  Imported packages and files traced from the configuration.   | <a href="https://bazel.build/concepts/labels">List of labels</a> | optional |  `[]`  |
| <a id="js_vitest-data"></a>data |  Declared runtime packages, sources, checked configs and build prerequisites.   | <a href="https://bazel.build/concepts/labels">List of labels</a> | optional |  `[]`  |
| <a id="js_vitest-aliases"></a>aliases |  -   | Dictionary: String -> Label | optional |  `{}`  |
| <a id="js_vitest-browser"></a>browser |  One checksum-pinned Playwright browser installation, whose native payload stays at its canonical runfiles location.   | <a href="https://bazel.build/concepts/labels">Label</a> | optional |  `None`  |
| <a id="js_vitest-config"></a>config |  The js_vite_config target Vitest evaluates.   | <a href="https://bazel.build/concepts/labels">Label</a> | required |  |
| <a id="js_vitest-coverage_provider"></a>coverage_provider |  The coverage provider package Vitest loads under `bazel coverage`, staged with Vitest itself.   | <a href="https://bazel.build/concepts/labels">Label</a> | required |  |
| <a id="js_vitest-data_paths"></a>data_paths |  Single file or directory targets mapped to explicit tree paths.   | <a href="https://bazel.build/rules/lib/core/dict">Dictionary: Label -> String</a> | optional |  `{}`  |
| <a id="js_vitest-env"></a>env |  Runtime environment overrides.   | <a href="https://bazel.build/rules/lib/core/dict">Dictionary: String -> String</a> | optional |  `{}`  |
| <a id="js_vitest-env_inherit"></a>env_inherit |  Caller environment names the test inherits: they keep the caller's value, and the test gets no private default for them, such as its home.   | List of strings | optional |  `[]`  |
| <a id="js_vitest-env_paths"></a>env_paths |  Environment values resolved as paths inside the tree.   | <a href="https://bazel.build/rules/lib/core/dict">Dictionary: String -> String</a> | optional |  `{}`  |
| <a id="js_vitest-mode"></a>mode |  -   | String | optional |  `"test"`  |
| <a id="js_vitest-vitest"></a>vitest |  -   | <a href="https://bazel.build/concepts/labels">Label</a> | required |  |


<a id="js_knip_test"></a>

## js_knip_test

<pre>
load("@latticebuild_js//js:defs.bzl", "js_knip_test")

js_knip_test(<a href="#js_knip_test-name">name</a>, <a href="#js_knip_test-workspace">workspace</a>, <a href="#js_knip_test-production">production</a>, <a href="#js_knip_test-knip">knip</a>, <a href="#js_knip_test-env">env</a>, <a href="#js_knip_test-kwargs">**kwargs</a>)
</pre>

Runs Knip on one workspace and, unless `production`, declares `<name minus _test>_fix`.

`bazel run` of the twin applies Knip's default-mode fixes in the invoking
workspace. A production test has no twin: production-mode fixes would
remove exports that only tests import.


**PARAMETERS**


| Name  | Description | Default Value |
| :------------- | :------------- | :------------- |
| <a id="js_knip_test-name"></a>name |  Test name, ending in `_test`.   |  none |
| <a id="js_knip_test-workspace"></a>workspace |  The workspace Knip selects: the package name, or "." when unnamed.   |  `None` |
| <a id="js_knip_test-production"></a>production |  Whether Knip analyzes production code only.   |  `False` |
| <a id="js_knip_test-knip"></a>knip |  The Knip command.   |  `None` |
| <a id="js_knip_test-env"></a>env |  Runtime environment overrides, shared with the twin.   |  `None` |
| <a id="js_knip_test-kwargs"></a>kwargs |  Staged inputs and standard test attributes; the twin shares the test's visibility.   |  none |


<a id="js_oxfmt_test"></a>

## js_oxfmt_test

<pre>
load("@latticebuild_js//js:defs.bzl", "js_oxfmt_test")

js_oxfmt_test(<a href="#js_oxfmt_test-name">name</a>, <a href="#js_oxfmt_test-paths">paths</a>, <a href="#js_oxfmt_test-oxfmt">oxfmt</a>, <a href="#js_oxfmt_test-env">env</a>, <a href="#js_oxfmt_test-kwargs">**kwargs</a>)
</pre>

Checks formatting with Oxfmt and declares `<name minus _test>_fix`.

`bazel run` of the twin formats the same paths in the invoking workspace.


**PARAMETERS**


| Name  | Description | Default Value |
| :------------- | :------------- | :------------- |
| <a id="js_oxfmt_test-name"></a>name |  Test name, ending in `_test`.   |  none |
| <a id="js_oxfmt_test-paths"></a>paths |  Operands relative to the package; defaults to the whole package.   |  `None` |
| <a id="js_oxfmt_test-oxfmt"></a>oxfmt |  The Oxfmt command.   |  `None` |
| <a id="js_oxfmt_test-env"></a>env |  Runtime environment overrides, shared with the twin.   |  `None` |
| <a id="js_oxfmt_test-kwargs"></a>kwargs |  Staged inputs and standard test attributes; the twin shares the test's visibility.   |  none |


<a id="js_oxlint_test"></a>

## js_oxlint_test

<pre>
load("@latticebuild_js//js:defs.bzl", "js_oxlint_test")

js_oxlint_test(<a href="#js_oxlint_test-name">name</a>, <a href="#js_oxlint_test-paths">paths</a>, <a href="#js_oxlint_test-oxlint">oxlint</a>, <a href="#js_oxlint_test-env">env</a>, <a href="#js_oxlint_test-kwargs">**kwargs</a>)
</pre>

Lints with Oxlint, failing on warnings, and declares `<name minus _test>_fix`.

`bazel run` of the twin applies Oxlint's fixes to the same paths in the
invoking workspace.


**PARAMETERS**


| Name  | Description | Default Value |
| :------------- | :------------- | :------------- |
| <a id="js_oxlint_test-name"></a>name |  Test name, ending in `_test`.   |  none |
| <a id="js_oxlint_test-paths"></a>paths |  Operands relative to the package; defaults to the whole package.   |  `None` |
| <a id="js_oxlint_test-oxlint"></a>oxlint |  The Oxlint command.   |  `None` |
| <a id="js_oxlint_test-env"></a>env |  Runtime environment overrides, shared with the twin.   |  `None` |
| <a id="js_oxlint_test-kwargs"></a>kwargs |  Staged inputs and standard test attributes; the twin shares the test's visibility.   |  none |


<a id="js_package"></a>

## js_package

<pre>
load("@latticebuild_js//js:defs.bzl", "js_package")

js_package(<a href="#js_package-name">name</a>, <a href="#js_package-package_name">package_name</a>, <a href="#js_package-package">package</a>, <a href="#js_package-kwargs">**kwargs</a>)
</pre>

Declares a package's files and import bindings at its repository location.

**PARAMETERS**


| Name  | Description | Default Value |
| :------------- | :------------- | :------------- |
| <a id="js_package-name"></a>name |  Bazel target name.   |  none |
| <a id="js_package-package_name"></a>package_name |  npm import name for this package.   |  none |
| <a id="js_package-package"></a>package |  Package manifest in the calling BUILD package.   |  `"package.json"` |
| <a id="js_package-kwargs"></a>kwargs |  srcs, deps, aliases, observed installs and standard rule attributes.   |  none |


<a id="js_prettier_test"></a>

## js_prettier_test

<pre>
load("@latticebuild_js//js:defs.bzl", "js_prettier_test")

js_prettier_test(<a href="#js_prettier_test-name">name</a>, <a href="#js_prettier_test-paths">paths</a>, <a href="#js_prettier_test-prettier">prettier</a>, <a href="#js_prettier_test-env">env</a>, <a href="#js_prettier_test-kwargs">**kwargs</a>)
</pre>

Checks formatting with Prettier and declares `<name minus _test>_fix`.

`bazel run` of the twin formats the same paths in the invoking workspace.
Plugins that a Prettier configuration names are staged through `data`.


**PARAMETERS**


| Name  | Description | Default Value |
| :------------- | :------------- | :------------- |
| <a id="js_prettier_test-name"></a>name |  Test name, ending in `_test`.   |  none |
| <a id="js_prettier_test-paths"></a>paths |  Operands relative to the package; defaults to the whole package.   |  `None` |
| <a id="js_prettier_test-prettier"></a>prettier |  The Prettier command.   |  `None` |
| <a id="js_prettier_test-env"></a>env |  Runtime environment overrides, shared with the twin.   |  `None` |
| <a id="js_prettier_test-kwargs"></a>kwargs |  Staged inputs and standard test attributes; the twin shares the test's visibility.   |  none |


<a id="js_storybook"></a>

## js_storybook

<pre>
load("@latticebuild_js//js:defs.bzl", "js_storybook")

js_storybook(<a href="#js_storybook-name">name</a>, <a href="#js_storybook-package">package</a>, <a href="#js_storybook-kwargs">**kwargs</a>)
</pre>

Starts a loopback Storybook development server with declared package inputs.

**PARAMETERS**


| Name  | Description | Default Value |
| :------------- | :------------- | :------------- |
| <a id="js_storybook-name"></a>name |  Executable target name.   |  none |
| <a id="js_storybook-package"></a>package |  Package manifest in the calling BUILD package.   |  `"package.json"` |
| <a id="js_storybook-kwargs"></a>kwargs |  srcs, deps, aliases, config_dir, port, storybook and runtime attributes.   |  none |


<a id="js_svelte_kit"></a>

## js_svelte_kit

<pre>
load("@latticebuild_js//js:defs.bzl", "js_svelte_kit")

js_svelte_kit(<a href="#js_svelte_kit-name">name</a>, <a href="#js_svelte_kit-config">config</a>, <a href="#js_svelte_kit-package">package</a>, <a href="#js_svelte_kit-compiler_options">compiler_options</a>, <a href="#js_svelte_kit-kwargs">**kwargs</a>)
</pre>

Generates SvelteKit's package-local configuration and route declarations.

A js_tsconfig whose configuration extends `$app/tsconfig` names
this target in `extends` and inherits everything sync writes: the
configuration, its declarations and the route types.
`bazel run <name>_write` refreshes those outputs in the checkout for
editors and Gazelle; building either target never edits the checkout.


**PARAMETERS**


| Name  | Description | Default Value |
| :------------- | :------------- | :------------- |
| <a id="js_svelte_kit-name"></a>name |  Target name, `kit` by convention.   |  none |
| <a id="js_svelte_kit-config"></a>config |  The package's Vite configuration containing the SvelteKit plugin.   |  `"vite.config.ts"` |
| <a id="js_svelte_kit-package"></a>package |  The package manifest, which makes the configuration an ES module.   |  `"package.json"` |
| <a id="js_svelte_kit-compiler_options"></a>compiler_options |  The generated configuration's output-affecting options.   |  `{}` |
| <a id="js_svelte_kit-kwargs"></a>kwargs |  srcs, deps, tool_deps, kit, typescript and standard Bazel attributes.   |  none |


<a id="js_tsconfig"></a>

## js_tsconfig

<pre>
load("@latticebuild_js//js:defs.bzl", "js_tsconfig")

js_tsconfig(<a href="#js_tsconfig-name">name</a>, <a href="#js_tsconfig-config">config</a>, <a href="#js_tsconfig-srcs">srcs</a>, <a href="#js_tsconfig-compiler_options">compiler_options</a>, <a href="#js_tsconfig-package">package</a>, <a href="#js_tsconfig-kwargs">**kwargs</a>)
</pre>

Declares a project's inputs, the configurations it extends and the compiler options it sets.

**PARAMETERS**


| Name  | Description | Default Value |
| :------------- | :------------- | :------------- |
| <a id="js_tsconfig-name"></a>name |  Configuration target name.   |  none |
| <a id="js_tsconfig-config"></a>config |  Original TypeScript JSON configuration.   |  none |
| <a id="js_tsconfig-srcs"></a>srcs |  Complete project source inventory.   |  `[]` |
| <a id="js_tsconfig-compiler_options"></a>compiler_options |  What config contributes to the output-affecting options over the configurations it extends: the values it changes, paths relative to it, None resetting an inherited one.   |  `{}` |
| <a id="js_tsconfig-package"></a>package |  Optional manifest, defaulting to a local package.json when present.   |  `None` |
| <a id="js_tsconfig-kwargs"></a>kwargs |  Extended configurations, declared deps, data, aliases and standard Bazel attributes.   |  none |


<a id="js_vite_config"></a>

## js_vite_config

<pre>
load("@latticebuild_js//js:defs.bzl", "js_vite_config")

js_vite_config(<a href="#js_vite_config-name">name</a>, <a href="#js_vite_config-config">config</a>, <a href="#js_vite_config-package">package</a>, <a href="#js_vite_config-kwargs">**kwargs</a>)
</pre>

Declares a Vite or Vitest configuration and its reusable inputs.

**PARAMETERS**


| Name  | Description | Default Value |
| :------------- | :------------- | :------------- |
| <a id="js_vite_config-name"></a>name |  Configuration target name.   |  none |
| <a id="js_vite_config-config"></a>config |  Original Vite or Vitest configuration file.   |  none |
| <a id="js_vite_config-package"></a>package |  Manifest controlling configuration module evaluation.   |  `"package.json"` |
| <a id="js_vite_config-kwargs"></a>kwargs |  Declared deps, data, aliases and standard Bazel attributes.   |  none |
