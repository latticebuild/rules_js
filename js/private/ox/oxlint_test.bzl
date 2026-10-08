"""js_oxlint_test: Oxlint's check, with a twin that applies its fixes in place."""

load("//js:providers.bzl", "JsBinaryInfo")
load("//js/support:execution.bzl", "EXECUTABLE_TOOLCHAINS", "PACKAGE_EXECUTABLE_ATTRS", "TEST_RUNTIME_ATTRS", "package_executable")
load("//js/support:fix.bzl", "fix_name")
load("//js/support:layout.bzl", "checked_path")
load(":oxlint_fix.bzl", "js_oxlint_fix")

visibility("//...")

def js_oxlint_test(name, paths = None, oxlint = None, env = None, **kwargs):
    """Lints with Oxlint, failing on warnings, and declares `<name minus _test>_fix`.

    `bazel run` of the twin applies Oxlint's fixes to the same paths in the
    invoking workspace.

    Args:
      name: Test name, ending in `_test`.
      paths: Operands relative to the package; defaults to the whole package.
      oxlint: The Oxlint command.
      env: Runtime environment overrides, shared with the twin.
      **kwargs: Staged inputs and standard test attributes; the twin shares
        the test's visibility.
    """
    shared = {key: value for key, value in {"env": env, "oxlint": oxlint, "paths": paths}.items() if value != None}
    js_oxlint_fix(name = fix_name(name), tags = kwargs.get("tags", []), target_compatible_with = [], visibility = kwargs.get("visibility"), **shared)
    _js_oxlint_test(name = name, **(_DEFAULTS | kwargs | shared))

_FLAGS = ["--deny-warnings", "--no-error-on-unmatched-pattern"]

# Preparing the package tree dominates a check test's run time: seconds alone,
# but staging it came close to the moderate limit when many ran at once.
_DEFAULTS = {"timeout": "long"}

_ATTRS = {
    "oxlint": attr.label(
        doc = "The Oxlint command.",
        mandatory = True,
        providers = [JsBinaryInfo],
        executable = True,
        cfg = "target",
    ),
    "paths": attr.string_list(default = ["."], doc = "Operands relative to the package."),
}

def _js_oxlint_test_impl(ctx):
    return package_executable(ctx, ctx.attr.oxlint, args = _FLAGS + [checked_path(path, allow_root = True) for path in ctx.attr.paths], env = ctx.attr.env, test = True)

_js_oxlint_test = rule(
    implementation = _js_oxlint_test_impl,
    doc = "Runs Oxlint on the paths in the staged package, failing on warnings.",
    test = True,
    toolchains = EXECUTABLE_TOOLCHAINS,
    attrs = PACKAGE_EXECUTABLE_ATTRS | _ATTRS | TEST_RUNTIME_ATTRS,
)
