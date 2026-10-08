"""Bind the runtime Node and optional native adapter with a private filesystem tree."""

load("@rules_bound//bound:defs.bzl", "BOUND_TOOLCHAIN_TYPE", "BoundInfo", "bound_context", "bound_layout", "bundle_path", "rlocation")
load(":coverage.bzl", "coverage_file")
load(":fix.bzl", "workspace_directory")
load(":layout.bzl", "repository_path", "tree_layout")

visibility("//...")

NODE_RUNTIME_TOOLCHAIN_TYPE = Label("@rules_nodejs//nodejs:runtime_toolchain_type")

# An executable rule declares both: node for its platform, and bound, which
# binds node with the tree.
EXECUTABLE_TOOLCHAINS = [NODE_RUNTIME_TOOLCHAIN_TYPE, BOUND_TOOLCHAIN_TYPE]

# The tree's private temporary directory and home, for tools run in a
# package directory, and the names under which tools look for them.
_PRIVATE_DIRECTORIES = {
    "HOME": "tree/.home",
    "TEMP": "tree/.tmp",
    "TMP": "tree/.tmp",
    "TMPDIR": "tree/.tmp",
    "USERPROFILE": "tree/.home",
}

def tree_directory(package):
    """The bundle path of a package's directory in the staged tree.

    Args:
      package: a repository-relative package directory; `""` or `"."` for
        the root package.

    Returns:
      `tree/<package>`, or `tree` for the root package.
    """
    return "tree" if package in ("", ".") else "tree/" + package

