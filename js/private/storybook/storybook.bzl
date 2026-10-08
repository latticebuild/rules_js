"""Storybook development servers."""

load("//js:providers.bzl", "JsBinaryInfo")
load("//js/support:execution.bzl", "EXECUTABLE_TOOLCHAINS", "PACKAGE_EXECUTABLE_ATTRS", "package_executable")
load("//js/support:layout.bzl", "checked_path")

visibility("//...")

def js_storybook(name, package = "package.json", **kwargs):
    """Starts a loopback Storybook development server with declared package inputs.

    Args:
      name: Executable target name.
      package: Package manifest in the calling BUILD package.
      **kwargs: srcs, deps, aliases, config_dir, port, storybook and runtime attributes.
    """
    _js_storybook(name = name, package = package, **kwargs)

def _js_storybook_impl(ctx):
    if ctx.attr.port < 1 or ctx.attr.port > 65535:
        fail("port must be between 1 and 65535")
    return package_executable(
        ctx,
        ctx.attr.storybook,
        files = [ctx.file.package],
        args = ["dev", "--config-dir", checked_path(ctx.attr.config_dir), "--host", "127.0.0.1", "--port", str(ctx.attr.port), "--ci", "--no-open", "--exact-port"],
        # `storybook dev` needs a package manager, and the bundle carries none
        # (PATH is Node's directory, then the caller's, which may have none):
        # this names npm, whose support in `storybook dev` reads package
        # manifests and runs no npm command.
        env = dict({"STORYBOOK_DISABLE_TELEMETRY": "1", "npm_config_user_agent": "npm"}, **ctx.attr.env),
    )

_js_storybook = rule(
    implementation = _js_storybook_impl,
    doc = "Runs storybook dev on loopback. `ibazel run` restarts it on each change.",
    executable = True,
    toolchains = EXECUTABLE_TOOLCHAINS,
    attrs = PACKAGE_EXECUTABLE_ATTRS | {
        "config_dir": attr.string(default = ".storybook", doc = "Package-relative Storybook configuration directory."),
        "package": attr.label(allow_single_file = True, mandatory = True),
        "port": attr.int(default = 6006, doc = "Loopback development server port."),
        "storybook": attr.label(
            doc = "Target-platform Storybook command.",
            cfg = "target",
            mandatory = True,
            executable = True,
            providers = [JsBinaryInfo],
        ),
    },
)
