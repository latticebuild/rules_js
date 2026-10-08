"""js_binary rule: a package.json bin script and the executable that runs it."""

load("//:providers.bzl", "JsBinaryInfo")
load(":layout.bzl", "validation_outputs")
load(":package_execution.bzl", "executable_inputs")
load(":runtime.bzl", "EXECUTABLE_TOOLCHAINS", "node_executable")

visibility("//...")

def _js_binary_impl(ctx):
    inputs = executable_inputs(ctx, ctx.attr.data, files = [ctx.file.bin])
    return [
        JsBinaryInfo(
            bin = ctx.file.bin,
            files = inputs.runfiles.files,
            packages = depset(inputs.packages.values()),
            links = tuple(sorted(inputs.links.items())),
        ),
        OutputGroupInfo(_validation = validation_outputs(ctx.attr.data + [ctx.attr.bin])),
    ] + node_executable(ctx, [ctx.file.bin], inputs)

js_binary = rule(
    implementation = _js_binary_impl,
    doc = """A package.json bin script.

    bazel run, a test, or an action that names it as a tool runs its
    executable: one file holding node, the script and its packages, which runs
    anywhere. The build rules here read JsBinaryInfo instead and run node on
    the script in the tree they lay out, so the tool and the package's own
    plugins resolve one node_modules/.
    """,
    executable = True,
    toolchains = EXECUTABLE_TOOLCHAINS,
    provides = [JsBinaryInfo],
    attrs = {
        "bin": attr.label(
            doc = "The script, a file of this package.",
            allow_single_file = True,
            mandatory = True,
        ),
        "data": attr.label_list(
            doc = "What the script needs at run time: its js_package, which brings its dependencies.",
            allow_files = True,
        ),
    },
)
