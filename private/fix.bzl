"""Host configuration and workspace paths shared by source-fixing rules."""

visibility("//...")

def workspace_directory(package):
    """A package's directory in the invoking workspace, as the fix entry script takes it.

    Args:
      package: a repository-relative package directory; `""` or `"."` for
        the root package.

    Returns:
      `package`, or `.` for the root package, since an empty argument is
      not portable.
    """
    return "." if package in ("", ".") else package

def _host_transition_impl(_settings, _attr):
    return {"//command_line_option:platforms": str(Label("@platforms//host"))}

# `bazel run` executes on the invoking machine, whose platform need not be the
# configured target platform; native tool bindings must match the machine.
host_transition = transition(
    implementation = _host_transition_impl,
    inputs = [],
    outputs = ["//command_line_option:platforms"],
)

def fix_name(name):
    """The fix executable declared beside a check test.

    Args:
      name: The check test's name, which must end in `_test`.

    Returns:
      `name` with its `_test` suffix replaced by `_fix`.
    """
    if not name.endswith("_test"):
        fail("%s: a check test's name ends in _test, so that its fix twin can be named" % name)
    return name.removesuffix("_test") + "_fix"
