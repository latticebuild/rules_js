"""The staged checkout shared by actions, executables, and runtime images.

Repository paths, package records and their import bindings, complete tree
layouts, the manifests and declared-input inventories that hand a layout to the
Go runner, and the inputs that configuration providers contribute to a tree.
"""

load("@bazel_skylib//lib:paths.bzl", "paths")
load("//js:providers.bzl", "JsPackageInfo", "JsTsconfigInfo", "JsViteConfigInfo")

visibility("//...")

def tree_layout(label, packages, files = [], links = {}, runtime_only = False, mapped_files = {}):
    """Merges declared packages and extra inputs into one checked layout.

    Installed files retain pnpm's paths. Only observed installation symlinks
    and workspace output bindings add links; npm dependency edges add files.

    Args:
      label: The consumer's label, for errors.
      packages: Package records keyed by physical repository path.
      files: Additional input Files, placed at their repository-relative paths.
      links: Additional repository link paths mapped to package paths.
      runtime_only: Omit declarations, maps, build info, and installed @types
        packages, including links to them.
      mapped_files: Extra Files keyed by an explicit staged path.

    Returns:
      A struct with files (tree path to File) and links (tree path to tree path).
    """
    placed = {}
    bindings = {}
    for record in packages.values():
        if runtime_only and _types_path(record.package):
            continue
        prefix = record.package + "/" if record.package else ""
        for file in record.files.to_list():
            path = repository_path(file)
            if file.owner.workspace_name != record.repository or not path.startswith(prefix):
                fail("%s: %s places %s outside its directory %s" % (label, record.name, path, record.package))
            _place_input(label, placed, path, file, runtime_only)
        for path in record.installs:
            _bind(label, bindings, path, record.package, runtime_only)
        for path, package in record.links:
            if package not in packages:
                fail("%s: %s imports from undeclared package %s" % (label, path, package))
            _bind(label, bindings, path, package, runtime_only)
    for file in files:
        _place_input(label, placed, repository_path(file), file, runtime_only)
    for path, file in mapped_files.items():
        place(label, placed, checked_path(path), file)
    for path, package in links.items():
        _bind(label, bindings, path, package, runtime_only)

    # A trailing slash keeps a directory beside its descendants when a sibling
    # such as "foo-bar" would otherwise sort between them.
    destinations = sorted([path + "/" for path in placed.keys() + bindings.keys()])
    for previous, current in zip(destinations, destinations[1:]):
        if current.startswith(previous):
            fail("%s: overlapping tree destinations %s and %s" % (label, previous.removesuffix("/"), current.removesuffix("/")))
    return struct(files = placed, links = bindings)

_BUILD_ONLY_SUFFIXES = (".d.ts", ".d.mts", ".d.cts", ".map", ".tsbuildinfo")

def _place_input(label, files, path, file, runtime_only):
    # A runtime tree omits declarations, maps and build info. The runner
    # filters directory artifacts leaf by leaf when it copies them.
    if runtime_only and not file.is_directory and (_types_path(path) or path.endswith(_BUILD_ONLY_SUFFIXES)):
        return
    place(label, files, path, file)

def _bind(label, links, path, package, runtime_only):
    # A link to its own directory is the directory itself, and a runtime tree
    # holds no @types package to link to.
    if path == package or (runtime_only and (_types_path(path) or _types_path(package))):
        return
    link(label, links, path, package)

def _types_path(path):
    # Whether a path is, or lies inside, an installed @types package.
    return "/node_modules/@types/" in "/" + path

def place(label, files, path, file):
    """Adds a file at a tree path, failing if another file is already there.

    Args:
      label: The target laying the tree out, for errors.
      files: dict from tree path to File.
      path: The tree path.
      file: The File.
    """
    previous = files.get(path)
    if previous and previous.path != file.path:
        fail("%s: %s is both %s and %s" % (label, path, previous.path, file.path))
    files[path] = file

def link(label, links, path, target):
    """Adds one import destination, rejecting a conflicting binding."""
    if path in links and links[path] != target:
        fail("%s: import %s resolves to both %s and %s" % (label, path, links[path], target))
    links[path] = target

