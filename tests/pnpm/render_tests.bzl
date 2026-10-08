"""Installation ownership controls generated visibility and workspace aliases."""

load("@bazel_skylib//lib:unittest.bzl", "asserts", "unittest")
load("//js/private/pnpm:render.bzl", "package_build", "workspace_index")

def render_test_suite(name):
    """Check main and external installation ownership.

    Args:
        name: Suite name.
    """
    unittest.suite(name, _owner_visibility_test, _workspace_alias_test)

def _owner_visibility_test_impl(ctx):
    env = unittest.begin(ctx)
    module = {
        "bins": [("compiler", "bin_", "cli.js")],
        "edges": {"always": [], "when": {}},
        "helper": False,
        "installs": {"node_modules/compiler": True},
        "license": "",
        "license_expression": "",
        "metadata": "package_metadata",
        "name": "compiler",
        "namespace": "",
        "published_name": "compiler",
        "rel": "node_modules/compiler",
        "target": "compiler",
        "version": "1.0.0",
    }
    for owner in ["@//:__subpackages__", "@@prepared+//:__subpackages__"]:
        generated = package_build(module, owner)

        # Both the executable and package stay usable by the installation owner.
        asserts.equals(env, 2, generated.count('visibility = ["//:__subpackages__", %r]' % owner))
        asserts.true(env, "visibility = [%r])" % owner in generated)
        asserts.false(env, "//visibility:public" in generated)
    return unittest.end(env)

_owner_visibility_test = unittest.make(_owner_visibility_test_impl)

def _workspace_alias_test_impl(ctx):
    env = unittest.begin(ctx)
    for owner, expected in [("@", "//lib:js"), ("@@prepared+", "@@prepared+//lib:js")]:
        dependency = owner + "//lib:js"
        modules = [
            {
                "development": {},
                "edges": {"always": [dependency], "when": {}},
                "imports": {"renamed": 1},
                "installs": {},
                "manifest": "app/package.json",
                "name": "app",
                "rel": "app",
                "target": "js",
                "workspace": True,
            },
            {
                "edges": {"always": [], "when": {}},
                "imports": {},
                "installs": {},
                "label": dependency,
                "manifest": "lib/package.json",
                "name": "library",
                "rel": "lib",
                "target": "js",
                "workspace": True,
            },
        ]
        index = workspace_index(modules, "npm", {})
        asserts.equals(env, {"renamed": expected}, index["packages"]["app"]["aliases"])
        asserts.equals(env, [], index["packages"]["app"]["deps"]["always"])
        modules[0]["imports"] = {"library": 1}
        index = workspace_index(modules, "npm", {})
        asserts.equals(env, {}, index["packages"]["app"]["aliases"])
        asserts.equals(env, [expected], index["packages"]["app"]["deps"]["always"])
    return unittest.end(env)

_workspace_alias_test = unittest.make(_workspace_alias_test_impl)
