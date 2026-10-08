"""js_tree rule: the part of a monorepo checkout that running a JavaScript program needs."""

load("//:providers.bzl", "JsPackageInfo")
load(":action.bzl", "NODE_TOOLCHAIN_TYPE")
load(":layout.bzl", "package_records", "relative_links", "tree_layout", "write_input_list")

visibility("//...")

# Declared under app/ so image_layer(srcs = {"/": ...}) lands the tree at /app.
_APP_DIR = "app/"

def _js_tree_impl(ctx):
    files = [file for dep in ctx.attr.deps if JsPackageInfo not in dep for file in dep[DefaultInfo].files.to_list()]
    layout = tree_layout(ctx.label, package_records(ctx.attr.deps), files = files, runtime_only = ctx.attr.runtime_only)

    outputs = []
    for path, file in layout.files.items():
        if file.is_directory:
            out = ctx.actions.declare_directory(_APP_DIR + path)
        else:
            out = ctx.actions.declare_file(_APP_DIR + path)
        if file.is_directory and ctx.attr.runtime_only:
            declared_inputs = write_input_list(ctx, ctx.label.name + ".inputs/" + path + ".jsonl", [file])
            ctx.actions.run(
                executable = ctx.executable._filter_runtime,
                arguments = [file.path, out.path, declared_inputs.path],
                inputs = [file, declared_inputs],
                outputs = [out],
                mnemonic = "JsRuntimeFilter",
                progress_message = "Filtering runtime files for %{label}",
            )
        else:
            ctx.actions.symlink(output = out, target_file = file)
        outputs.append(out)
    for path, target in relative_links(layout.links):
        link = ctx.actions.declare_symlink(_APP_DIR + path)
        ctx.actions.symlink(output = link, target_path = target)
        outputs.append(link)
    return [DefaultInfo(files = depset(outputs))]

js_tree = rule(
    implementation = _js_tree_impl,
    doc = """The part of a monorepo checkout that running a JavaScript program needs.

    A local install and build would leave the same layout: each js_package in
    deps and their closure lands at its repository path (workspace packages,
    with only the files Node loads by default) or its node_modules/ path (installed npm
    packages: those npm installs on the target platform, which the generated
    edges select, excluding the types-only @types/* by default); workspace packages' links
    become relative node_modules links; each other target's files land at their
    package paths. Express a program's own package as a js_package whose deps
    give its workspace links. The tree is declared under app/ in this package,
    for image_layer(srcs = {"/": ":tree"}), so a package holds one js_tree and
    no other target named app. An image for a musl distribution builds for a
    platform that names //platforms:musl. Set runtime_only = False
    when the program typechecks or otherwise reads declarations or source maps.
    """,
    attrs = {
        "deps": attr.label_list(
            doc = "The js_package targets the program needs, and plain file targets to place at their package paths.",
            allow_files = True,
        ),
        "runtime_only": attr.bool(
            doc = "Omit declarations, source maps, build info, and @types packages; disable for programs that typecheck.",
            default = True,
        ),
        "_filter_runtime": attr.label(
            executable = True,
            cfg = "exec",
            default = Label("//private/tools/filter-runtime"),
        ),
    },
    # Filtering is an action of this rule, so its cfg=exec Go runner cannot
    # constrain the parent's platform. Resolve the JS runtime platform here.
    toolchains = [NODE_TOOLCHAIN_TYPE],
)
