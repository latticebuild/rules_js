"""Node lookup and complete per-platform dependency closures."""

load("@bazel_skylib//lib:paths.bzl", "paths")
load(":packages.bzl", "package_label")

PLATFORMS = [
    ("darwin", "arm64", ""),
    ("darwin", "x64", ""),
    ("linux", "arm64", "glibc"),
    ("linux", "arm64", "musl"),
    ("linux", "x64", "glibc"),
    ("linux", "x64", "musl"),
    ("win32", "arm64", ""),
    ("win32", "x64", ""),
]

def resolve_graph(modules):
    """Resolve imports once and attach acyclic closures and workspace bindings.

    Args:
        modules: Mutable physical package records with names and allocated labels.
            Receives imports, platform edges and the source-helper requirement.

    Returns:
        Nonfatal diagnostics for missing peers that consumers must provide.
    """
    diagnostics = []
    by_path = {}
    workspaces = {}
    for i, module in enumerate(modules):
        by_path[module["rel"]] = i
        for install in module["installs"]:
            by_path[install] = i
        if module["workspace"]:
            name = module["name"]
            if name in workspaces and modules[workspaces[name]]["rel"] != module["rel"]:
                fail("Duplicate workspace package name: %s" % name)
            workspaces[name] = i
        module["helper"] = False
        module["imports"] = {}
    candidates = []
    for module in modules:
        edges = []
        raw = module["raw"]
        required = _mapping(raw, "dependencies", module["workspace"])
        optional = _mapping(raw, "optionalDependencies", module["workspace"])
        peers = _mapping(raw, "peerDependencies", module["workspace"])
        peer_meta = _mapping(raw, "peerDependenciesMeta", False)
        names = dict(required)
        names.update(optional)
        for name, value in peers.items():
            meta = peer_meta.get(name)
            if not (type(meta) == "dict" and meta.get("optional") == True) and name not in names:
                names[name] = value
        if module["workspace"] and not module["target"]:
            names = {}
        for name in sorted(names):
            to = _resolve(module["rel"], name, by_path)
            spec = names[name]
            if module["workspace"] and spec.startswith("workspace:"):
                to = _workspace_dependency(module, name, spec, by_path, workspaces)
                if to != None and not modules[to]["workspace"]:
                    fail("%s: workspace dependency %s names an installed package" % (module["rel"], spec))
            if to == None:
                if name in optional:
                    continue
                if name in peers and name not in required:
                    diagnostics.append("%s requires peer %s; no installation resolves it, so its consumer must provide it" % (module["rel"], name))
                    continue
                fail("%s: required dependency %s is not installed where Node resolves it" % (module["rel"], name))
            target = modules[to]
            if target["workspace"] and not target["label"]:
                fail("%s: workspace dependency %s (%s) has no declared js_package" % (module["rel"], name, target["rel"]))
            edges.append(to)
            module["imports"][name] = to
        candidates.append(edges)
    per_platform = [[] for _ in modules]
    guarded = [False for _ in modules]
    for platform in PLATFORMS:
        graph = [[to for to in edges if _enabled(modules[to], platform)] if _enabled(modules[i], platform) else [] for i, edges in enumerate(candidates)]
        components, membership = _components(graph)
        for i, module in enumerate(modules):
            component = components[membership[i]]
            guarded[i] = guarded[i] or any([
                _restricted(modules[member]) or any([_restricted(modules[to]) for to in candidates[member]])
                for member in component
            ])
            labels = {}
            if len(component) > 1 or i in graph[i]:
                workspace_names = [modules[member]["name"] for member in component if modules[member]["workspace"]]
                if workspace_names:
                    fail("Workspace compilation cycle on %s involving %s" % (setting(platform), ", ".join(workspace_names)))
                for member in component:
                    modules[member]["helper"] = True
                    if member != i:
                        target = modules[member]
                        labels[package_label(target["rel"], target["sources"])] = True
            for member in component:
                for to in graph[member]:
                    if membership[to] != membership[i]:
                        labels[modules[to]["label"]] = True
            per_platform[i].append(sorted(labels))
    for i, module in enumerate(modules):
        module["edges"] = platform_edges(per_platform[i], guarded[i])

    # Build/test imports are lookup bindings, never runtime graph edges. In
    # particular, a development cycle must not create a compilation cycle.
    root = None
    for module in modules:
        if module["rel"] == "":
            root = module
    root_development = _mapping(root["raw"], "devDependencies", True) if root else {}
    for module in modules:
        module["development"] = {}
        if not module["workspace"]:
            continue
        names = dict(root_development)
        for field in ["dependencies", "optionalDependencies", "peerDependencies", "devDependencies"]:
            names.update(_mapping(module["raw"], field, True))
        for name in sorted(names):
            to = _resolve(module["rel"], name, by_path)
            if names[name].startswith("workspace:"):
                to = _workspace_dependency(module, name, names[name], by_path, workspaces)
            if to != None:
                target = modules[to]
                module["development"][name] = {
                    "label": target["label"],
                    "name": target["name"],
                    "path": target["rel"],
                    "platforms": [setting(platform) for platform in PLATFORMS if _enabled(target, platform)],
                    "workspace": target["workspace"],
                    "binaries": {command: package_label(target["rel"], allocated) for command, allocated, _ in target.get("bins", [])},
                }
    return diagnostics

