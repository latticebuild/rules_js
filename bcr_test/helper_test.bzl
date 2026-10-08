"""A caller-owned test built through the public runtime support facade."""

load("@subject//js/support:execution.bzl", "EXECUTABLE_TOOLCHAINS", "TEST_RUNTIME_ATTRS", "executable_inputs", "node_executable")

def _helper_test_impl(ctx):
    inputs = executable_inputs(ctx, ctx.attr.data, files = [ctx.file.src])
    return node_executable(ctx, [ctx.file.src], inputs, test = True)

helper_test = rule(
    implementation = _helper_test_impl,
    test = True,
    toolchains = EXECUTABLE_TOOLCHAINS,
    attrs = TEST_RUNTIME_ATTRS | {
        "data": attr.label_list(),
        "src": attr.label(allow_single_file = [".mjs"], mandatory = True),
    },
)