def node_executable(ctx, scripts, inputs, args = [], cwd = None, env = {}, env_paths = {}, env_inherit = [], mapped_files = {}, workspace_package = None, coverage = False, adapter = None, test = False, adapter_args = [], native_runfiles = None):
    """Binds node and the staged tree into one executable with its bound arguments and environment.

    bound sets the declared environment, puts Node's directory first on
    PATH (unless the target declares PATH), and for a package working
    directory starts node there with a private home and temporary directory
    in the tree. What bound cannot set up, the native runtime adapter does before
    it runs the tool in its process: it enters a fix twin's package in the
    invoking workspace, translates the LCOV a tool writes with the map this
    binds, and applies Bazel's test environment to the test runner.

    Args:
      ctx: Executable rule context with EXECUTABLE_TOOLCHAINS, runtime adapter attributes
        for an executable that uses the adapter, and
        COVERAGE_ATTRS for a tool that writes LCOV.
      scripts: Files passed to node, resolved at their paths in the tree: the
        tool's script, or a test's files.
      inputs: The executable_inputs result.
      args: Arguments after the scripts, before the caller's.
      cwd: Package directory in the tree to run in (`""` for the root
        package); None keeps the caller's.
      env: Declared environment overrides.
      env_paths: Environment values resolved as paths in the tree.
      env_inherit: Names the target inherits from the caller; bound binds no
        private default for them, so the caller's value passes through.
      mapped_files: Extra inputs placed at explicitly declared tree paths.
      workspace_package: Package directory of the invoking workspace to run
        in, for executables that edit sources; they refuse to run outside
        `bazel run`. The root package is `""`.
      coverage: Whether the tool writes LCOV tracefiles to COVERAGE_DIR under
        `bazel coverage`, which the adapter translates to Bazel's paths with the
        map this binds.
      adapter: Optional target-configured native runtime adapter.
      test: Whether the rule is a test, which keeps out of bound's per-user
        cache.
      adapter_args: Additional bound arguments preceding the adapter's separator.
      native_runfiles: Immutable native tools kept outside the private tree.

    Returns:
      A list of providers: DefaultInfo, and a test's RunEnvironmentInfo.
    """
    mapped = {file: True for file in mapped_files.values()}
    layout = tree_layout(ctx.label, inputs.packages, files = [file for file in inputs.runfiles.files.to_list() if file not in mapped], links = inputs.links, mapped_files = mapped_files)
    if native_runfiles != None:
        native_files = {file: True for file in native_runfiles.files.to_list()}
        for file in layout.files.values():
            if file in native_files:
                fail("%s: native browser input %s also appears in application data or configuration" % (ctx.label, file.short_path))

    # Files at their repository paths are placed when the action runs, from
    # their runfiles paths; the few placed elsewhere are listed one by one.
    placed = []
    moved = []
    for path, file in layout.files.items():
        if path == repository_path(file):
            placed.append(file)
        else:
            moved.append(bound_layout.file("tree/" + path, file))
    node = ctx.toolchains[NODE_RUNTIME_TOOLCHAIN_TYPE].nodeinfo.node
    entries = [
        bound_layout.file("node/" + node.basename, node),
        bound_layout.directory("tree"),
        bound_layout.files(placed, dest = "tree", strip_prefix = sorted({rlocation(file).split("/")[0]: True for file in placed})),
        bound_layout.symlinks(sorted(layout.links.items()), dest = "tree"),
        # Native file walkers stop ancestor ignore discovery at .git: the
        # empty boundary keeps what lies above the tree out of the checkout.
        bound_layout.directory("tree/.git"),
        bound_layout.directory("tree/.home"),
        bound_layout.directory("tree/.tmp"),
    ] + moved

    # Declared variables, then defaults for the names the target neither
    # declares nor inherits. Names compare case-insensitively, as bound
    # compares them.
    environment = dict(env)
    for name, path in env_paths.items():
        environment[name] = bundle_path("tree" if path == "." else "tree/" + path)
    declared = {name.upper(): True for name in environment}
    inherited = {name.upper(): True for name in env_inherit}
    if cwd != None:
        entries.append(bound_layout.directory(tree_directory(cwd)))
        for name, path in _PRIVATE_DIRECTORIES.items():
            if name not in declared and name not in inherited:
                environment[name] = bundle_path(path)

    options = []
    if workspace_package != None:
        # The invoking workspace exists only once `bazel run` names it, so
        # bound cannot start node there; the adapter enters it.
        options.extend(["--workspace", workspace_directory(workspace_package)])
    if coverage:
        # Bazel's coverage manifest lists first-party files by execution
        # path: a source file's repository path, or bazel-out/... for a
        # generated one. Only the layout knows which file a tree path came
        # from. A mapped artifact sits at a path that is not its own and is no
        # coverage source. Whether a run writes coverage is known only when it
        # runs, and report tests give fixtures a COVERAGE_DIR outside `bazel
        # coverage`, so every run gets the map.
        coverage_map = coverage_file(ctx, layout, mapped)
        entries.append(bound_layout.file("coverage_map.json", coverage_map))
        options.extend(["--coverage", bundle_path("coverage_map.json")])
    program = "node/" + node.basename
    entry = []
    if adapter != None:
        executable = adapter[DefaultInfo].files_to_run.executable
        program = "adapter/" + executable.basename
        entries.append(bound_layout.file(program, executable))
        entry = ["--node", bundle_path("node/" + node.basename)] + options + adapter_args + ["--"]
    elif options or adapter_args:
        fail("%s: runtime setup requires its declared adapter" % ctx.label)

    info = BoundInfo(
        layout = entries,
        program = program,
        args = entry + [bundle_path("tree/" + repository_path(file)) for file in scripts] + args,
        env = environment,
        # Child tools use the same interpreter. A declared PATH replaces it;
        # an inherited one still follows Node's directory.
        env_prepend = {} if "PATH" in declared else {"PATH": [bundle_path("node")]},
        # An enclosing Node coverage run would also collect this tool's V8
        # profiles into its own directory; node reads the variable at start.
        unset = ["NODE_V8_COVERAGE"] if coverage else [],
    )
    result = bound_context(ctx).bind(info, bundle = "private", cwd = bundle_path(tree_directory(cwd)) if cwd != None else "inherit")
    providers = [DefaultInfo(executable = result.executable, runfiles = native_runfiles)]
    if test:
        # bound's per-user cache never evicts; a test's run leaves nothing.
        providers.append(RunEnvironmentInfo(environment = {"BOUND_CACHE": "0"}, inherited_environment = env_inherit))
    return providers
