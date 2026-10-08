"""Vite bundles in isolated checkout-shaped trees."""

load("//js:providers.bzl", "JsBinaryInfo", "JsPackageInfo", "JsViteConfigInfo")
load("//js/support:action.bzl", "ACTION_ATTRS", "NODE_TOOLCHAIN_TYPE", "action_inputs", "run_in_tree")
load("//js/support:layout.bzl", "checked_path", "merge_record", "package_relative_path", "place", "repository_path", "validation_outputs")

visibility("//...")

def _js_vite_impl(ctx):
    config = ctx.attr.config[JsViteConfigInfo]
    out = ctx.actions.declare_directory(checked_path(ctx.attr.out))
    inputs = action_inputs(
        ctx,
        ctx.attr.vite[JsBinaryInfo],
        files = ctx.files.srcs,
        deps = ctx.attr.deps,
        aliases = ctx.attr.aliases,
        tool_files = ctx.files.tool_data,
        tool_deps = ctx.attr.tool_deps + [ctx.attr.config],
    )
    inputs, checks = _reconcile_packages(ctx, inputs)
    run_in_tree(
        ctx,
        inputs,
        arguments = ["build", "--mode", ctx.attr.mode, "--config", package_relative_path(ctx, config.config)],
        env = ctx.attr.env,
        checks = checks,
        outputs = [out],
        mnemonic = "ViteBuild",
        progress_message = "Building %{label} with Vite",
    )
    return [
        DefaultInfo(files = depset([out])),
        OutputGroupInfo(_validation = validation_outputs(ctx.attr.srcs + ctx.attr.deps + ctx.attr.tool_data + ctx.attr.tool_deps + ctx.attr.aliases.values() + [ctx.attr.config, ctx.attr.vite])),
    ]

js_vite = rule(
    implementation = _js_vite_impl,
    doc = "Runs vite build and publishes the directory selected by its configuration.",
    toolchains = [NODE_TOOLCHAIN_TYPE],
    attrs = ACTION_ATTRS | {
        "aliases": attr.string_keyed_label_dict(providers = [JsPackageInfo]),
        "config": attr.label(providers = [JsViteConfigInfo], cfg = "exec", mandatory = True),
        "deps": attr.label_list(providers = [JsPackageInfo]),
        "env": attr.string_dict(),
        "mode": attr.string(default = "production"),
        "out": attr.string(default = "dist"),
        "srcs": attr.label_list(doc = "File inputs in the target configuration: application files, project file inventories and configuration files, without implicit package bindings.", allow_files = True),
        "tool_data": attr.label_list(
            doc = "Additional file inputs in the execution configuration, such as executables or generated files the build tooling reads, without package bindings.",
            allow_files = True,
            cfg = "exec",
        ),
        "tool_deps": attr.label_list(providers = [JsPackageInfo], cfg = "exec"),
        "vite": attr.label(mandatory = True, providers = [JsBinaryInfo], executable = True, cfg = "exec"),
        "_compare_packages": attr.label(default = Label("//js/private/vite/tools/compare-packages"), executable = True, cfg = "exec"),
    },
)

def _reconcile_packages(ctx, inputs):
    packages = dict(inputs.packages)
    comparisons = []
    for record in inputs.execution.values():
        comparisons.extend(_share_execution_package(packages, record))
    checks = []
    if comparisons:
        inventory = ctx.actions.declare_file(ctx.label.name + ".packages.json")
        stamp = ctx.actions.declare_file(ctx.label.name + ".packages_check")
        ctx.actions.write(inventory, json.encode([[path, target.path, execution.path] for path, target, execution in comparisons]))

        # compare-packages uses only its declared inventory and file inputs.
        ctx.actions.run(
            executable = ctx.executable._compare_packages,
            arguments = [inventory.path, stamp.path],
            inputs = [inventory] + [file for _, target, execution in comparisons for file in [target, execution]],
            outputs = [stamp],
            use_default_shell_env = False,
            mnemonic = "VitePackageCheck",
            progress_message = "Checking shared target and tool packages for %{label}",
        )
        checks.append(stamp)

    equivalents = {execution.path: target for _, target, execution in comparisons}
    return struct(
        script = inputs.script,
        packages = packages,
        execution = {},
        files = [equivalents.get(file.path, file) for file in inputs.files],
        links = inputs.links,
    ), checks

def _share_execution_package(records, record):
    # Returns (path, target File, execution File) for each file whose two
    # configured copies must be proven identical before they can share a path.
    previous = records.get(record.package)
    if not previous:
        merge_record(records, record)
        return []
    target_files = {}
    execution_files = {}
    for file in previous.files.to_list():
        place(record.package, target_files, repository_path(file), file)
    for file in record.files.to_list():
        place(record.package, execution_files, repository_path(file), file)
    if previous.repository != record.repository or previous.name != record.name:
        fail("conflicting package identities at %s" % record.package)
    if sorted(target_files) != sorted(execution_files):
        fail("conflicting target and execution package inventories at %s" % record.package)
    if previous.links != record.links or previous.installs != record.installs:
        fail("conflicting target and execution package bindings at %s" % record.package)
    comparisons = []
    for path, target in target_files.items():
        execution = execution_files[path]
        if target.path == execution.path:
            continue
        if target.owner != execution.owner or target.short_path != execution.short_path or target.is_directory or execution.is_directory:
            fail("%s: %s is both %s and %s" % (record.package, path, target.path, execution.path))
        comparisons.append((path, target, execution))
    return comparisons
