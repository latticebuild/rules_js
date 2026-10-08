"""Thin benchmark adapter invoking the production scratch action runner."""

load("@subject//js:providers.bzl", "JsBinaryInfo")
load("@subject//js/support:action.bzl", "ACTION_ATTRS", "NODE_TOOLCHAIN_TYPE", "action_inputs", "run_in_tree")

def _impl(ctx):
    output = ctx.actions.declare_file(ctx.attr.out)
    inputs = action_inputs(ctx, ctx.attr.tool[JsBinaryInfo], files = [ctx.file.srcs])
    run_in_tree(ctx, inputs, arguments = ["--input", ctx.file.srcs.basename, "--output", ctx.attr.out] + ctx.attr.extra_args, outputs = [output], mnemonic = "BenchmarkNode", progress_message = "Benchmark %{label}")
    return [DefaultInfo(files = depset([output]))]

benchmark_action = rule(implementation = _impl, toolchains = [NODE_TOOLCHAIN_TYPE], attrs = ACTION_ATTRS | {"tool": attr.label(providers = [JsBinaryInfo], executable = True, cfg = "exec", mandatory = True), "srcs": attr.label(allow_single_file = True), "out": attr.string(mandatory = True), "extra_args": attr.string_list()})
