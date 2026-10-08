<!-- Generated with Stardoc: http://skydoc.bazel.build -->

Providers shared by the JavaScript rules.

A package record is a struct describing one package directory in a staged
tree:

- name: the node_modules key Node resolves it by.
- repository: the canonical Bazel repository that owns its files.
- package: its repository-relative directory. A package under a node_modules
  directory is installed; any other package belongs to the workspace.
- files: depset of its own files, at repository paths under package.
- links: sorted (link path, package path) pairs binding its workspace imports.
- installs: sorted repository paths of observed node_modules symlinks to it.

Records are immutable so that depsets can hold them. Consumers merge the
records of one directory and reject conflicting identities, files or links.

<a id="JsBinaryInfo"></a>

## JsBinaryInfo

<pre>
load("@latticebuild_js//js:providers.bzl", "JsBinaryInfo")

JsBinaryInfo(<a href="#JsBinaryInfo-bin">bin</a>, <a href="#JsBinaryInfo-files">files</a>, <a href="#JsBinaryInfo-links">links</a>, <a href="#JsBinaryInfo-packages">packages</a>)
</pre>

A published package.json bin script, for rules that run it with node in a tree of their own.

**FIELDS**

| Name  | Description |
| :------------- | :------------- |
| <a id="JsBinaryInfo-bin"></a>bin |  File of the published bin script. Build actions pass this path to node.    |
| <a id="JsBinaryInfo-files"></a>files |  depset of script and data files, excluding the executable and Node runtime.    |
| <a id="JsBinaryInfo-links"></a>links |  tuple of direct (link path, package path) bindings for a workspace executable's data dependencies.    |
| <a id="JsBinaryInfo-packages"></a>packages |  depset of the merged package records the script needs, one per package directory.    |


<a id="JsPackageInfo"></a>

## JsPackageInfo

<pre>
load("@latticebuild_js//js:providers.bzl", "JsPackageInfo")

JsPackageInfo(<a href="#JsPackageInfo-closure">closure</a>, <a href="#JsPackageInfo-name">name</a>, <a href="#JsPackageInfo-package">package</a>)
</pre>

An npm package: where it lives and the package records a tree needs to hold it.

**FIELDS**

| Name  | Description |
| :------------- | :------------- |
| <a id="JsPackageInfo-closure"></a>closure |  depset of package records for this package and everything it reaches. Generated platform edges select the required records.    |
| <a id="JsPackageInfo-name"></a>name |  The node_modules key, such as @latticebuild/base or string-width-cjs.    |
| <a id="JsPackageInfo-package"></a>package |  Repository directory of the package: node_modules/<name> (or a workspace package's own node_modules/<name>) for an installed package, the Bazel package for a workspace package.    |


<a id="JsTsconfigInfo"></a>

## JsTsconfigInfo

<pre>
load("@latticebuild_js//js:providers.bzl", "JsTsconfigInfo")

JsTsconfigInfo(<a href="#JsTsconfigInfo-compiler_options">compiler_options</a>, <a href="#JsTsconfigInfo-config">config</a>, <a href="#JsTsconfigInfo-inheritance">inheritance</a>, <a href="#JsTsconfigInfo-links">links</a>, <a href="#JsTsconfigInfo-packages">packages</a>, <a href="#JsTsconfigInfo-sources">sources</a>)
</pre>

A declared TypeScript project, independent of compilation.

DefaultInfo.files holds the project's own files (configuration, sources,
data and manifest) and what the configurations it extends pass on, without
its package dependencies.

**FIELDS**

| Name  | Description |
| :------------- | :------------- |
| <a id="JsTsconfigInfo-compiler_options"></a>compiler_options |  Effective output-affecting options, with config-relative paths: what the configurations it extends pass on, overlaid by its own.    |
| <a id="JsTsconfigInfo-config"></a>config |  Original configuration File.    |
| <a id="JsTsconfigInfo-inheritance"></a>inheritance |  depset of the files a configuration extending this one stages: this configuration, its data and what the configurations it extends pass on, such as SvelteKit's generated declarations. Never its sources.    |
| <a id="JsTsconfigInfo-links"></a>links |  tuple of (link path, package path) import bindings, including those of the configurations it extends.    |
| <a id="JsTsconfigInfo-packages"></a>packages |  depset of the merged package records of deps, aliases and the configurations it extends, one per package directory.    |
| <a id="JsTsconfigInfo-sources"></a>sources |  depset of the project's source inventory.    |


<a id="JsViteConfigInfo"></a>

## JsViteConfigInfo

<pre>
load("@latticebuild_js//js:providers.bzl", "JsViteConfigInfo")

JsViteConfigInfo(<a href="#JsViteConfigInfo-config">config</a>, <a href="#JsViteConfigInfo-links">links</a>, <a href="#JsViteConfigInfo-packages">packages</a>)
</pre>

A reusable Vite or Vitest configuration and its declared inputs, without evaluation.

DefaultInfo.files holds the configuration's own files: configuration,
manifest, data and the files of non-package dependencies.

**FIELDS**

| Name  | Description |
| :------------- | :------------- |
| <a id="JsViteConfigInfo-config"></a>config |  Original configuration File.    |
| <a id="JsViteConfigInfo-links"></a>links |  tuple of (link path, package path) import bindings.    |
| <a id="JsViteConfigInfo-packages"></a>packages |  depset of the merged package records of deps and aliases, one per package directory.    |
