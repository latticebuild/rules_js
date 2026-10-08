"""js_prettier_test: Prettier's formatting check, with a twin that formats in place."""

load("//js:providers.bzl", "JsBinaryInfo")
load("//js/support:execution.bzl", "EXECUTABLE_TOOLCHAINS", "PACKAGE_EXECUTABLE_ATTRS", "TEST_RUNTIME_ATTRS", "package_executable")
load("//js/support:fix.bzl", "fix_name")
load("//js/support:layout.bzl", "checked_path")
load(":prettier_fix.bzl", "js_prettier_fix")

visibility("//...")

def js_prettier_test(name, paths = None, prettier = None, env = None, **kwargs):
    """Checks formatting with Prettier and declares `<name minus _test>_fix`.

    `bazel run` of the twin formats the same paths in the invoking workspace.
    Plugins that a Prettier configuration names are staged through `data`.

    Args:
      name: Test name, ending in `_test`.
      paths: Operands relative to the package; defaults to the whole package.
      prettier: The Prettier command.
      env: Runtime environment overrides, shared with the twin.
      **kwargs: Staged inputs and standard test attributes; the twin shares
        the test's visibility.
    """
    shared = {key: value for key, value in {"env": env, "paths": paths, "prettier": prettier}.items() if value != None}
    js_prettier_fix(name = fix_name(name), tags = kwargs.get("tags", []), target_compatible_with = [], visibility = kwargs.get("visibility"), **shared)
    _js_prettier_test(name = name, **(_DEFAULTS | kwargs | shared))

# Preparing the package tree dominates a check test's run time: seconds alone,
# but staging it came close to the moderate limit when many ran at once.
_DEFAULTS = {"timeout": "long"}

_ATTRS = {
    "paths": attr.string_list(default = ["."], doc = "Operands relative to the package."),
    "prettier": attr.label(
        doc = "The Prettier command.",
        mandatory = True,
        providers = [JsBinaryInfo],
        executable = True,
        cfg = "target",
    ),
}

def _js_prettier_test_impl(ctx):
    return package_executable(ctx, ctx.attr.prettier, args = ["--check"] + [checked_path(path, allow_root = True) for path in ctx.attr.paths], env = ctx.attr.env, test = True)

_js_prettier_test = rule(
    implementation = _js_prettier_test_impl,
    doc = "Runs `prettier --check` on the paths in the staged package.",
    test = True,
    toolchains = EXECUTABLE_TOOLCHAINS,
    attrs = PACKAGE_EXECUTABLE_ATTRS | _ATTRS | TEST_RUNTIME_ATTRS,
)
