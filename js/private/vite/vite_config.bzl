"""Reusable Vite-family configuration inputs, without evaluation."""

load("//js:providers.bzl", "JsPackageInfo", "JsViteConfigInfo")
load("//js/support:layout.bzl", "config_providers")

visibility("//...")

def js_vite_config(name, config, package = "package.json", **kwargs):
    """Declares a Vite or Vitest configuration and its reusable inputs.

    Args:
      name: Configuration target name.
      config: Original Vite or Vitest configuration file.
      package: Manifest controlling configuration module evaluation.
      **kwargs: Declared deps, data, aliases and standard Bazel attributes.
    """
    _js_vite_config(name = name, config = config, package = package, **kwargs)

def _js_vite_config_impl(ctx):
    own = [ctx.file.config, ctx.file.package] + ctx.files.data
    config = config_providers(ctx, depset(own, transitive = [target[DefaultInfo].files for target in ctx.attr.deps if JsPackageInfo not in target]), ctx.attr.data)
    return [
        JsViteConfigInfo(
            config = ctx.file.config,
            packages = config.packages,
            links = config.links,
        ),
    ] + config.providers

_js_vite_config = rule(
    implementation = _js_vite_config_impl,
    doc = "Declares configuration inputs without evaluating code, bundling or starting a tool.",
    provides = [JsViteConfigInfo],
    attrs = {
        "aliases": attr.string_keyed_label_dict(providers = [JsPackageInfo]),
        "config": attr.label(allow_single_file = True, mandatory = True),
        "data": attr.label_list(doc = "Authored configuration file inputs, such as policy files and manifests.", allow_files = True),
        "deps": attr.label_list(doc = "Imported files, packages and reusable configurations.", allow_files = True),
        "package": attr.label(allow_single_file = [".json"], mandatory = True),
    },
)