def _mapping(raw, key, strict):
    value = raw.get(key, {})
    if type(value) != "dict":
        if strict:
            fail("Workspace package.json %s must be an object" % key)
        return {}
    if strict and any([type(spec) != "string" for spec in value.values()]):
        fail("Workspace package.json %s entries must be strings" % key)
    return {name: value[name] for name in value if name}

def _resolve(rel, name, by_path):
    parts = rel.split("/") if rel else []
    for size in range(len(parts), -1, -1):
        candidate = paths.join("/".join(parts[:size]), "node_modules", name)
        if candidate in by_path:
            return by_path[candidate]
    return None

def _workspace_dependency(module, name, spec, by_path, workspaces):
    value = spec.removeprefix("workspace:")
    if value in [".", ".."] or value.startswith(("./", "../")):
        target = paths.normalize(paths.join(module["rel"], value))
        if target == ".." or target.startswith("../") or paths.is_absolute(target):
            fail("Workspace dependency escapes the repository: %s" % spec)
        return by_path.get("" if target == "." else target)
    alias = value.rsplit("@", 1)
    if len(alias) == 2 and alias[0]:
        name = alias[0]
    return workspaces.get(name)

def _components(graph):
    # Iterative Kosaraju avoids Starlark recursion, preserving O(V + E) work.
    limit = len(graph) + 1
    for edges in graph:
        limit += len(edges)
    visited = {}
    order = []
    reverse = [[] for _ in graph]
    for source, edges in enumerate(graph):
        for target in edges:
            reverse[target].append(source)
        pending = [(source, False)]
        for _ in range(2 * limit):
            if not pending:
                break
            node, complete = pending.pop()
            if complete:
                order.append(node)
            elif node not in visited:
                visited[node] = True
                pending.append((node, True))
                pending.extend([(target, False) for target in graph[node] if target not in visited])
        if pending:
            fail("Dependency graph traversal exceeded its edge bound")
    membership = {}
    components = []
    for source in reversed(order):
        if source in membership:
            continue
        members = []
        pending = [source]
        membership[source] = len(components)
        for _ in range(limit):
            if not pending:
                break
            node = pending.pop()
            members.append(node)
            for target in reverse[node]:
                if target not in membership:
                    membership[target] = len(components)
                    pending.append(target)
        if pending:
            fail("Dependency component traversal exceeded its edge bound")
        components.append(sorted(members))
    return components, membership

def installs(raw, platform):
    """Evaluate npm's positive/negative OS, CPU and Linux-only libc restrictions."""
    return all([_allows(_strings(raw.get(field)), value) for field, value in zip(["os", "cpu", "libc"], platform) if field != "libc" or platform[0] == "linux"])

def _strings(value):
    if type(value) == "string":
        return [value]
    if type(value) == "list" and all([type(item) == "string" for item in value]):
        return value
    return []

def _allows(entries, value):
    if "!" + value in entries:
        return False
    positive = [entry for entry in entries if not entry.startswith("!")]
    return not positive or value in positive

def _enabled(module, platform):
    return module["workspace"] or installs(module["raw"], platform)

def _restricted(module):
    return not module["workspace"] and any([_strings(module["raw"].get(field)) for field in ["os", "cpu", "libc"]])

def platform_edges(values, restricted):
    """Factor common labels while retaining guards on unsupported platforms.

    Args:
        values: Label lists in PLATFORMS order.
        restricted: Whether a platform restriction requires explicit select guards.

    Returns:
        Common labels and per-platform select branches, without a default branch.
    """
    common = {label: True for label in values[0]}
    for labels in values[1:]:
        common = {label: True for label in common if label in labels}
    branches = {setting(platform): [label for label in labels if label not in common] for platform, labels in zip(PLATFORMS, values)}
    return {"always": sorted(common), "when": branches if restricted or any(branches.values()) else {}}

def setting(platform):
    """The copied toolkit's config_setting label for one supported platform."""
    return str(Label("//platforms:" + "_".join([part for part in platform if part])))
