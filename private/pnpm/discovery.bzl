"""Discover declared workspace bindings and pnpm's physical installations."""

load("@bazel_skylib//lib:paths.bzl", "paths")
load(":selection.bzl", "compile_patterns", "matches", "repository_selection")

def discover(ctx, root):
    """Return physical package records, including declared and linked workspaces.

    Args:
        ctx: Repository context used to watch installation inputs.
        root: Canonical main-workspace path.

    Returns:
        Sorted package records, the shared traversal budget, and workspace metadata.
    """
    budget = [0]
    workspace, install_roots, config = _workspace(ctx, root, budget)
    root_install = root.get_child("node_modules")
    ctx.watch(root_install)
    if not root_install.exists or not root_install.is_dir:
        fail("node_modules directory does not exist: %s; run pnpm install" % root_install)
    modules = dict(workspace)
    pending = [(directory, False, 0) for directory in install_roots]
    visited = {}
    for _ in range(1000000):
        if not pending:
            break
        directory, scope, depth = pending.pop()
        physical = directory.realpath
        relative_path(root, physical)
        key = (str(physical), scope)
        if key in visited:
            continue
        visited[key] = True
        for child in directory_entries(root, physical, budget, depth):
            if child.basename.startswith(".") or not child.is_dir:
                continue
            resolved = child.realpath
            if resolved != child:
                ctx.watch(child)

            # An external development link cannot supply declared package inputs.
            if not str(resolved).startswith(str(root) + "/") and resolved != root:
                continue
            logical = relative_path(root, directory.get_child(child.basename))
            rel = relative_path(root, resolved)
            if not scope and child.basename.startswith("@"):
                pending.append((directory.get_child(child.basename), True, depth + 1))
                continue
            if rel in modules:
                modules[rel]["installs"][logical] = True
                continue
            manifest = resolved.get_child("package.json")
            ctx.watch(manifest)
            if not manifest.exists:
                continue
            contents, raw = read_manifest(ctx, root, manifest)
            source = "/node_modules/" not in "/" + rel + "/"
            modules[rel] = {
                "dir": resolved,
                "installs": {install: True for install in [rel, logical]},
                "manifest": contents,
                "raw": raw,
                "rel": rel,
                "target": "",
                "workspace": source,
            }
            if not source:
                nested = resolved.get_child("node_modules")
                ctx.watch(nested)
                if nested.exists and nested.is_dir:
                    pending.append((nested, False, depth + 1))
    if pending:
        fail("pnpm discovery exceeds 1000000 directories")
    return [modules[rel] for rel in sorted(modules)], budget, config

def _workspace(ctx, root, budget):
    config = _workspace_patterns(ctx, root)
    selection = repository_selection(ctx, root)
    config["selection"] = selection
    includes, excludes = workspace_patterns(config["packages"])
    packages = {}
    installs = {}
    repository_exclusions = []
    for pattern in selection["directories"]:
        repository_exclusions.extend(compile_patterns(pattern))
    pending = [(root, [], 0)]
    for _ in range(1000000):
        if not pending:
            break
        directory, inherited, depth = pending.pop()
        rel = relative_path(root, directory)
        if matches(repository_exclusions, rel) or rel in selection["paths"]:
            continue
        entries = directory_entries(root, directory, budget, depth)
        exclusions = list(inherited)
        target = ""
        build_ignored = paths.join(rel, "BUILD.bazel") in selection["paths"]
        selected = not matches(inherited, paths.join(rel, "BUILD.bazel")) and not build_ignored
        build = directory.get_child("BUILD.bazel")
        ctx.watch(build)
        if build.exists and selected:
            relative_path(root, build.realpath)
            for directive, value in _directives(ctx.read(build)):
                if directive == "gazelle:exclude":
                    exclusions.extend(compile_patterns(paths.normalize(paths.join(rel, value))))
                elif directive == "gazelle:js_package":
                    if target:
                        fail("Duplicate gazelle:js_package directive in %s" % build)
                    if not target_name(value):
                        fail("Invalid gazelle:js_package target %r in %s" % (value, build))
                    target = value
        member = workspace_member(includes, excludes, rel)
        if target and not member:
            fail("Declared js_package is outside pnpm workspace membership: %s" % directory)
        manifest = directory.get_child("package.json")
        if member:
            ctx.watch(manifest)
        if member and manifest.exists:
            contents, raw = read_manifest(ctx, root, manifest)
            if not target and (raw.get("private") != True or "files" in raw or any([raw.get(key) for key in ["main", "module", "types", "typings", "exports", "bin", "svelte"]])):
                target = "pkg" if rel == "" or directory.basename == "lib" else directory.basename
            if target and not target_name(target):
                fail("Workspace package needs an explicit gazelle:js_package target: %s" % directory)
            target_path = directory.get_child(target)
            ctx.watch(target_path)
            if target and target_path.exists and not target_path.is_dir:
                fail("Workspace target %s collides with a file in %s" % (target, directory))
            if type(raw.get("name")) != "string" or not raw["name"]:
                fail("Workspace package has no name: %s" % directory)
            packages[rel] = {
                "dir": directory,
                "installs": {},
                "manifest": contents,
                "member": True,
                "raw": raw,
                "rel": rel,
                "target": target,
                "workspace": True,
            }

        # A handwritten application may own nested installed versions without
        # exposing a generated package. Its installation is still authoritative.
        installed = directory.get_child("node_modules")
        ctx.watch(installed)
        if installed.exists and installed.is_dir:
            relative_path(root, installed.realpath)
            installs[str(installed)] = installed
        for child in entries:
            # Gazelle itself excludes Git metadata; pnpm installations have
            # separate indexed package ownership, never workspace membership.
            if child.basename in [".git", "node_modules"]:
                continue
            child_rel = paths.join(rel, child.basename)
            if child.is_dir and child.realpath == child and not matches(exclusions, child_rel):
                pending.append((child, exclusions, depth + 1))
    if pending:
        fail("Workspace discovery exceeds 1000000 directories")
    return packages, installs.values(), config

