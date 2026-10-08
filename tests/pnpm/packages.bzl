"""Physical package records for the graph refusal tests."""

load("@bazel_skylib//lib:paths.bzl", "paths")

def package_records(records):
    """Fill discovery metadata while keeping each case's dependency graph visible.

    Args:
        records: Mutable package records with at least a repository-relative path.

    Returns:
        The same records with names, target labels, and empty metadata defaults.
    """
    for module in records:
        module.setdefault("workspace", False)
        module.setdefault("installs", {})
        module.setdefault("target", "pkg")
        module.setdefault("sources", "sources")
        module.setdefault("name", paths.basename(module["rel"]))
        module.setdefault("raw", {})
        prefix = "@" if module["workspace"] else ""
        module["label"] = "%s//%s:%s" % (prefix, module["rel"], module["target"]) if module["target"] else ""
    return records