def write_manifest(ctx, name, layout, fields):
    """Serializes a checked layout and operation fields for the runner.

    Args:
      ctx: Rule context.
      name: Output basename, without the .json suffix.
      layout: Checked tree_layout result.
      fields: Operation-specific manifest fields, such as scripts and outputs.

    Returns:
      The manifest File and its declared-input inventory File.
    """
    inventory = write_input_list(ctx, name + ".inputs", layout.files.values())
    manifest = ctx.actions.declare_file(name + ".json")
    ctx.actions.write(manifest, json.encode(dict(fields, **{
        "files": [[path, file.path] for path, file in sorted(layout.files.items())],
        "input_list": inventory.path,
        "links": relative_links(layout.links),
    })))
    return manifest, inventory

def write_input_list(ctx, name, files):
    """Writes JSON lines with Bazel-expanded file paths, including directory leaves.

    Args:
      ctx: Rule context.
      name: Output name.
      files: Declared input Files.

    Returns:
      The inventory File.
    """
    output = ctx.actions.declare_file(name)
    args = ctx.actions.args()
    args.set_param_file_format("multiline")
    args.add_all(files, map_each = _action_entry)
    ctx.actions.write(output, args)
    return output

def _action_entry(file):
    return json.encode(file.path)

def relative_links(links):
    """Encodes tree links as sorted [path, target relative to the link's directory] pairs.

    Args:
      links: dict from tree link path to tree package path.

    Returns:
      A sorted list of two-element lists.
    """
    return sorted([
        [path, relpath(path.rpartition("/")[0], target)]
        for path, target in links.items()
    ])

def package_records(targets):
    """The package records of package/config targets, by package path.

    Args:
      targets: Targets; package and configuration providers contribute their closures.

    Returns:
      dict from package path to record.
    """
    records = {}
    for target in targets:
        for provider in [JsTsconfigInfo, JsViteConfigInfo]:
            if provider in target:
                for record in target[provider].packages.to_list():
                    merge_record(records, record)
        if JsPackageInfo in target:
            for record in target[JsPackageInfo].closure.to_list():
                merge_record(records, record)
    return records

def merge_record(records, record):
    """Combines compatible records for one physical package; rejects ambiguity.

    Args:
      records: Mutable records keyed by staged package path.
      record: A package record with canonical repository identity.
    """
    previous = records.get(record.package)
    if not previous:
        records[record.package] = record
        return
    if previous.repository != record.repository:
        fail("conflicting package repositories at %s: %s and %s" % (record.package, previous.repository, record.repository))
    if previous.name != record.name:
        fail("conflicting package identities at %s: %s and %s" % (record.package, previous.name, record.name))
    files = {}
    for file in previous.files.to_list() + record.files.to_list():
        place(record.package, files, repository_path(file), file)
    links = dict(previous.links)
    for path, package in record.links:
        link(record.package, links, path, package)
    records[record.package] = struct(
        name = record.name,
        repository = record.repository,
        package = record.package,
        files = depset(files.values()),
        links = tuple(sorted(links.items())),
        installs = tuple(sorted({path: True for path in previous.installs + record.installs})),
    )

def dependency_links(label, package, deps, aliases):
    """Binds workspace outputs; installed packages already have pnpm's layout.

    Args:
      label: The consuming target, for diagnostics.
      package: Its repository-relative directory.
      deps: Declared package and configuration-provider targets.
      aliases: Additional workspace import names mapped to package targets.

    Returns:
      Import-link paths mapped to canonical package directories.
    """
    imports = {}
    if not is_installed(package):
        for target in deps:
            if JsPackageInfo in target and not is_installed(target[JsPackageInfo].package):
                link(label, imports, target[JsPackageInfo].name, target[JsPackageInfo].package)
    for name, target in aliases.items():
        link(label, imports, name, target[JsPackageInfo].package)
    links = {}
    for name, target in imports.items():
        import_name(name)
        checked_path(target, allow_root = True)
        links[joined(package, "node_modules/" + name)] = target
    for target in deps:
        for provider in [JsTsconfigInfo, JsViteConfigInfo]:
            if provider in target:
                for path, bound in target[provider].links:
                    link(label, links, path, bound)
    return links

