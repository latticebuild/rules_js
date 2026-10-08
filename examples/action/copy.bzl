"""A custom adapter built entirely on the public JS action API."""

load("//js:providers.bzl", "JsBinaryInfo", "JsPackageInfo")
load("//js/support:action.bzl", "ACTION_ATTRS", "NODE_TOOLCHAIN_TYPE", "action_inputs", "run_in_tree")

def _copy_impl(ctx):
    output = ctx.actions.declare_file("result.txt")
    run_in_tree(ctx, action_inputs(ctx, ctx.attr.tool[JsBinaryInfo], files = ctx.files.srcs, deps = ctx.attr.deps, aliases = ctx.attr.aliases), arguments = [output.basename], outputs = [output], mnemonic = "ExampleCopy", progress_message = "Running scratch example %{label}", env = {"MODE": "release"})
    return [DefaultInfo(files = depset([output]))]

scratch_copy = rule(implementation = _copy_impl, toolchains = [NODE_TOOLCHAIN_TYPE], attrs = ACTION_ATTRS | {
    "tool": attr.label(mandatory = True, executable = True, cfg = "exec", providers = [JsBinaryInfo]),
    "srcs": attr.label_list(allow_files = True),
    "deps": attr.label_list(providers = [JsPackageInfo]),
    "aliases": attr.string_keyed_label_dict(providers = [JsPackageInfo]),
})
