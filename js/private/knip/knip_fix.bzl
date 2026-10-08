"""Apply knip changes inside the invoking workspace package."""

load("//js:providers.bzl", "JsBinaryInfo")
load("//js/support:execution.bzl", "EXECUTABLE_TOOLCHAINS", "executable_inputs", "node_executable")
load("//js/support:fix.bzl", "host_transition")

visibility("//...")

_FLAGS = ["--no-progress"]

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

def _js_knip_fix_impl(ctx):
    tool = ctx.attr.knip[JsBinaryInfo]
    return node_executable(
        ctx,
        [tool.bin],
        executable_inputs(ctx, [], tool = tool),
        args = _FLAGS + ["--workspace", ctx.attr.workspace, "--fix"],
        env = ctx.attr.env,
        workspace_package = ctx.label.package,
        adapter = ctx.attr._fix,
    )

js_knip_fix = rule(
    implementation = _js_knip_fix_impl,
    doc = "Applies Knip's default-mode fixes in the invoking workspace.",
    executable = True,
    cfg = host_transition,
    toolchains = EXECUTABLE_TOOLCHAINS,
    attrs = _ATTRS | {
        "env": attr.string_dict(doc = "Runtime environment overrides."),
        "_fix": attr.label(default = Label("//js/support:run_fix"), executable = True, cfg = "target"),
    },
)
