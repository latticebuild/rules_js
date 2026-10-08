"""js_package rule: an npm package's files, where it lives, and what it depends on."""

load("//js:providers.bzl", "JsPackageInfo")
load(":layout.bzl", "checked_path", "dependency_links", "import_name", "is_installed", "validation_outputs")

visibility("//...")

# Keep package.json relative to the calling BUILD, not this .bzl file.
def js_package(name, package_name, package = "package.json", **kwargs):
    _js_package(
        name = name,
        package_name = package_name,
        package = package,
        **kwargs
    )

def _js_package_impl(ctx):
    import_name(ctx.attr.package_name)
    installs = {}
    for path in ctx.attr.installs:
        if checked_path(path) != path or not is_installed(path):
            fail("%s: installation path must be a normalized node_modules package entry: %r" % (ctx.label, path))
        import_name(("/" + path).rpartition("/node_modules/")[2])
        if path != ctx.label.package:
            installs[path] = True
    files = depset([ctx.file.package] + ctx.files.srcs)
    dependencies = ctx.attr.deps + ctx.attr.aliases.values()

    # depset elements must be immutable, hence the tuples.
    record = struct(
        name = ctx.attr.package_name,
        repository = ctx.label.workspace_name,
        package = ctx.label.package,
        files = files,
        links = tuple(sorted(dependency_links(ctx.label, ctx.label.package, ctx.attr.deps, ctx.attr.aliases).items())),
        installs = tuple(sorted(installs)),
    )

    # Sandboxed execution only sees declared files; keep sources in runfiles.
    runfiles = ctx.runfiles(transitive_files = files).merge_all(
        [dep[DefaultInfo].default_runfiles for dep in dependencies],
    )
    return [
        JsPackageInfo(
            name = record.name,
            package = record.package,
            closure = depset([record], transitive = [dep[JsPackageInfo].closure for dep in dependencies]),
        ),
        DefaultInfo(files = files, runfiles = runfiles),
        OutputGroupInfo(_validation = validation_outputs(ctx.attr.srcs + dependencies)),
    ]

_js_package = rule(
    implementation = _js_package_impl,
    doc = """A workspace or installed npm package, preserving its repository layout.

    Which of the two it is follows from where the target lives: a package under
    a node_modules/ directory is installed (the pnpm repository rule generates those
    targets), every other one is a workspace package.
    """,
    provides = [JsPackageInfo],
    attrs = {
        "aliases": attr.string_keyed_label_dict(
            doc = "Additional import names mapped to canonical package targets; targets contribute their closures.",
            providers = [JsPackageInfo],
        ),
        "deps": attr.label_list(
            doc = "The packages this one needs to run: package.json's dependencies, and the peers it cannot run without.",
            providers = [JsPackageInfo],
        ),
        "installs": attr.string_list(doc = "Repository-relative package directory symlinks observed by the pnpm repository rule; empty for ordinary installations."),
        "package": attr.label(
            doc = "The package's package.json.",
            allow_single_file = True,
            mandatory = True,
        ),
        "package_name": attr.string(
            doc = "The node_modules key Node resolves the package by, such as @latticebuild/base.",
            mandatory = True,
        ),
        "srcs": attr.label_list(
            doc = "The package's files besides package.json, all in this Bazel package.",
            allow_files = True,
            allow_empty = True,
        ),
    },
)
