"""Apply oxfmt changes inside the invoking workspace package."""

load("//js:providers.bzl", "JsBinaryInfo")
load("//js/support:execution.bzl", "EXECUTABLE_TOOLCHAINS", "executable_inputs", "node_executable")
load("//js/support:fix.bzl", "host_transition")
load("//js/support:layout.bzl", "checked_path")

visibility("//...")

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

def _js_oxfmt_fix_impl(ctx):
    tool = ctx.attr.oxfmt[JsBinaryInfo]
    return node_executable(
        ctx,
        [tool.bin],
        executable_inputs(ctx, [], tool = tool),
        args = [checked_path(path, allow_root = True) for path in ctx.attr.paths],
        env = ctx.attr.env,
        workspace_package = ctx.label.package,
        adapter = ctx.attr._fix,
    )

js_oxfmt_fix = rule(
    implementation = _js_oxfmt_fix_impl,
    doc = "Formats the paths in the invoking workspace with Oxfmt.",
    executable = True,
    cfg = host_transition,
    toolchains = EXECUTABLE_TOOLCHAINS,
    attrs = _ATTRS | {
        "env": attr.string_dict(doc = "Runtime environment overrides."),
        "_fix": attr.label(default = Label("//js/support:run_fix"), executable = True, cfg = "target"),
    },
)
