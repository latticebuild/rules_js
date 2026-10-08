<!-- Generated with Stardoc: http://skydoc.bazel.build -->

Supported execution primitives for JavaScript adapters.

<a id="executable_inputs"></a>

## executable_inputs

<pre>
load("@latticebuild_js//js/support:execution.bzl", "executable_inputs")

executable_inputs(<a href="#executable_inputs-ctx">ctx</a>, <a href="#executable_inputs-targets">targets</a>, <a href="#executable_inputs-files">files</a>, <a href="#executable_inputs-aliases">aliases</a>, <a href="#executable_inputs-tool">tool</a>)
</pre>

Gathers the files, package records and import bindings an executable lays out.

**PARAMETERS**


| Name  | Description | Default Value |
| :------------- | :------------- | :------------- |
| <a id="executable_inputs-ctx"></a>ctx |  Executable rule context.   |  none |
| <a id="executable_inputs-targets"></a>targets |  Declared runtime targets: packages, configuration providers and files.   |  none |
| <a id="executable_inputs-files"></a>files |  Additional Files, such as the scripts to run.   |  `[]` |
| <a id="executable_inputs-aliases"></a>aliases |  Additional import names mapped to package targets.   |  `{}` |
| <a id="executable_inputs-tool"></a>tool |  Optional JsBinaryInfo whose files, packages and bindings join the tree.   |  `None` |

**RETURNS**

A struct with runfiles, packages (package path to record) and links
(link path to package path).


<a id="node_executable"></a>

## node_executable

<pre>
load("@latticebuild_js//js/support:execution.bzl", "node_executable")

node_executable(<a href="#node_executable-ctx">ctx</a>, <a href="#node_executable-scripts">scripts</a>, <a href="#node_executable-inputs">inputs</a>, <a href="#node_executable-args">args</a>, <a href="#node_executable-cwd">cwd</a>, <a href="#node_executable-env">env</a>, <a href="#node_executable-env_paths">env_paths</a>, <a href="#node_executable-env_inherit">env_inherit</a>, <a href="#node_executable-mapped_files">mapped_files</a>,
                <a href="#node_executable-workspace_package">workspace_package</a>, <a href="#node_executable-coverage">coverage</a>, <a href="#node_executable-adapter">adapter</a>, <a href="#node_executable-test">test</a>, <a href="#node_executable-adapter_args">adapter_args</a>, <a href="#node_executable-native_runfiles">native_runfiles</a>)
</pre>

Binds node and the staged tree into one executable with its bound arguments and environment.

bound sets the declared environment, puts Node's directory first on
PATH (unless the target declares PATH), and for a package working
directory starts node there with a private home and temporary directory
in the tree. What bound cannot set up, the native runtime adapter does before
it runs the tool in its process: it enters a fix twin's package in the
invoking workspace, translates the LCOV a tool writes with the map this
binds, and applies Bazel's test environment to the test runner.


**PARAMETERS**


| Name  | Description | Default Value |
| :------------- | :------------- | :------------- |
| <a id="node_executable-ctx"></a>ctx |  Executable rule context with EXECUTABLE_TOOLCHAINS, runtime adapter attributes for an executable that uses the adapter, and COVERAGE_ATTRS for a tool that writes LCOV.   |  none |
| <a id="node_executable-scripts"></a>scripts |  Files passed to node, resolved at their paths in the tree: the tool's script, or a test's files.   |  none |
| <a id="node_executable-inputs"></a>inputs |  The executable_inputs result.   |  none |
| <a id="node_executable-args"></a>args |  Arguments after the scripts, before the caller's.   |  `[]` |
| <a id="node_executable-cwd"></a>cwd |  Package directory in the tree to run in (`""` for the root package); None keeps the caller's.   |  `None` |
| <a id="node_executable-env"></a>env |  Declared environment overrides.   |  `{}` |
| <a id="node_executable-env_paths"></a>env_paths |  Environment values resolved as paths in the tree.   |  `{}` |
| <a id="node_executable-env_inherit"></a>env_inherit |  Names the target inherits from the caller; bound binds no private default for them, so the caller's value passes through.   |  `[]` |
| <a id="node_executable-mapped_files"></a>mapped_files |  Extra inputs placed at explicitly declared tree paths.   |  `{}` |
| <a id="node_executable-workspace_package"></a>workspace_package |  Package directory of the invoking workspace to run in, for executables that edit sources; they refuse to run outside `bazel run`. The root package is `""`.   |  `None` |
| <a id="node_executable-coverage"></a>coverage |  Whether the tool writes LCOV tracefiles to COVERAGE_DIR under `bazel coverage`, which the adapter translates to Bazel's paths with the map this binds.   |  `False` |
| <a id="node_executable-adapter"></a>adapter |  Optional target-configured native runtime adapter.   |  `None` |
| <a id="node_executable-test"></a>test |  Whether the rule is a test, which keeps out of bound's per-user cache.   |  `False` |
| <a id="node_executable-adapter_args"></a>adapter_args |  Additional bound arguments preceding the adapter's separator.   |  `[]` |
| <a id="node_executable-native_runfiles"></a>native_runfiles |  Immutable native tools kept outside the private tree.   |  `None` |

