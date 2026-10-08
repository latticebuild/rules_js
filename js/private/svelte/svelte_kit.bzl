"""SvelteKit's generated TypeScript configuration and types, from `svelte-kit sync`."""

load("//js:providers.bzl", "JsBinaryInfo", "JsPackageInfo", "JsTsconfigInfo")
load("//js/support:action.bzl", "ACTION_ATTRS", "NODE_TOOLCHAIN_TYPE", "action_inputs", "run_in_tree")
load("//js/support:layout.bzl", "dependency_links", "package_records", "package_relative_path", "repository_path", "validation_outputs")
load(":svelte_kit_write.bzl", "js_svelte_kit_write")

visibility("//...")

# Package-relative outputs; route declarations remain a tree following routes.
_FILES = ["node_modules/$app/tsconfig.json", "node_modules/$app/tsconfig/service-worker.json", "node_modules/$app/types/index.d.ts", "node_modules/$app/types/env.d.ts"]

# The configurations sync loads from the package directory: Vite's, whose
# SvelteKit plugin can supply the configuration, then SvelteKit's own.
_VITE_CONFIGS = ["vite.config.js", "vite.config.mjs", "vite.config.ts", "vite.config.cjs", "vite.config.mts", "vite.config.cts"]
_SVELTE_CONFIGS = ["svelte.config.js", "svelte.config.ts"]

def js_svelte_kit(name, config = "vite.config.ts", package = "package.json", compiler_options = {}, **kwargs):
    """Generates SvelteKit's package-local configuration and route declarations.

    A js_tsconfig whose configuration extends `$app/tsconfig` names
    this target in `extends` and inherits everything sync writes: the
    configuration, its declarations and the route types.
    `bazel run <name>_write` refreshes those outputs in the checkout for
    editors and Gazelle; building either target never edits the checkout.

    Args:
      name: Target name, `kit` by convention.
      config: The package's Vite configuration containing the SvelteKit plugin.
      package: The package manifest, which makes the configuration an ES module.
      compiler_options: The generated configuration's output-affecting options.
      **kwargs: srcs, deps, tool_deps, kit, typescript and standard Bazel attributes.
    """
    _js_svelte_kit(name = name, config = config, package = package, compiler_options = json.encode(compiler_options), **kwargs)
    js_svelte_kit_write(
        name = name + "_write",
        src = ":" + name,
        tags = kwargs.get("tags", []),
        target_compatible_with = [],
        testonly = kwargs.get("testonly"),
        visibility = kwargs.get("visibility"),
    )

def _js_svelte_kit_impl(ctx):
    config = package_relative_path(ctx, ctx.file.config)
    if config not in _VITE_CONFIGS:
        fail("%s: config must be the package's Vite configuration, not %s" % (ctx.label, config))
    for file in ctx.files.srcs:
        refusal = _refusal(ctx.label.package, repository_path(file))
        if refusal:
            fail("%s: srcs must not stage %s: %s" % (ctx.label, file.short_path, refusal))
    files = [ctx.actions.declare_file(name) for name in _FILES]
    outputs = files + [ctx.actions.declare_directory(".svelte-kit/types")]
    run_in_tree(
        ctx,
        action_inputs(
            ctx,
            ctx.attr.kit[JsBinaryInfo],
            files = ctx.files.srcs + [ctx.file.config, ctx.file.package],
            tool_deps = ctx.attr.tool_deps + [ctx.attr.typescript],
        ),
        arguments = ["sync", "--config", config],
        outputs = outputs,
        mnemonic = "SvelteKitSync",
        progress_message = "Generating SvelteKit types for %{label}",
    )
    generated = depset(outputs)
    packages = package_records(ctx.attr.deps)
    return [
        DefaultInfo(
            files = generated,
            runfiles = ctx.runfiles(transitive_files = depset(transitive = [generated] + [record.files for record in packages.values()])),
        ),
        JsTsconfigInfo(
            config = files[0],
            compiler_options = {name: value for name, value in json.decode(ctx.attr.compiler_options).items() if value != None},
            sources = depset(),
            packages = depset(packages.values()),
            links = tuple(sorted(dependency_links(ctx.label, ctx.label.package, ctx.attr.deps, {}).items())),
            inheritance = generated,
        ),
        OutputGroupInfo(_validation = validation_outputs(ctx.attr.srcs + ctx.attr.deps + ctx.attr.tool_deps + [ctx.attr.kit, ctx.attr.typescript])),
    ]

_js_svelte_kit = rule(
    implementation = _js_svelte_kit_impl,
    doc = "Runs `svelte-kit sync` and provides `$app/tsconfig`, passing on generated declarations and route types to extending configurations.",
    provides = [JsTsconfigInfo],
    toolchains = [NODE_TOOLCHAIN_TYPE],
    attrs = ACTION_ATTRS | {
        "compiler_options": attr.string(doc = "The generated configuration's output-affecting options, as JSON.", default = "{}"),
        "config": attr.label(doc = "The Vite configuration containing the SvelteKit plugin.", allow_single_file = [".js", ".mjs", ".ts", ".cjs", ".mts", ".cts"], mandatory = True),
        "deps": attr.label_list(
            doc = "The packages the generated declarations import, @sveltejs/kit and svelte, which extending configurations inherit.",
            providers = [JsPackageInfo],
        ),
        "kit": attr.label(mandatory = True, providers = [JsBinaryInfo], executable = True, cfg = "exec"),
        "package": attr.label(doc = "The package manifest.", allow_single_file = [".json"], mandatory = True),
        "srcs": attr.label_list(
            doc = "What sync reads besides the configuration: its local imports, environment declarations, parameter matchers, routes and static assets. Secrets in .env files must not be staged.",
            allow_files = True,
        ),
        "tool_deps": attr.label_list(
            doc = "The packages the configuration imports, such as its adapter and @sveltejs/vite-plugin-svelte.",
            providers = [JsPackageInfo],
            cfg = "exec",
        ),
        # An optional peer of SvelteKit, which the package graph leaves out.
        "typescript": attr.label(
            doc = "The TypeScript package sync imports; without it, sync writes no route types.",
            mandatory = True,
            providers = [JsPackageInfo],
            cfg = "exec",
        ),
    },
)

def _refusal(package, path):
    # Sync reads these from the package directory in place of, or besides, what
    # the rule declares.
    directory, _, name = path.rpartition("/")
    if directory != package:
        return None
    if name in _VITE_CONFIGS:
        return "config declares the Vite configuration; srcs must not stage another candidate"
    if name in _SVELTE_CONFIGS:
        return "SvelteKit configuration belongs in the declared Vite configuration"
    if name in ["tsconfig.json", "jsconfig.json"]:
        return "sync reads it only to warn about its options, so it would only make the outputs depend on it"
    return None
