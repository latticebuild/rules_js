"""Apply prettier changes inside the invoking workspace package."""

load("//js:providers.bzl", "JsBinaryInfo")
load("//js/support:execution.bzl", "EXECUTABLE_TOOLCHAINS", "executable_inputs", "node_executable")
load("//js/support:fix.bzl", "host_transition")
load("//js/support:layout.bzl", "checked_path")

visibility("//...")

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

def _js_prettier_fix_impl(ctx):
    tool = ctx.attr.prettier[JsBinaryInfo]
    return node_executable(
        ctx,
        [tool.bin],
        executable_inputs(ctx, [], tool = tool),
        args = ["--write"] + [checked_path(path, allow_root = True) for path in ctx.attr.paths],
        env = ctx.attr.env,
        workspace_package = ctx.label.package,
        adapter = ctx.attr._fix,
    )

js_prettier_fix = rule(
    implementation = _js_prettier_fix_impl,
    doc = "Formats the paths in the invoking workspace with Prettier.",
    executable = True,
    cfg = host_transition,
    toolchains = EXECUTABLE_TOOLCHAINS,
    attrs = _ATTRS | {
        "env": attr.string_dict(doc = "Runtime environment overrides."),
        "_fix": attr.label(default = Label("//js/support:run_fix"), executable = True, cfg = "target"),
    },
)
