"""Apply oxlint changes inside the invoking workspace package."""

load("//js:providers.bzl", "JsBinaryInfo")
load("//js/support:execution.bzl", "EXECUTABLE_TOOLCHAINS", "executable_inputs", "node_executable")
load("//js/support:fix.bzl", "host_transition")
load("//js/support:layout.bzl", "checked_path")

visibility("//...")

_FLAGS = ["--deny-warnings", "--no-error-on-unmatched-pattern"]

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

def _js_oxlint_fix_impl(ctx):
    tool = ctx.attr.oxlint[JsBinaryInfo]
    return node_executable(
        ctx,
        [tool.bin],
        executable_inputs(ctx, [], tool = tool),
        args = _FLAGS + ["--fix"] + [checked_path(path, allow_root = True) for path in ctx.attr.paths],
        env = ctx.attr.env,
        workspace_package = ctx.label.package,
        adapter = ctx.attr._fix,
    )

js_oxlint_fix = rule(
    implementation = _js_oxlint_fix_impl,
    doc = "Applies Oxlint's fixes to the paths in the invoking workspace.",
    executable = True,
    cfg = host_transition,
    toolchains = EXECUTABLE_TOOLCHAINS,
    attrs = _ATTRS | {
        "env": attr.string_dict(doc = "Runtime environment overrides."),
        "_fix": attr.label(default = Label("//js/support:run_fix"), executable = True, cfg = "target"),
    },
)
