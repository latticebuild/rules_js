"""Package resolution refuses missing, cyclic, escaping and unbound dependencies."""

load("@bazel_skylib//lib:partial.bzl", "partial")
load("@bazel_skylib//lib:unittest.bzl", "analysistest", "asserts", "unittest")
load("//private/pnpm:graph.bzl", "resolve_graph")
load(":packages.bzl", "package_records")

def graph_test_suite(name):
    """Check dependency refusals and permitted missing peers.

    Args:
        name: Name of the test suite and prefix for its test subjects.
    """
    failures = []
    for case, records, message in [
        ("missing_required", [
            {"raw": {"dependencies": {"gone": "*"}}, "rel": "node_modules/a"},
        ], "required dependency gone"),
        ("workspace_cycle", [
            {"raw": {"dependencies": {"b": "workspace:*"}}, "rel": "a", "workspace": True},
            {"raw": {"dependencies": {"a": "workspace:*"}}, "rel": "b", "workspace": True},
        ], "Workspace compilation cycle"),
        ("npm_peer_cycle", [
            {"installs": {"node_modules/a": True}, "raw": {"dependencies": {"b": "*"}}, "rel": "a", "workspace": True},
            {"raw": {"peerDependencies": {"a": "*"}}, "rel": "node_modules/b"},
        ], "Workspace compilation cycle"),
        ("escaping_workspace", [
            {"raw": {"dependencies": {"b": "workspace:../../b"}}, "rel": "a", "workspace": True},
        ], "escapes the repository"),
        ("installed_as_workspace", [
            {"raw": {"dependencies": {"b": "workspace:../node_modules/b"}}, "rel": "a", "workspace": True},
            {"rel": "node_modules/b"},
        ], "names an installed package"),
        ("unbound_workspace_peer", [
            {"raw": {"peerDependencies": {"b": "*"}}, "rel": "node_modules/a"},
            {"installs": {"node_modules/b": True}, "rel": "b", "target": "", "workspace": True},
        ], "has no declared js_package"),
    ]:
        target = name + "_" + case
        _invalid_graph(name = target, records = json.encode(records), tags = ["manual"])
        failures.append(partial.make(_graph_failure_test, target_under_test = ":" + target, message = message))
    unittest.suite(name, *(failures + [_peer_diagnostics_test]))

def _peer_diagnostics_test_impl(ctx):
    env = unittest.begin(ctx)
    modules = package_records([
        {
            "raw": {
                "dependencies": {"b": "*"},
                "peerDependencies": {"b": "*", "missing": "*", "optional": "*"},
                "peerDependenciesMeta": {"optional": {"optional": True}},
            },
            "rel": "node_modules/a",
        },
        {"rel": "node_modules/b"},
    ])
    asserts.equals(env, ["node_modules/a requires peer missing; no installation resolves it, so its consumer must provide it"], resolve_graph(modules))
    asserts.equals(env, {"b": 1}, modules[0]["imports"])
    asserts.equals(env, {}, modules[1]["imports"])
    return unittest.end(env)

_peer_diagnostics_test = unittest.make(_peer_diagnostics_test_impl)

def _invalid_graph_impl(ctx):
    resolve_graph(package_records(json.decode(ctx.attr.records)))
    return []

_invalid_graph = rule(implementation = _invalid_graph_impl, attrs = {"records": attr.string(mandatory = True)})

def _graph_failure_test_impl(ctx):
    env = analysistest.begin(ctx)
    asserts.expect_failure(env, ctx.attr.message)
    return analysistest.end(env)

_graph_failure_test = analysistest.make(_graph_failure_test_impl, expect_failure = True, attrs = {"message": attr.string(mandatory = True)})
