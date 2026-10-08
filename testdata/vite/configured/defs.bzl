"""Generated inputs that distinguish target and execution configurations."""

load("//js:providers.bzl", "JsPackageInfo")

visibility("//testdata/vite/configured/...")

def _configured_file_impl(ctx):
    output = ctx.actions.declare_file("value.js")
    ctx.actions.write(output, "export const configuration = " + repr(ctx.bin_dir.path) + ";\n")
    return [DefaultInfo(files = depset([output]))]

configured_file = rule(implementation = _configured_file_impl)

def _mismatched_package_impl(ctx):
    execution = "-exec-" in ctx.bin_dir.path
    package = ctx.label.package + "/" + ctx.label.name
    output = ctx.actions.declare_file(ctx.label.name + "/value.js")
    ctx.actions.write(output, "export {};\n")
    files = [output]
    if ctx.attr.case == "inventory" and execution:
        extra = ctx.actions.declare_file(ctx.label.name + "/extra.js")
        ctx.actions.write(extra, "export {};\n")
        files.append(extra)
    record = struct(
        name = "different" if ctx.attr.case == "identity" and execution else "configured-dep",
        repository = ctx.label.workspace_name,
        package = package,
        files = depset(files),
        links = ((package + "/node_modules/other", package + ("/exec" if execution else "/target")),) if ctx.attr.case == "bindings" else (),
        installs = (),
    )
    return [
        DefaultInfo(files = record.files),
        JsPackageInfo(name = record.name, package = package, closure = depset([record])),
    ]

mismatched_package = rule(implementation = _mismatched_package_impl, attrs = {"case": attr.string(mandatory = True)})
