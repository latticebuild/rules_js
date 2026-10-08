"""Refusals of ambiguous or escaping tree layouts."""

load("@bazel_skylib//lib:partial.bzl", "partial")
load("@bazel_skylib//lib:unittest.bzl", "analysistest", "asserts", "unittest")
load("//private:layout.bzl", "merge_record", "tree_layout")

visibility("private")

def tree_test_suite(name):
    """Covers ambiguous destinations and invalid package installs.

    Args:
      name: The test suite name.
    """
    failures = []
    for case, message in {
        "alias_directory": "overlapping tree destinations",
        "alias_file": "overlapping tree destinations",
        "alias_prefix": "overlapping tree destinations",
        "conflicting_installs": "resolves to both",
        "direct_link": "resolves to both",
        "direct_link_file": "overlapping tree destinations",
        "extra_contents": "is both",
        "extra_file": "overlapping tree destinations",
        "repository_collision": "conflicting package repositories",
    }.items():
        target = name + "_" + case
        _invalid_layout(name = target, case = case, tags = ["manual"])
        failures.append(partial.make(_layout_failure_test, target_under_test = ":" + target, message = message))
    for case, message in {
        "escaping_install": "leaves or replaces its containing directory",
        "noninstall": "normalized node_modules package entry",
        "nonpackage_install": "invalid package import name",
        "unnormalized_install": "normalized node_modules package entry",
    }.items():
        failures.append(partial.make(_layout_failure_test, target_under_test = "//testdata/tree:" + case, message = message))
    unittest.suite(name, *failures)

def _invalid_layout_impl(ctx):
    owner = _record("owner", "node_modules/owner", ["package.json"], installs = ["node_modules/shared"])
    other = {
        "alias_directory": _record("other", "node_modules/shared", ["package.json"]),
        "alias_file": _record("other", "node_modules", ["shared"]),
        "alias_prefix": _record("other", "node_modules/other", ["package.json"], installs = ["node_modules/shared/node_modules/nested"]),
        "conflicting_installs": _record("other", "node_modules/other", ["package.json"], installs = ["node_modules/shared"]),
    }.get(ctx.attr.case, _record("other", "node_modules/other", ["package.json"]))
    files = []
    links = {}
    if ctx.attr.case == "extra_file":
        files = [_file("node_modules/shared/index.js")]
    elif ctx.attr.case == "extra_contents":
        files = [struct(short_path = "node_modules/owner/package.json", path = "other/package.json", is_directory = False)]
    elif ctx.attr.case == "direct_link":
        links["node_modules/shared"] = other.package
    elif ctx.attr.case == "direct_link_file":
        links["node_modules/owner/package.json"] = other.package
    if ctx.attr.case == "repository_collision":
        other = _record("owner", "node_modules/owner", ["package.json"], repository = "another")
    packages = {}
    for record in [owner, other]:
        merge_record(packages, record)
    tree_layout(ctx.label, packages, files = files, links = links)
    return [DefaultInfo()]

_invalid_layout = rule(implementation = _invalid_layout_impl, attrs = {"case": attr.string()})

def _layout_failure_test_impl(ctx):
    env = analysistest.begin(ctx)
    asserts.expect_failure(env, ctx.attr.message)
    return analysistest.end(env)

_layout_failure_test = analysistest.make(_layout_failure_test_impl, expect_failure = True, attrs = {"message": attr.string()})

def _file(short_path, repository = ""):
    return struct(
        short_path = "../" + repository + "/" + short_path if repository else short_path,
        path = "external/" + repository + "/" + short_path if repository else short_path,
        owner = struct(workspace_name = repository),
        is_directory = False,
    )

def _record(name, package, files = [], installs = [], repository = ""):
    return struct(
        name = name,
        package = package,
        repository = repository,
        files = depset([_file(package + "/" + path, repository = repository) for path in files]),
        links = (),
        installs = tuple(installs),
    )
