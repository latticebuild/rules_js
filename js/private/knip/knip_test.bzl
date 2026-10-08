"""js_knip_test: Knip's unused-code check, with a twin that applies its fixes in place."""

load("//js:providers.bzl", "JsBinaryInfo")
load("//js/support:execution.bzl", "EXECUTABLE_TOOLCHAINS", "PACKAGE_EXECUTABLE_ATTRS", "TEST_RUNTIME_ATTRS", "package_executable")
load("//js/support:fix.bzl", "fix_name")
load(":knip_fix.bzl", "js_knip_fix")

visibility("//...")

def js_knip_test(name, workspace = None, production = False, knip = None, env = None, **kwargs):
    """Runs Knip on one workspace and, unless `production`, declares `<name minus _test>_fix`.

    `bazel run` of the twin applies Knip's default-mode fixes in the invoking
    workspace. A production test has no twin: production-mode fixes would
    remove exports that only tests import.

    Args:
      name: Test name, ending in `_test`.
      workspace: The workspace Knip selects: the package name, or "." when unnamed.
      production: Whether Knip analyzes production code only.
      knip: The Knip command.
      env: Runtime environment overrides, shared with the twin.
      **kwargs: Staged inputs and standard test attributes; the twin shares
        the test's visibility.
    """
    shared = {key: value for key, value in {"env": env, "knip": knip, "workspace": workspace}.items() if value != None}
    if not production:
        js_knip_fix(name = fix_name(name), tags = kwargs.get("tags", []), target_compatible_with = [], visibility = kwargs.get("visibility"), **shared)
    elif not name.endswith("_test"):
        fail("%s: a check test's name ends in _test" % name)
    _js_knip_test(name = name, production = production, **(_DEFAULTS | kwargs | shared))

_FLAGS = ["--no-progress"]

# Preparing the package tree dominates a check test's run time: seconds alone,
# but staging it came close to the moderate limit when many ran at once.
_DEFAULTS = {"timeout": "long"}

_ATTRS = {
    "knip": attr.label(
        doc = "The Knip command.",
        mandatory = True,
        providers = [JsBinaryInfo],
        executable = True,
        cfg = "target",
    ),
    "workspace": attr.string(default = ".", doc = "The workspace Knip selects: the package name, or \".\"."),
}

def _js_knip_test_impl(ctx):
    args = _FLAGS + ["--treat-config-hints-as-errors"] + (["--production"] if ctx.attr.production else []) + ["--workspace", ctx.attr.workspace]
    return package_executable(ctx, ctx.attr.knip, args = args, env = ctx.attr.env, test = True)

_js_knip_test = rule(
    implementation = _js_knip_test_impl,
    doc = "Runs Knip on the staged package, failing on issues and configuration hints.",
    test = True,
    toolchains = EXECUTABLE_TOOLCHAINS,
    attrs = PACKAGE_EXECUTABLE_ATTRS | _ATTRS | TEST_RUNTIME_ATTRS | {
        "production": attr.bool(doc = "Analyze production code only."),
    },
)