def config_providers(ctx, files, inputs, bases = []):
    """The bindings and providers shared by js_tsconfig and js_vite_config.

    Args:
      ctx: Rule context with config, deps and aliases attributes.
      files: depset of the configuration's own files and those it inherits.
      inputs: The configuration's own file-input targets, for runfiles and validation.
      bases: Extended configuration targets. Their package records and bindings
        join this configuration's and their validations propagate; their
        runfiles, which carry their sources, do not.

    Returns:
      A struct with packages (depset of merged package records), links (tuple
      of import bindings) and providers (DefaultInfo and validation outputs).
    """
    package_relative_path(ctx, ctx.file.config)  # Primary configs belong to their consumer's package.
    dependencies = ctx.attr.deps + ctx.attr.aliases.values()
    packages = package_records(dependencies + bases)
    runfiles = ctx.runfiles(transitive_files = depset(transitive = [files] + [record.files for record in packages.values()])).merge_all(
        [target[DefaultInfo].default_runfiles for target in inputs + dependencies],
    )
    return struct(
        packages = depset(packages.values()),
        links = tuple(sorted(dependency_links(ctx.label, ctx.label.package, ctx.attr.deps + bases, ctx.attr.aliases).items())),
        providers = [
            DefaultInfo(files = files, runfiles = runfiles),
            OutputGroupInfo(_validation = validation_outputs(inputs + dependencies + bases)),
        ],
    )

def validation_outputs(targets):
    """Retains validation stamps when a configured dependency is used as a tool.

    Args:
      targets: Dependencies whose validation groups must reach the consumer.

    Returns:
      A depset of stamps, separate from staged files and action inputs.
    """
    return depset(transitive = [
        target[OutputGroupInfo]._validation
        for target in targets
        if OutputGroupInfo in target and hasattr(target[OutputGroupInfo], "_validation")
    ])

def package_relative_path(ctx, file):
    """The path of one of this package's files relative to the package.

    Args:
      ctx: The rule context.
      file: A File of ctx's package.

    Returns:
      The path, such as src/index.ts.
    """
    package = ctx.label.package
    if file.owner.package != package or file.owner.workspace_name != ctx.label.workspace_name:
        fail("%s is not in package //%s" % (file.short_path, package))
    path = repository_path(file)
    return path[len(package) + 1:] if package else path

def repository_path(file):
    """The path inside a File's repository, also used in the staged Node layout.

    Args:
      file: A source or generated File.

    Returns:
      Its path without Bazel's external-repository runfiles prefix.
    """
    path = file.short_path
    if path.startswith("../"):
        path = path.split("/", 2)[2]
    return checked_path(path)

def relpath(from_dir, to_path):
    """The relative path of to_path from the directory from_dir.

    skylib's paths.relativize never climbs, and a link from one package to
    another does.

    Args:
      from_dir: A directory path.
      to_path: A path under the same root.

    Returns:
      The path, with a ".." for each directory to leave.
    """
    from_parts = [part for part in from_dir.split("/") if part and part != "."]
    to_parts = [part for part in to_path.split("/") if part and part != "."]
    common = 0
    for from_part, to_part in zip(from_parts, to_parts):
        if from_part != to_part:
            break
        common += 1
    parts = [".."] * (len(from_parts) - common) + to_parts[common:]
    if not parts:
        return "."
    return "/".join(parts)

def is_installed(package):
    """Whether a Bazel package is an installed npm package.

    Installed packages sit under a node_modules/ directory, at the root or in
    a workspace package's own node_modules/; every other js_package is a
    workspace package.

    Args:
      package: A Bazel package path.

    Returns:
      True for an installed npm package.
    """
    return ("/" + package + "/").find("/node_modules/") >= 0

def joined(package, path):
    """Joins a repository package and a relative path, including the root package."""
    return package + "/" + path if package else path

def checked_path(path, allow_root = False):
    """Normalizes a relative slash path without allowing escape from its package.

    Args:
      path: A relative path using slash separators.
      allow_root: Whether the containing directory itself is allowed.

    Returns:
      The normalized relative path.
    """
    if path.startswith("/") or "\\" in path or ":" in path:
        fail("expected a relative slash path, got %r" % path)
    normalized = paths.normalize(path)
    if normalized == ".." or normalized.startswith("../") or (normalized == "." and not allow_root):
        fail("path %r leaves or replaces its containing directory" % path)
    return normalized

def import_name(name):
    """Validates one unscoped or scoped node_modules import name.

    Args:
      name: A package import name, such as lodash or @types/node.

    Returns:
      The validated name, unchanged.
    """
    parts = name.split("/")
    expected = 2 if name.startswith("@") else 1
    if len(parts) != expected or any([part in ["", ".", "..", "@"] for part in parts]) or "\\" in name or ":" in name:
        fail("invalid package import name %r" % name)
    return name
