"""Collect action inputs and run commands in one declared scratch tree."""

load("//js:providers.bzl", "JsPackageInfo")
load(":execution_environment.bzl", "ExecutionEnvironmentInfo")
load(":layout.bzl", "dependency_links", "joined", "link", "merge_record", "package_records", "repository_path", "tree_layout", "write_manifest")

visibility("//...")

NODE_TOOLCHAIN_TYPE = Label("@rules_nodejs//nodejs:toolchain_type")

ACTION_ATTRS = {
    "_execution_environment": attr.label(
        cfg = "exec",
        default = Label("//js/support:execution_environment"),
        providers = [ExecutionEnvironmentInfo],
    ),
    "_run_action": attr.label(
        executable = True,
        cfg = "exec",
        default = Label("//js/support:run_action"),
    ),
}

def action_inputs(ctx, tool, script = None, files = [], deps = [], aliases = {}, tool_files = [], tool_deps = []):
    """Collect target and execution inventories without reconciling their packages.

    Args:
      ctx: Consuming action rule context.
      tool: Execution-configured JsBinaryInfo for the command.
      script: Script to run; defaults to the tool's bin.
      files: Target-configured files.
      deps: Target-configured packages and configuration providers.
      aliases: Target import bindings.
      tool_files: Execution-configured files.
      tool_deps: Execution-configured packages and configuration providers.

    Returns:
      The script, separate package inventories, import links and input files.
    """
    script = script or tool.bin
    packages = package_records(deps + aliases.values())
    execution = package_records(tool_deps)
    for record in tool.packages.to_list():
        merge_record(execution, record)
    links = dependency_links(ctx.label, ctx.label.package, deps + tool_deps, aliases)
    for path, package in tool.links:
        link(ctx.label, links, path, package)
    inputs = files + tool_files + tool.files.to_list() + [script]
    for target in deps + tool_deps:
        if JsPackageInfo not in target:
            inputs.extend(target[DefaultInfo].files.to_list())

    return struct(script = script, packages = packages, execution = execution, links = links, files = inputs)

def run_in_tree(ctx, inputs, arguments, outputs, mnemonic, progress_message, env = {}, executable = None, checks = []):
    """Validate the final layout and invoke run-action for its entire scratch lifetime.

    Args:
      ctx: Rule context with ACTION_ATTRS and the Node execution toolchain.
      inputs: Collected inventories, reconciled by the owning rule if needed.
      arguments: Literal arguments after the staged script.
      outputs: Declared output files and directories.
      mnemonic: Action mnemonic.
      progress_message: Action progress message.
      env: Declared environment; {STABLE_KEY} values add the stable status input.
      executable: Optional native tool target, receiving the staged script first.
      checks: Validation stamps required before staging starts.
    """
    packages = dict(inputs.packages)
    for record in inputs.execution.values():
        merge_record(packages, record)
    layout = tree_layout(ctx.label, packages, files = inputs.files, links = inputs.links)
    fields = {
        "args": arguments,
        "cwd": ctx.label.package,
        "env": env,
        "outputs": [[repository_path(file), file.path] for file in outputs],
        "root": ctx.bin_dir.path + "/" + joined(ctx.label.workspace_root, joined(ctx.label.package, ctx.label.name + ".scratch")),
        "scripts": [repository_path(inputs.script)],
    }
    if executable != None:
        fields["executable"] = executable[DefaultInfo].files_to_run.executable.path
    status = []
    if [value for value in env.values() if "{STABLE_" in value]:
        status = [ctx.info_file]
        fields["status_file"] = ctx.info_file.path
    manifest, declared_inputs = write_manifest(ctx, ctx.label.name + ".tree", layout, fields)
    node = ctx.toolchains[NODE_TOOLCHAIN_TYPE].nodeinfo

    # OS facilities explicitly configured by the execution platform are part of
    # the action key. Do not inherit shell Node options or package lookup paths.
    action_env = dict(ctx.attr._execution_environment[ExecutionEnvironmentInfo].values)
    action_env["NODE"] = node.node.path
    ctx.actions.run(
        executable = ctx.executable._run_action,
        arguments = [manifest.path],
        tools = [executable[DefaultInfo].files_to_run] if executable != None else [],
        env = action_env,
        inputs = depset([manifest, declared_inputs, node.node] + checks + status + layout.files.values()),
        outputs = outputs,
        use_default_shell_env = False,
        mnemonic = mnemonic,
        progress_message = progress_message,
    )