def _workspace_patterns(ctx, root):
    manifest = root.get_child("pnpm-workspace.yaml")
    ctx.watch(manifest)
    if not manifest.exists:
        return {"architectures": {}, "packages": [], "source": ""}
    relative_path(root, manifest.realpath)
    source = ctx.read(manifest)
    if len(source) > 128 * 1024 * 1024:
        fail("pnpm-workspace.yaml exceeds 128 MiB")
    operating_system = ctx.os.name.lower()
    operating_system = "darwin" if operating_system.startswith("mac") else ("windows" if operating_system.startswith("windows") else operating_system)
    architecture = {"aarch64": "arm64", "x86_64": "amd64"}.get(ctx.os.arch, ctx.os.arch)
    platform = operating_system + "_" + architecture
    binaries = [label for label, selected in ctx.attr._yq_binaries.items() if selected == platform]
    if not binaries:
        fail("No declared YAML reader for host platform %s" % platform)

    # Resolve the platform file directly: upstream's host symlink does not cause
    # its destination repository to be fetched during repository evaluation.
    result = ctx.execute([ctx.path(binaries[0]), "-o=json", ".", manifest], timeout = 30, quiet = True)
    if result.return_code:
        fail("Declared YAML reader failed for pnpm-workspace.yaml: %s" % result.stderr)
    if len(result.stdout) > 1024 * 1024:
        fail("pnpm workspace patterns exceed 1 MiB")
    decoded = json.decode(result.stdout)
    if type(decoded) != "dict":
        fail("pnpm-workspace.yaml must be a mapping")
    return {
        "architectures": decoded.get("supportedArchitectures", {}),
        "packages": decoded.get("packages", []),
        "source": source,
    }

def workspace_patterns(patterns):
    """Compile the YAML package selection, including negated patterns.

    Args:
        patterns: The decoded packages list.

    Returns:
        Positive and negative compiled patterns.
    """
    if type(patterns) != "list":
        fail("pnpm workspace packages must be a list")
    positive, negative = [], []
    for pattern in patterns:
        if type(pattern) != "string":
            fail("pnpm workspace packages must contain strings")
        excluded = pattern.startswith("!")
        value = pattern[1:] if excluded else pattern
        if not value or value.startswith("/") or "\\" in value or ":" in value or ".." in value.split("/"):
            fail("Invalid pnpm workspace pattern: %r" % pattern)
        selected = negative if excluded else positive
        selected.extend(compile_patterns(paths.normalize(value)))
    return positive, negative

def workspace_member(includes, excludes, rel):
    """Whether a relative directory is selected; the root is always a member.

    Args:
        includes: Compiled positive patterns.
        excludes: Compiled negative patterns.
        rel: Workspace-relative directory.

    Returns:
        Whether the directory belongs to the pnpm workspace.
    """
    return rel == "" or (matches(includes, rel) and not matches(excludes, rel))

