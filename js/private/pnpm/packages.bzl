"""Installed manifest interpretation, target allocation, and executable metadata."""

load("@bazel_skylib//lib:paths.bzl", "paths")
load(":discovery.bzl", "directory_entries", "relative_path")

visibility("//...")

def describe_packages(ctx, root, modules, budget, spdx, owner):
    """Attach package names, labels, and metadata to discovered records.

    Args:
        ctx: Repository context that watches executable metadata inputs.
        root: Canonical installation path.
        modules: Mutable records returned by discovery, updated in place.
        budget: Traversal budget shared with discovery and inventory.
        spdx: Pinned set of supported SPDX identifiers.
        owner: Canonical installation repository prefix, or @ for the main repository.
    """
    for module in modules:
        raw = module["raw"]
        rel = module["rel"]
        if module["workspace"]:
            module["name"] = raw.get("name", "")
            module["label"] = owner + package_label(rel, module["target"]) if module["target"] else ""
            continue
        module["name"] = rel.split("/node_modules/")[-1].removeprefix("node_modules/")
        published_name = raw.get("name")
        if type(published_name) != "string" or not published_name:
            published_name = module["name"]
        namespace, _, name = published_name.partition("/")
        if not namespace.startswith("@") or not name:
            namespace, name = "", paths.basename(published_name)
        root_files = [child.basename for child in directory_entries(root, module["dir"], budget, 0) if not child.is_dir]
        taken = {name: True for name in root_files}
        module["target"] = _free_name(taken, [paths.basename(rel), "pkg"])
        module["metadata"] = _free_name(taken, ["package_metadata"])
        module["license_target"] = _free_name(taken, ["package_license"])
        module["license_kind_target"] = _free_name(taken, ["package_license_kind"])
        module["bins"] = _bins(ctx, root, module, name, taken, budget)
        module["sources"] = _free_name(taken, ["package_sources"])
        module["label"] = package_label(rel, module["target"])
        module["namespace"] = namespace
        module["published_name"] = name
        module["version"] = raw.get("version") if type(raw.get("version")) == "string" else ""
        module["license"] = _license_id(raw, spdx)
        module["license_file"] = _license_file(root_files)
        module["license_expression"] = "" if module["license"] else _license_expression(raw, module["license_file"])

def _license_id(raw, spdx):
    license = _declared_license(raw)
    return license if license in spdx else ""

def _license_expression(raw, license_file):
    # Anything but one listed SPDX id stays verbatim for the supply-chain
    # check to parse or a reviewed override to cover. A package that declares
    # nothing but ships a licence file asserts no licence; its text travels.
    license = _declared_license(raw)
    if license:
        return license
    return "NOASSERTION" if license_file else ""

def _declared_license(raw):
    license = raw.get("license")
    if license == None:
        legacy = raw.get("licenses")
        if type(legacy) == "list" and len(legacy) == 1:
            license = legacy[0]
    if type(license) == "dict":
        license = license.get("type")
    return license if type(license) == "string" and not license.startswith("#") else ""

def _free_name(taken, candidates):
    # Preserve the installed-label convention when choosing automatic names.
    first_chars = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ_"
    remaining_chars = first_chars + "0123456789.=-"
    for candidate in candidates:
        if candidate not in taken and candidate and candidate[0] in first_chars and all([char in remaining_chars for char in candidate.elems()]):
            taken[candidate] = True
            return candidate
    for suffix in range(1, len(taken) + 2):
        candidate = candidates[-1] + "_" * suffix
        if candidate not in taken:
            taken[candidate] = True
            return candidate
    fail("Could not allocate package target")

def _bins(ctx, root, module, package_name, taken, budget):
    raw = module["raw"]
    value = raw.get("bin")
    commands = {}
    if type(value) == "string":
        commands[package_name] = value
    elif type(value) == "dict":
        commands = {paths.basename(name): script for name, script in value.items() if type(script) == "string"}
    elif value == None:
        directories = raw.get("directories")
        directory = directories.get("bin") if type(directories) == "dict" else None
        if type(directory) == "string" and _package_path(directory):
            source = module["dir"].get_child(directory)
            ctx.watch(source)
            if source.exists and source.is_dir:
                for child in directory_entries(root, source, budget, 0):
                    if not child.is_dir and not child.basename.startswith("."):
                        commands[child.basename] = paths.join(directory, child.basename)
    bins = []
    for command in sorted(commands):
        script = commands[command]
        if not _package_path(script):
            continue
        script = paths.normalize(script)
        source = module["dir"].get_child(script)
        ctx.watch(source)
        if not source.exists or source.is_dir:
            continue
        relative_path(root, source.realpath)
        contents = ctx.read(source)
        if not _node_script(script, contents[:8192]):
            continue
        bins.append((command, _free_name(taken, [command, "bin"]), script))
    return sorted(bins)

def _package_path(path):
    clean = paths.normalize(path)
    return clean not in ["", ".", ".."] and not clean.startswith(("/", "../")) and "\\" not in clean and ":" not in clean

def _node_script(script, head):
    if "\000" in head:
        return False
    if head.startswith("#!"):
        words = [word for word in head.split("\n")[0][2:].replace("\t", " ").replace("\r", " ").split(" ") if word]
        if not words:
            return False
        interpreter = paths.basename(words[0])
        if interpreter in ["node", "nodejs"]:
            return True
        if interpreter != "env":
            return False
        for word in words[1:]:
            if word == "-S" or "=" in word:
                continue
            return word in ["node", "nodejs"]
        return False
    return paths.split_extension(script)[1] in [".js", ".mjs", ".cjs"]

def _license_file(files):
    for name in sorted(files):
        lower = name.lower()
        if paths.split_extension(lower)[1] in [".js", ".cjs", ".mjs", ".ts", ".json"]:
            continue
        for prefix in ["license", "licence", "copying", "unlicense"]:
            if lower == prefix or (lower.startswith(prefix) and lower[len(prefix)] in ".-_"):
                return name
    return ""

def package_label(rel, target):
    """Use the shorthand only when it names the actual allocated target."""
    if not rel:
        return "//:" + target
    return "//" + rel + ("" if target == paths.basename(rel) else ":" + target)
