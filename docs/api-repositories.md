<!-- Generated with Stardoc: http://skydoc.bazel.build -->

Repository rules for consuming an installed JavaScript workspace.

<a id="node_modules"></a>

## node_modules

<pre>
load("@latticebuild_js//js:repositories.bzl", "node_modules")

node_modules(<a href="#node_modules-name">name</a>, <a href="#node_modules-package_json">package_json</a>)
</pre>

Expose an installed workspace from an existing installation, without a setup generator.

**ATTRIBUTES**


| Name  | Description | Type | Mandatory | Default |
| :------------- | :------------- | :------------- | :------------- | :------------- |
| <a id="node_modules-name"></a>name |  A unique name for this repository.   | <a href="https://bazel.build/concepts/labels#target-names">Name</a> | required |  |
| <a id="node_modules-package_json"></a>package_json |  The installation owner's root package.json, beside its installed node_modules.   | <a href="https://bazel.build/concepts/labels">Label</a> | required |  |
