"""Assemble package inputs and settings for runtime executables."""

load("//js:providers.bzl", "JsBinaryInfo", "JsPackageInfo")
load(":layout.bzl", "checked_path", "dependency_links", "link", "merge_record", "package_records", "place", "validation_outputs")
load(":runtime.bzl", "node_executable")

visibility("//...")

def executable_inputs(ctx, targets, files = [], aliases = {}, tool = None):
    """Gathers the files, package records and import bindings an executable lays out.

    Args:
      ctx: Executable rule context.
      targets: Declared runtime targets: packages, configuration providers and files.
      files: Additional Files, such as the scripts to run.
      aliases: Additional import names mapped to package targets.
      tool: Optional JsBinaryInfo whose files, packages and bindings join the tree.

    Returns:
      A struct with runfiles, packages (package path to record) and links
      (link path to package path).
    """
    dependencies = targets + aliases.values()
    runfiles = ctx.runfiles(
        files = files,
        transitive_files = depset(transitive = [target[DefaultInfo].files for target in dependencies] + ([tool.files] if tool else [])),
    ).merge_all([target[DefaultInfo].default_runfiles for target in dependencies])
    packages = package_records(dependencies)
    links = dependency_links(ctx.label, ctx.label.package, targets, aliases)
    if tool:
        for record in tool.packages.to_list():
            merge_record(packages, record)
        for path, package in tool.links:
            link(ctx.label, links, path, package)
    return struct(runfiles = runfiles, packages = packages, links = links)

PACKAGE_EXECUTABLE_ATTRS = {
    "aliases": attr.string_keyed_label_dict(providers = [JsPackageInfo]),
    "data": attr.label_list(
        doc = "Declared runtime packages, sources, checked configs and build prerequisites.",
        allow_files = True,
    ),
    "data_paths": attr.label_keyed_string_dict(
        doc = "Single file or directory targets mapped to explicit tree paths.",
        allow_files = True,
    ),
    "deps": attr.label_list(doc = "Imported files, packages and configuration providers.", allow_files = True),
    "env": attr.string_dict(doc = "Runtime environment overrides."),
    "env_paths": attr.string_dict(doc = "Environment values resolved as paths inside the tree."),
    "srcs": attr.label_list(doc = "Imported packages and files traced from the configuration.", allow_files = True),
}

def package_executable(ctx, tool, files = [], args = [], env = {}, env_inherit = [], configurations = [], packages = [], coverage = False, adapter = None, test = False, adapter_args = [], native_runfiles = None):
    """Creates one executable that runs a package tool in its package directory.

    Args:
      ctx: Consuming rule context with PACKAGE_EXECUTABLE_ATTRS.
      tool: The runtime tool target, providing JsBinaryInfo.
      files: Additional files at their repository paths, such as
        configuration files.
      args: Tool arguments preceding forwarded caller arguments.
      env: Runtime environment overrides.
      env_inherit: Names the target inherits from the caller, such as the
        home under which a tool finds its plugins.
      configurations: Configuration providers consumed by the executable.
      packages: Packages the rule itself adds, such as a tool's plugin.
      coverage: Whether the tool writes LCOV tracefiles under `bazel coverage`.
      adapter: Optional native runtime adapter target.
      test: Whether the rule is a test.
      adapter_args: Additional bound arguments preceding the adapter's separator.
      native_runfiles: Immutable native tools kept outside the private tree.

    Returns:
      The executable's providers and propagated validation outputs.
    """
    info = tool[JsBinaryInfo]
    targets = ctx.attr.srcs + ctx.attr.deps + ctx.attr.data + ctx.attr.data_paths.keys() + configurations + packages
    mapped = {}
    for target, path in ctx.attr.data_paths.items():
        outputs = target[DefaultInfo].files.to_list()
        if len(outputs) != 1:
            fail("%s: data_paths requires one artifact per target, got %s" % (ctx.label, target.label))
        place(ctx.label, mapped, checked_path(path), outputs[0])
    return node_executable(
        ctx,
        [info.bin],
        executable_inputs(ctx, targets, files = files, aliases = ctx.attr.aliases, tool = info),
        args = args,
        cwd = ctx.label.package,
        env = env,
        env_paths = {key: checked_path(path, allow_root = True) for key, path in ctx.attr.env_paths.items()},
        env_inherit = env_inherit,
        mapped_files = mapped,
        coverage = coverage,
        adapter = adapter,
        test = test,
        adapter_args = adapter_args,
        native_runfiles = native_runfiles,
    ) + [OutputGroupInfo(_validation = validation_outputs(targets + ctx.attr.aliases.values() + [tool]))]
