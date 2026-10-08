<!-- Generated with Stardoc: http://skydoc.bazel.build -->

Supported fix primitives for JavaScript adapters.

<a id="fix_name"></a>

## fix_name

<pre>
load("@latticebuild_js//js/support:fix.bzl", "fix_name")

fix_name(<a href="#fix_name-name">name</a>)
</pre>

The fix executable declared beside a check test.

**PARAMETERS**


| Name  | Description | Default Value |
| :------------- | :------------- | :------------- |
| <a id="fix_name-name"></a>name |  The check test's name, which must end in `_test`.   |  none |

**RETURNS**

`name` with its `_test` suffix replaced by `_fix`.


<a id="workspace_directory"></a>

## workspace_directory

<pre>
load("@latticebuild_js//js/support:fix.bzl", "workspace_directory")

workspace_directory(<a href="#workspace_directory-package">package</a>)
</pre>

A package's directory in the invoking workspace, as the fix entry script takes it.

**PARAMETERS**


| Name  | Description | Default Value |
| :------------- | :------------- | :------------- |
| <a id="workspace_directory-package"></a>package |  a repository-relative package directory; `""` or `"."` for the root package.   |  none |

**RETURNS**

`package`, or `.` for the root package, since an empty argument is
not portable.
