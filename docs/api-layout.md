<!-- Generated with Stardoc: http://skydoc.bazel.build -->

Supported layout primitives for JavaScript adapters.

<a id="checked_path"></a>

## checked_path

<pre>
load("@latticebuild_js//js/support:layout.bzl", "checked_path")

checked_path(<a href="#checked_path-path">path</a>, <a href="#checked_path-allow_root">allow_root</a>)
</pre>

Normalizes a relative slash path without allowing escape from its package.

**PARAMETERS**


| Name  | Description | Default Value |
| :------------- | :------------- | :------------- |
| <a id="checked_path-path"></a>path |  A relative path using slash separators.   |  none |
| <a id="checked_path-allow_root"></a>allow_root |  Whether the containing directory itself is allowed.   |  `False` |

**RETURNS**

The normalized relative path.


<a id="config_providers"></a>

## config_providers

<pre>
load("@latticebuild_js//js/support:layout.bzl", "config_providers")

config_providers(<a href="#config_providers-ctx">ctx</a>, <a href="#config_providers-files">files</a>, <a href="#config_providers-inputs">inputs</a>, <a href="#config_providers-bases">bases</a>)
</pre>

The bindings and providers shared by js_tsconfig and js_vite_config.

**PARAMETERS**


| Name  | Description | Default Value |
| :------------- | :------------- | :------------- |
| <a id="config_providers-ctx"></a>ctx |  Rule context with config, deps and aliases attributes.   |  none |
| <a id="config_providers-files"></a>files |  depset of the configuration's own files and those it inherits.   |  none |
| <a id="config_providers-inputs"></a>inputs |  The configuration's own file-input targets, for runfiles and validation.   |  none |
| <a id="config_providers-bases"></a>bases |  Extended configuration targets. Their package records and bindings join this configuration's and their validations propagate; their runfiles, which carry their sources, do not.   |  `[]` |

**RETURNS**

A struct with packages (depset of merged package records), links (tuple
of import bindings) and providers (DefaultInfo and validation outputs).


<a id="dependency_links"></a>

## dependency_links

<pre>
load("@latticebuild_js//js/support:layout.bzl", "dependency_links")

dependency_links(<a href="#dependency_links-label">label</a>, <a href="#dependency_links-package">package</a>, <a href="#dependency_links-deps">deps</a>, <a href="#dependency_links-aliases">aliases</a>)
</pre>

Binds workspace outputs; installed packages already have pnpm's layout.

**PARAMETERS**


| Name  | Description | Default Value |
| :------------- | :------------- | :------------- |
| <a id="dependency_links-label"></a>label |  The consuming target, for diagnostics.   |  none |
| <a id="dependency_links-package"></a>package |  Its repository-relative directory.   |  none |
| <a id="dependency_links-deps"></a>deps |  Declared package and configuration-provider targets.   |  none |
| <a id="dependency_links-aliases"></a>aliases |  Additional workspace import names mapped to package targets.   |  none |

**RETURNS**

Import-link paths mapped to canonical package directories.


<a id="merge_record"></a>

## merge_record

<pre>
load("@latticebuild_js//js/support:layout.bzl", "merge_record")

merge_record(<a href="#merge_record-records">records</a>, <a href="#merge_record-record">record</a>)
</pre>

Combines compatible records for one physical package; rejects ambiguity.

**PARAMETERS**


| Name  | Description | Default Value |
| :------------- | :------------- | :------------- |
| <a id="merge_record-records"></a>records |  Mutable records keyed by staged package path.   |  none |
| <a id="merge_record-record"></a>record |  A package record with canonical repository identity.   |  none |


<a id="package_records"></a>

## package_records

<pre>
load("@latticebuild_js//js/support:layout.bzl", "package_records")

package_records(<a href="#package_records-targets">targets</a>)
</pre>

The package records of package/config targets, by package path.

**PARAMETERS**


| Name  | Description | Default Value |
| :------------- | :------------- | :------------- |
| <a id="package_records-targets"></a>targets |  Targets; package and configuration providers contribute their closures.   |  none |

**RETURNS**

dict from package path to record.


<a id="package_relative_path"></a>

## package_relative_path

<pre>
load("@latticebuild_js//js/support:layout.bzl", "package_relative_path")

package_relative_path(<a href="#package_relative_path-ctx">ctx</a>, <a href="#package_relative_path-file">file</a>)
</pre>

The path of one of this package's files relative to the package.

**PARAMETERS**


| Name  | Description | Default Value |
| :------------- | :------------- | :------------- |
| <a id="package_relative_path-ctx"></a>ctx |  The rule context.   |  none |
| <a id="package_relative_path-file"></a>file |  A File of ctx's package.   |  none |

**RETURNS**

The path, such as src/index.ts.


<a id="place"></a>

## place

<pre>
load("@latticebuild_js//js/support:layout.bzl", "place")

place(<a href="#place-label">label</a>, <a href="#place-files">files</a>, <a href="#place-path">path</a>, <a href="#place-file">file</a>)
</pre>

Adds a file at a tree path, failing if another file is already there.

**PARAMETERS**


| Name  | Description | Default Value |
| :------------- | :------------- | :------------- |
| <a id="place-label"></a>label |  The target laying the tree out, for errors.   |  none |
| <a id="place-files"></a>files |  dict from tree path to File.   |  none |
| <a id="place-path"></a>path |  The tree path.   |  none |
| <a id="place-file"></a>file |  The File.   |  none |


<a id="relpath"></a>

## relpath

<pre>
load("@latticebuild_js//js/support:layout.bzl", "relpath")

relpath(<a href="#relpath-from_dir">from_dir</a>, <a href="#relpath-to_path">to_path</a>)
</pre>

The relative path of to_path from the directory from_dir.

skylib's paths.relativize never climbs, and a link from one package to
another does.


**PARAMETERS**


| Name  | Description | Default Value |
| :------------- | :------------- | :------------- |
| <a id="relpath-from_dir"></a>from_dir |  A directory path.   |  none |
| <a id="relpath-to_path"></a>to_path |  A path under the same root.   |  none |

**RETURNS**

The path, with a ".." for each directory to leave.


<a id="repository_path"></a>

## repository_path

<pre>
load("@latticebuild_js//js/support:layout.bzl", "repository_path")

repository_path(<a href="#repository_path-file">file</a>)
</pre>

The path inside a File's repository, also used in the staged Node layout.

**PARAMETERS**


| Name  | Description | Default Value |
| :------------- | :------------- | :------------- |
| <a id="repository_path-file"></a>file |  A source or generated File.   |  none |

**RETURNS**

Its path without Bazel's external-repository runfiles prefix.


<a id="validation_outputs"></a>

## validation_outputs

<pre>
load("@latticebuild_js//js/support:layout.bzl", "validation_outputs")

validation_outputs(<a href="#validation_outputs-targets">targets</a>)
</pre>

Retains validation stamps when a configured dependency is used as a tool.

**PARAMETERS**


| Name  | Description | Default Value |
| :------------- | :------------- | :------------- |
| <a id="validation_outputs-targets"></a>targets |  Dependencies whose validation groups must reach the consumer.   |  none |

**RETURNS**

A depset of stamps, separate from staged files and action inputs.
