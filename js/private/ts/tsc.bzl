"""Compile one declared TypeScript project to its standard outputs."""

load("@bazel_skylib//lib:paths.bzl", "paths")
load("//js:providers.bzl", "JsBinaryInfo", "JsPackageInfo", "JsTsconfigInfo")
load("//js/support:action.bzl", "ACTION_ATTRS", "NODE_TOOLCHAIN_TYPE", "action_inputs", "run_in_tree")
load("//js/support:layout.bzl", "checked_path", "package_relative_path", "relpath", "repository_path", "validation_outputs")
load(":tsconfig.bzl", "PATH_OPTIONS")

visibility("//...")

_OUTPUT_EXTENSIONS = {
    ".cjs": (".cjs", ".d.cts"),
    ".cts": (".cjs", ".d.cts"),
    ".js": (".js", ".d.ts"),
    ".json": (".json", None),
    ".jsx": (".js", ".d.ts"),
    ".mjs": (".mjs", ".d.mts"),
    ".mts": (".mjs", ".d.mts"),
    ".ts": (".js", ".d.ts"),
    ".tsx": (".js", ".d.ts"),
}

def _js_tsc_impl(ctx):
    project = ctx.attr.config[JsTsconfigInfo]
    config = package_relative_path(ctx, project.config)
    options = {name: value for name, value in project.compiler_options.items() if value != None}
    for name in PATH_OPTIONS:
        if name in options:
            options[name] = checked_path(paths.join(paths.dirname(config), options[name]), allow_root = name in ["rootDir", "outDir", "declarationDir"])
    sources = [file for file in project.sources.to_list() if not file.is_directory and _compiler_source(repository_path(file), options)]
    source_paths = [relpath(ctx.label.package, repository_path(file)) for file in sources]

    # A generated directory among the project's files, own or inherited, such as
    # SvelteKit's route types, holds compiler inputs no target declares one by
    # one.
    directories = [relpath(ctx.label.package, repository_path(file)) for file in ctx.attr.config[DefaultInfo].files.to_list() if file.is_directory]
    inferred = tsc_project_outputs(source_paths, options, config)
    if inferred == None:
        fail("%s: cannot infer standard outputs from the declared sources and compiler_options" % ctx.label)
    if len(inferred) != len({output: True for output in inferred}):
        fail("%s: compiler output collision" % ctx.label)
    validation = ctx.label.name + ".tsconfig_check"
    if validation in inferred:
        fail("%s: compiler output collides with private validation output %s" % (ctx.label, validation))
    outputs = [ctx.actions.declare_file(checked_path(output)) for output in inferred]
    stamp = ctx.actions.declare_file(validation)
    compiler = ctx.attr.tsc[JsBinaryInfo]
    run_in_tree(
        ctx,
        action_inputs(ctx, compiler, deps = [ctx.attr.config], tool_deps = [ctx.attr.typescript]),
        executable = ctx.attr._checker,
        arguments = [
            config,
            json.encode(project.compiler_options),
            json.encode(source_paths),
            json.encode(directories),
            json.encode([package_relative_path(ctx, output) for output in outputs]),
            relpath(ctx.label.package, ctx.attr.typescript[JsPackageInfo].package),
            package_relative_path(ctx, stamp),
        ],
        outputs = outputs + [stamp],
        mnemonic = "Tsc",
        progress_message = "Compiling TypeScript %{label}",
    )
    return [
        DefaultInfo(files = depset(outputs)),
        OutputGroupInfo(_validation = depset([stamp], transitive = [validation_outputs([ctx.attr.config, ctx.attr.tsc, ctx.attr.typescript])])),
    ]

js_tsc = rule(
    implementation = _js_tsc_impl,
    doc = "Compiles or checks a js_tsconfig project during builds, publishing only standard compiler emissions.",
    toolchains = [NODE_TOOLCHAIN_TYPE],
    attrs = ACTION_ATTRS | {
        "config": attr.label(providers = [JsTsconfigInfo], mandatory = True),
        "tsc": attr.label(mandatory = True, providers = [JsBinaryInfo], executable = True, cfg = "exec"),
        "typescript": attr.label(mandatory = True, providers = [JsPackageInfo], cfg = "exec"),
        "_checker": attr.label(default = Label("//js/private/ts/tools/compile-typescript"), executable = True, cfg = "exec"),
    },
)