def _directives(contents):
    # Only standalone, top-level comments are directives. A small lexical scan
    # avoids interpreting strings or comments inside rule calls as declarations;
    # Gazelle remains responsible for parsing and editing the BUILD syntax.
    result = []
    quote = ""
    escaped = False
    depth = 0
    for line in contents.splitlines():
        skip = 0
        for i, char in enumerate(line.elems()):
            if i < skip:
                continue
            if escaped:
                escaped = False
            elif char == "\\":
                escaped = True
            elif quote:
                if line[i:].startswith(quote):
                    skip = i + len(quote)
                    quote = ""
            elif char in ["'", '"']:
                quote = char * 3 if line[i:].startswith(char * 3) else char
                skip = i + len(quote)
            elif char == "#":
                if depth == 0 and not line[:i].strip():
                    text = line[i + 1:].strip().replace("\t", " ").replace("\r", " ").replace("\014", " ")
                    key, _, value = text.partition(" ")
                    if key in ["gazelle:js_package", "gazelle:exclude"]:
                        result.append((key, value.strip()))
                break
            elif char in ["(", "[", "{"]:
                depth += 1
            elif char in [")", "]", "}"]:
                depth -= 1
        escaped = False
    return result

def directory_entries(root, directory, budget, depth):
    """Read and watch a bounded directory, confined to the source workspace.

    Args:
        root: Canonical main-workspace path.
        directory: Directory to read after resolving its containment.
        budget: One-element list counting entries across all traversals.
        depth: Current traversal depth.

    Returns:
        Entries sorted by their complete path.
    """
    if depth > 128:
        fail("pnpm traversal exceeds 128 directories at %s" % directory)
    relative_path(root, directory.realpath)
    entries = directory.readdir(watch = "yes")
    budget[0] += len(entries)
    if budget[0] > 1000000:
        fail("pnpm traversal exceeds 1000000 entries at %s" % directory)
    return sorted(entries, key = str)

def read_manifest(ctx, root, file):
    """Read a watched JSON object without allowing an escaping manifest link.

    Args:
        ctx: Repository context that watches file reads.
        root: Canonical main-workspace path.
        file: Manifest path, possibly an in-workspace symlink.

    Returns:
        The original text, which the workspace index publishes, and the decoded object.
    """
    if not file.exists:
        fail("Missing package.json: %s" % file)
    relative_path(root, file.realpath)
    contents = ctx.read(file)
    if len(contents) > 128 * 1024 * 1024:
        fail("package.json exceeds 128 MiB: %s" % file)
    raw = json.decode(contents)
    if type(raw) != "dict":
        fail("package.json must be an object: %s" % file)
    return contents, raw

def relative_path(root, file):
    """Return a slash path inside root, rejecting escaping or ambiguous names.

    Args:
        root: Canonical main-workspace path.
        file: Absolute path whose containment is required.

    Returns:
        A repository-relative slash path, empty for the root itself.
    """
    if file == root:
        return ""
    prefix = str(root) + "/"
    if not str(file).startswith(prefix):
        fail("Installed path leaves the workspace: %s" % file)
    rel = str(file)[len(prefix):]
    if len(rel.split("/")) > 128 or "\\" in rel or ":" in rel or any([part in ["", ".", ".."] for part in rel.split("/")]):
        fail("Invalid pnpm path: %r" % rel)
    return rel

def target_name(value):
    """Whether a name is valid for a Bazel rule target.

    Args:
        value: Proposed target name, without a label prefix.

    Returns:
        Whether the name has valid characters and normalized path segments.
    """
    if any([part in ["", ".", ".."] for part in value.split("/")]):
        return False
    for char in value.elems():
        if char < " " or char in ["\177", ":", "\\"]:
            return False
    return True

def inventory(ctx, root, modules, budget):
    """Map repository destinations to source leaves without modifying the install.

    Args:
        ctx: Repository context that watches logical symlinks.
        root: Canonical main-workspace path.
        modules: Discovered physical package records; workspaces are omitted.
        budget: Traversal budget shared with discovery and metadata inspection.

    Returns:
        Repository-relative destinations mapped to canonical source paths.
    """
    files = {}
    pending = [(module["dir"], module["rel"], 0, ()) for module in modules if not module["workspace"]]
    for _ in range(1000000):
        if not pending:
            break
        directory, destination, depth, ancestors = pending.pop()
        physical = directory.realpath
        relative_path(root, physical)
        if physical in ancestors:
            fail("Package directory symlink cycle at %s" % directory)
        ancestors = ancestors + (physical,)
        for child in directory_entries(root, physical, budget, depth):
            if depth == 0 and child.basename == "node_modules":
                continue
            target = paths.join(destination, child.basename)
            resolved = child.realpath
            if resolved != child:
                ctx.watch(child)
            relative_path(root, resolved)
            if child.is_dir:
                pending.append((child, target, depth + 1, ancestors))
                continue
            if not child.exists:
                fail("Installed file is missing: %s" % child)
            if child.basename in ["BUILD", "BUILD.bazel", "WORKSPACE", "WORKSPACE.bazel", "MODULE.bazel", "REPO.bazel"]:
                target += ".upstream"
            if target in files and files[target] != resolved:
                fail("Conflicting pnpm repository destination: %s" % target)
            files[target] = resolved
    if pending:
        fail("pnpm inventory exceeds 1000000 directories")
    return files
