<!-- Generated with Stardoc: http://skydoc.bazel.build -->

Supported action primitives for JavaScript adapters.

<a id="action_inputs"></a>

## action_inputs

<pre>
load("@latticebuild_js//js/support:action.bzl", "action_inputs")

action_inputs(<a href="#action_inputs-ctx">ctx</a>, <a href="#action_inputs-tool">tool</a>, <a href="#action_inputs-script">script</a>, <a href="#action_inputs-files">files</a>, <a href="#action_inputs-deps">deps</a>, <a href="#action_inputs-aliases">aliases</a>, <a href="#action_inputs-tool_files">tool_files</a>, <a href="#action_inputs-tool_deps">tool_deps</a>)
</pre>

Collect target and execution inventories without reconciling their packages.

**PARAMETERS**


| Name  | Description | Default Value |
| :------------- | :------------- | :------------- |
| <a id="action_inputs-ctx"></a>ctx |  Consuming action rule context.   |  none |
| <a id="action_inputs-tool"></a>tool |  Execution-configured JsBinaryInfo for the command.   |  none |
| <a id="action_inputs-script"></a>script |  Script to run; defaults to the tool's bin.   |  `None` |
| <a id="action_inputs-files"></a>files |  Target-configured files.   |  `[]` |
| <a id="action_inputs-deps"></a>deps |  Target-configured packages and configuration providers.   |  `[]` |
| <a id="action_inputs-aliases"></a>aliases |  Target import bindings.   |  `{}` |
| <a id="action_inputs-tool_files"></a>tool_files |  Execution-configured files.   |  `[]` |
| <a id="action_inputs-tool_deps"></a>tool_deps |  Execution-configured packages and configuration providers.   |  `[]` |

**RETURNS**

The script, separate package inventories, import links and input files.


<a id="run_in_tree"></a>

## run_in_tree

<pre>
load("@latticebuild_js//js/support:action.bzl", "run_in_tree")

run_in_tree(<a href="#run_in_tree-ctx">ctx</a>, <a href="#run_in_tree-inputs">inputs</a>, <a href="#run_in_tree-arguments">arguments</a>, <a href="#run_in_tree-outputs">outputs</a>, <a href="#run_in_tree-mnemonic">mnemonic</a>, <a href="#run_in_tree-progress_message">progress_message</a>, <a href="#run_in_tree-env">env</a>, <a href="#run_in_tree-executable">executable</a>, <a href="#run_in_tree-checks">checks</a>)
</pre>

Validate the final layout and invoke run-action for its entire scratch lifetime.

**PARAMETERS**


| Name  | Description | Default Value |
| :------------- | :------------- | :------------- |
| <a id="run_in_tree-ctx"></a>ctx |  Rule context with ACTION_ATTRS and the Node execution toolchain.   |  none |
| <a id="run_in_tree-inputs"></a>inputs |  Collected inventories, reconciled by the owning rule if needed.   |  none |
| <a id="run_in_tree-arguments"></a>arguments |  Literal arguments after the staged script.   |  none |
| <a id="run_in_tree-outputs"></a>outputs |  Declared output files and directories.   |  none |
| <a id="run_in_tree-mnemonic"></a>mnemonic |  Action mnemonic.   |  none |
| <a id="run_in_tree-progress_message"></a>progress_message |  Action progress message.   |  none |
| <a id="run_in_tree-env"></a>env |  Declared environment; {STABLE_KEY} values add the stable status input.   |  `{}` |
| <a id="run_in_tree-executable"></a>executable |  Optional native tool target, receiving the staged script first.   |  `None` |
| <a id="run_in_tree-checks"></a>checks |  Validation stamps required before staging starts.   |  `[]` |
