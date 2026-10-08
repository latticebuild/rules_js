"""js_oxfmt_test: Oxfmt's formatting check, with a twin that formats in place."""

load("//js:providers.bzl", "JsBinaryInfo")
load("//js/support:execution.bzl", "EXECUTABLE_TOOLCHAINS", "PACKAGE_EXECUTABLE_ATTRS", "package_executable")
load("//js/support:fix.bzl", "fix_name")
load("//js/support:layout.bzl", "checked_path")
load(":oxfmt_fix.bzl", "js_oxfmt_fix")

visibility("//...")

def js_oxfmt_test(name, paths = None, oxfmt = None, env = None, **kwargs):
    """Checks formatting with Oxfmt and declares `<name minus _test>_fix`.

    `bazel run` of the twin formats the same paths in the invoking workspace.

    Args:
      name: Test name, ending in `_test`.
      paths: Operands relative to the package; defaults to the whole package.
      oxfmt: The Oxfmt command.
      env: Runtime environment overrides, shared with the twin.
      **kwargs: Staged inputs and standard test attributes; the twin shares
        the test's visibility.
    """
    shared = {key: value for key, value in {"env": env, "oxfmt": oxfmt, "paths": paths}.items() if value != None}
    js_oxfmt_fix(name = fix_name(name), tags = kwargs.get("tags", []), target_compatible_with = [], visibility = kwargs.get("visibility"), **shared)
    _js_oxfmt_test(name = name, **(_DEFAULTS | kwargs | shared))

# Preparing the package tree dominates a check test's run time: seconds alone,
# but staging it came close to the moderate limit when many ran at once.
_DEFAULTS = {"timeout": "long"}

_ATTRS = {
    "oxfmt": attr.label(
        doc = "The Oxfmt command.",
        mandatory = True,
        providers = [JsBinaryInfo],
        executable = True,
        cfg = "target",
    ),
    "paths": attr.string_list(default = ["."], doc = "Operands relative to the package."),
}

def _js_oxfmt_test_impl(ctx):
    return package_executable(ctx, ctx.attr.oxfmt, args = ["--check"] + [checked_path(path, allow_root = True) for path in ctx.attr.paths], env = ctx.attr.env, test = True)

_js_oxfmt_test = rule(
    implementation = _js_oxfmt_test_impl,
    doc = "Runs `oxfmt --check` on the paths in the staged package.",
    test = True,
    toolchains = EXECUTABLE_TOOLCHAINS,
    attrs = PACKAGE_EXECUTABLE_ATTRS | _ATTRS,
)
