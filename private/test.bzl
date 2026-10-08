"""js_test rule: node:test over declared scripts, in a bound executable."""

load(":coverage.bzl", "COVERAGE_ATTRS", "instrumented_files")
load(":package_execution.bzl", "executable_inputs")
load(":runtime.bzl", "EXECUTABLE_TOOLCHAINS", "node_executable")

visibility("//...")

def _js_test_impl(ctx):
    scripts = ctx.files.srcs
    inputs = executable_inputs(ctx, ctx.attr.data, files = scripts)
    return node_executable(ctx, scripts, inputs, coverage = True, adapter = ctx.attr._node_test, test = True) + [
        instrumented_files(ctx, ["srcs", "data"]),
    ]

js_test = rule(
    implementation = _js_test_impl,
    doc = """Runs srcs with node:test, writing Bazel's JUnit and LCOV reports.

    The executable lays out the declared files with package import links.
    Relative imports, npm dependencies, and workspace packages resolve in that
    tree.
    """,
    test = True,
    toolchains = EXECUTABLE_TOOLCHAINS,
    attrs = {
        "data": attr.label_list(
            doc = "What the tests read or import: the files under test, fixtures, and js_package targets.",
            allow_files = True,
        ),
        "srcs": attr.label_list(
            doc = "The test files.",
            allow_files = [".js", ".mjs", ".cjs"],
            allow_empty = False,
            mandatory = True,
        ),
        "_node_test": attr.label(default = Label("//private/tools/run-node-tests"), executable = True, cfg = "target"),
    } | COVERAGE_ATTRS,
)
