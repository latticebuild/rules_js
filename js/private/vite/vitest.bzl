"""Vitest tests, evaluated only when they run."""

load("//js:providers.bzl", "JsBinaryInfo", "JsPackageInfo", "JsViteConfigInfo")
load("//js/support:coverage.bzl", "COVERAGE_ATTRS", "instrumented_files")
load("//js/support:execution.bzl", "EXECUTABLE_TOOLCHAINS", "PACKAGE_EXECUTABLE_ATTRS", "TEST_RUNTIME_ATTRS", "package_executable")
load("//js/support:layout.bzl", "package_relative_path")
load(":browser.bzl", "browser_inputs")

visibility("//...")

def _js_vitest_impl(ctx):
    config = ctx.attr.config[JsViteConfigInfo]
    browser = browser_inputs(ctx)
    return package_executable(
        ctx,
        ctx.attr.vitest,
        files = browser.files,
        configurations = [ctx.attr.config],
        packages = [ctx.attr.coverage_provider],
        args = ["run", "--config", package_relative_path(ctx, config.config), "--mode", ctx.attr.mode],
        env = dict({"CI": "1", "NODE_ENV": "test", "STORYBOOK_DISABLE_TELEMETRY": "1"}, **ctx.attr.env),
        env_inherit = ctx.attr.env_inherit,
        adapter_args = browser.adapter_args,
        native_runfiles = browser.runfiles,
        coverage = True,
        adapter = ctx.attr._vitest_adapter,
        test = True,
    ) + [
        instrumented_files(ctx, ["srcs", "deps", "data"]),
    ]

# Bazel requires a test rule's name to end in _test; defs.bzl exports it as js_vitest.
js_vitest_test = rule(
    implementation = _js_vitest_impl,
    doc = "Creates a Vitest test with Bazel arguments, filtering, sharding and reporting.",
    test = True,
    toolchains = EXECUTABLE_TOOLCHAINS,
    attrs = PACKAGE_EXECUTABLE_ATTRS | COVERAGE_ATTRS | TEST_RUNTIME_ATTRS | {
        "browser": attr.label(doc = "One checksum-pinned Playwright browser installation, whose native payload stays at its canonical runfiles location.", allow_files = True),
        "config": attr.label(doc = "The js_vite_config target Vitest evaluates.", providers = [JsViteConfigInfo], mandatory = True),
        "coverage_provider": attr.label(
            doc = "The coverage provider package Vitest loads under `bazel coverage`, staged with Vitest itself.",
            mandatory = True,
            providers = [JsPackageInfo],
        ),
        "env_inherit": attr.string_list(doc = "Caller environment names the test inherits: they keep the caller's value, and the test gets no private default for them, such as its home."),
        "mode": attr.string(default = "test"),
        "vitest": attr.label(mandatory = True, providers = [JsBinaryInfo], executable = True, cfg = "target"),
        "_vitest_adapter": attr.label(default = Label("//js/private/vite/tools/run-vitest"), executable = True, cfg = "target"),
    },
)
