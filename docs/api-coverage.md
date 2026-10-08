<!-- Generated with Stardoc: http://skydoc.bazel.build -->

Supported coverage primitives for JavaScript adapters.

<a id="instrumented_files"></a>

## instrumented_files

<pre>
load("@latticebuild_js//js/support:coverage.bzl", "instrumented_files")

instrumented_files(<a href="#instrumented_files-ctx">ctx</a>, <a href="#instrumented_files-attributes">attributes</a>)
</pre>

The coverage provider of a test whose tools instrument its own inputs.

The test's attributes carry the code under test, so they are its sources
in Bazel's coverage manifest (under `--instrument_test_targets`), and its
dependencies' providers join through the same attributes. Bazel's own
baseline would give every installed package file a zero-coverage record in
the merged report, so the baseline lists only first-party sources, without
the declaration and test files the tools never report.


**PARAMETERS**


| Name  | Description | Default Value |
| :------------- | :------------- | :------------- |
| <a id="instrumented_files-ctx"></a>ctx |  Test rule context.   |  none |
| <a id="instrumented_files-attributes"></a>attributes |  Names of the attributes that hold the code under test.   |  none |

**RETURNS**

InstrumentedFilesInfo.