**RETURNS**

A list of providers: DefaultInfo, and a test's RunEnvironmentInfo.


<a id="package_executable"></a>

## package_executable

<pre>
load("@latticebuild_js//js/support:execution.bzl", "package_executable")

package_executable(<a href="#package_executable-ctx">ctx</a>, <a href="#package_executable-tool">tool</a>, <a href="#package_executable-files">files</a>, <a href="#package_executable-args">args</a>, <a href="#package_executable-env">env</a>, <a href="#package_executable-env_inherit">env_inherit</a>, <a href="#package_executable-configurations">configurations</a>, <a href="#package_executable-packages">packages</a>, <a href="#package_executable-coverage">coverage</a>,
                   <a href="#package_executable-adapter">adapter</a>, <a href="#package_executable-test">test</a>, <a href="#package_executable-adapter_args">adapter_args</a>, <a href="#package_executable-native_runfiles">native_runfiles</a>)
</pre>

Creates one executable that runs a package tool in its package directory.

**PARAMETERS**


| Name  | Description | Default Value |
| :------------- | :------------- | :------------- |
| <a id="package_executable-ctx"></a>ctx |  Consuming rule context with PACKAGE_EXECUTABLE_ATTRS.   |  none |
| <a id="package_executable-tool"></a>tool |  The runtime tool target, providing JsBinaryInfo.   |  none |
| <a id="package_executable-files"></a>files |  Additional files at their repository paths, such as configuration files.   |  `[]` |
| <a id="package_executable-args"></a>args |  Tool arguments preceding forwarded caller arguments.   |  `[]` |
| <a id="package_executable-env"></a>env |  Runtime environment overrides.   |  `{}` |
| <a id="package_executable-env_inherit"></a>env_inherit |  Names the target inherits from the caller, such as the home under which a tool finds its plugins.   |  `[]` |
| <a id="package_executable-configurations"></a>configurations |  Configuration providers consumed by the executable.   |  `[]` |
| <a id="package_executable-packages"></a>packages |  Packages the rule itself adds, such as a tool's plugin.   |  `[]` |
| <a id="package_executable-coverage"></a>coverage |  Whether the tool writes LCOV tracefiles under `bazel coverage`.   |  `False` |
| <a id="package_executable-adapter"></a>adapter |  Optional native runtime adapter target.   |  `None` |
| <a id="package_executable-test"></a>test |  Whether the rule is a test.   |  `False` |
| <a id="package_executable-adapter_args"></a>adapter_args |  Additional bound arguments preceding the adapter's separator.   |  `[]` |
| <a id="package_executable-native_runfiles"></a>native_runfiles |  Immutable native tools kept outside the private tree.   |  `None` |

**RETURNS**

The executable's providers and propagated validation outputs.


<a id="tree_directory"></a>

## tree_directory

<pre>
load("@latticebuild_js//js/support:execution.bzl", "tree_directory")

tree_directory(<a href="#tree_directory-package">package</a>)
</pre>

The bundle path of a package's directory in the staged tree.

**PARAMETERS**


| Name  | Description | Default Value |
| :------------- | :------------- | :------------- |
| <a id="tree_directory-package"></a>package |  a repository-relative package directory; `""` or `"."` for the root package.   |  none |

**RETURNS**

`tree/<package>`, or `tree` for the root package.
