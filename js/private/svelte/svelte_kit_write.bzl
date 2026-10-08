"""Publish SvelteKit's declared outputs in the invoking workspace."""

load("@rules_bound//bound:defs.bzl", "BOUND_TOOLCHAIN_TYPE", "BoundInfo", "bound_context", "bound_layout", "bundle_path")
load("//js:providers.bzl", "JsTsconfigInfo")
load("//js/support:fix.bzl", "host_transition", "workspace_directory")
load("//js/support:layout.bzl", "package_relative_path")

visibility("//...")

def _js_svelte_kit_write_impl(ctx):
    if ctx.label.repo_name:
        fail("%s: SvelteKit write targets must belong to the invoking workspace" % ctx.label)
    writer = ctx.executable._writer
    program = "writer/" + writer.basename
    info = BoundInfo(
        program = program,
        layout = [
            bound_layout.file(program, writer),
            # Tree artifacts contribute their leaves, so keep route-less
            # packages' empty types directory in the executable too.
            bound_layout.directory("kit/.svelte-kit/types"),
        ] + [bound_layout.file("kit/" + package_relative_path(ctx, file), file) for file in ctx.files.src],
        args = [workspace_directory(ctx.label.package), bundle_path("kit")],
    )
    result = bound_context(ctx).bind(info, bundle = "private")
    return [DefaultInfo(executable = result.executable)]

js_svelte_kit_write = rule(
    implementation = _js_svelte_kit_write_impl,
    doc = "Writes generated SvelteKit configuration and types to the owning workspace package.",
    executable = True,
    cfg = host_transition,
    toolchains = [BOUND_TOOLCHAIN_TYPE],
    attrs = {
        "src": attr.label(mandatory = True, providers = [JsTsconfigInfo]),
        "_writer": attr.label(
            default = Label("//js/private/svelte/tools/write-svelte-kit"),
            executable = True,
            cfg = "target",
        ),
    },
)