def _compiler_source(path, options):
    if path.endswith((".ts", ".tsx", ".mts", ".cts")):
        return True
    if path.endswith(".json"):
        return options.get("resolveJsonModule", True)
    return options.get("allowJs", options.get("checkJs", False)) and path.endswith((".js", ".jsx", ".mjs", ".cjs"))

def tsc_project_outputs(sources, options, config):
    """Derives project emission; the selected compiler validates this inventory.

    Args:
      sources: Package-relative compiler input paths.
      options: Effective compilation options, with paths relative to the package.
      config: Package-relative configuration path.

    Returns:
      Output paths, or None when a source is outside rootDir.
    """
    outputs = []
    out_file = options.get("outFile")
    emitting = [source for source in sources if not source.endswith((".d.ts", ".d.mts", ".d.cts"))]
    if options.get("noEmit") or not emitting:
        pass
    elif out_file:
        if not options.get("emitDeclarationOnly"):
            outputs.append(out_file)
            if options.get("sourceMap") and not options.get("inlineSourceMap"):
                outputs.append(out_file + ".map")
        if options.get("declaration", options.get("composite", False)):
            declaration = paths.replace_extension(out_file, ".d.ts")
            outputs.append(declaration)
            if options.get("declarationMap"):
                outputs.append(declaration + ".map")
    else:
        # Current TypeScript uses the configuration directory when rootDir is
        # absent. A different selected compiler must pass inventory validation.
        per_source = dict({"rootDir": paths.dirname(config) or "."}, **options)
        for source in sources:
            emitted = tsc_output_paths(source, per_source)
            if emitted == None:
                return None
            outputs.extend(emitted)
    if options.get("incremental", options.get("composite", False)):
        metadata = options.get("tsBuildInfoFile")
        if not metadata:
            stem = paths.replace_extension(out_file or config, "")
            if not out_file and options.get("outDir"):
                suffix = relpath(options["rootDir"], stem) if options.get("rootDir") else paths.basename(stem)
                stem = paths.normalize(paths.join(options["outDir"], suffix))
            metadata = stem + ".tsbuildinfo"
        outputs.append(metadata)
    return outputs

def tsc_output_paths(rel, options):
    """The files tsc writes for a source, relative to the package.

    They follow rootDir, outDir, declaration, declarationMap, and sourceMap. A
    declaration source (.d.ts, .d.mts, .d.cts) is read and writes nothing.

    Args:
      rel: The source's package-relative path.
      options: dict of the project's compiler options.

    Returns:
      list of paths, or None when rel is not a source under rootDir.
    """
    if options.get("noEmit"):
        return []
    for declaration in [".d.ts", ".d.mts", ".d.cts"]:
        if rel.endswith(declaration):
            return []
    root = checked_path(options.get("rootDir", "."), allow_root = True)
    out = options.get("outDir")
    prefix = "" if root == "." else root + "/"
    if not rel.startswith(prefix):
        return None
    for source, (javascript, declaration) in _OUTPUT_EXTENSIONS.items():
        if rel.endswith(source):
            if source in [".tsx", ".jsx"] and options.get("jsx") == "preserve":
                javascript = ".jsx"
            stem = (out + "/" + rel[len(prefix):] if out else rel)[:-len(source)]
            outputs = [] if options.get("emitDeclarationOnly") or (source == ".json" and stem + javascript == rel) else [stem + javascript]
            if outputs and source != ".json" and options.get("sourceMap") and not options.get("inlineSourceMap"):
                outputs.append(stem + javascript + ".map")
            if declaration and options.get("declaration", options.get("composite", False)):
                declaration_out = options.get("declarationDir") or out
                declaration_stem = (declaration_out + "/" + rel[len(prefix):] if declaration_out else rel)[:-len(source)]
                outputs.append(declaration_stem + declaration)
                if options.get("declarationMap"):
                    outputs.append(declaration_stem + declaration + ".map")
            return outputs
    return None
