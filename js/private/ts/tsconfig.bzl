"""Declared TypeScript projects, without compiler actions."""

load("@bazel_skylib//lib:paths.bzl", "paths")
load("//js:providers.bzl", "JsPackageInfo", "JsTsconfigInfo")
load("//js/support:layout.bzl", "config_providers", "relpath", "repository_path")

visibility("//...")

# Options whose values are paths, relative to the configuration declaring them.
PATH_OPTIONS = ["rootDir", "outDir", "declarationDir", "outFile", "tsBuildInfoFile"]

def js_tsconfig(name, config, srcs = [], compiler_options = {}, package = None, **kwargs):
    """Declares a project's inputs, the configurations it extends and the compiler options it sets.

    Args:
      name: Configuration target name.
      config: Original TypeScript JSON configuration.
      srcs: Complete project source inventory.
      compiler_options: What config contributes to the output-affecting options
        over the configurations it extends: the values it changes, paths
        relative to it, None resetting an inherited one.
      package: Optional manifest, defaulting to a local package.json when present.
      **kwargs: Extended configurations, declared deps, data, aliases and standard Bazel attributes.
    """
    manifests = native.glob(["package.json"], allow_empty = True) if package == None else []
    _js_tsconfig(name = name, config = config, srcs = srcs, compiler_options = json.encode(compiler_options), package = package or (manifests[0] if manifests else None), **kwargs)

def _js_tsconfig_impl(ctx):
    # A configuration inherits what the ones it extends pass on, never their
    # sources: a plain file passes on itself.
    inherited = depset(transitive = [
        base[JsTsconfigInfo].inheritance if JsTsconfigInfo in base else base[DefaultInfo].files
        for base in ctx.attr.extends
    ])
    own = [ctx.file.config] + ctx.files.data
    files = depset(own + ctx.files.srcs + ([ctx.file.package] if ctx.file.package else []), transitive = [inherited])
    config = config_providers(ctx, files, ctx.attr.srcs + ctx.attr.data, bases = ctx.attr.extends)
    return [
        JsTsconfigInfo(
            config = ctx.file.config,
            compiler_options = _effective_options(ctx.file.config, json.decode(ctx.attr.compiler_options), ctx.attr.extends),
            sources = depset(ctx.files.srcs),
            packages = config.packages,
            links = config.links,
            inheritance = depset(own, transitive = [inherited]),
        ),
    ] + config.providers

_js_tsconfig = rule(
    implementation = _js_tsconfig_impl,
    doc = "Declares TypeScript project files and package bindings without checking or compiling them.",
    provides = [JsTsconfigInfo],
    attrs = {
        "aliases": attr.string_keyed_label_dict(providers = [JsPackageInfo]),
        "compiler_options": attr.string(default = "{}"),
        "config": attr.label(allow_single_file = [".json"], mandatory = True),
        "data": attr.label_list(doc = "Declared file inputs other than extended configurations.", allow_files = True),
        "deps": attr.label_list(doc = "Imported packages, including installed configuration packages.", providers = [JsPackageInfo]),
        "extends": attr.label_list(
            doc = "The configurations config extends: the rules providing them, whose inheritance this project stages, or their files.",
            allow_files = [".json"],
        ),
        "package": attr.label(allow_single_file = [".json"]),
        "srcs": attr.label_list(allow_files = True),
    },
)

def _effective_options(config, own, bases):
    """A configuration's effective output-affecting options.

    Args:
      config: The configuration File.
      own: The options it sets, None resetting an inherited one.
      bases: The targets of its `extends`.

    Returns:
      The options its bases' providers pass on, in `extends` order and with
      their paths moved to config's directory, overlaid by its own.
    """
    directory = paths.dirname(repository_path(config))
    options = {}
    for base in bases:
        if JsTsconfigInfo not in base:
            continue
        info = base[JsTsconfigInfo]
        base_directory = paths.dirname(repository_path(info.config))
        for name, value in info.compiler_options.items():
            if name in PATH_OPTIONS and base_directory != directory:
                value = relpath(directory, paths.normalize(paths.join(base_directory, value)))
            options[name] = value
    for name, value in own.items():
        if value == None:
            options.pop(name, None)
        else:
            options[name] = value
    return options
