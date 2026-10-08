"""Execution contracts for the real bundled test families and ordinary binaries."""

load("@bazel_skylib//lib:unittest.bzl", "analysistest", "asserts")

visibility("private")

def runtime_test_suite(name):
    """Verify test-only platform requirements across every shared runtime caller.

    Args:
      name: Test suite name.
    """
    tests = []
    for family, target in {
        "knip": "//examples/knip/library:unused_test",
        "node": "//examples/basics:alias_test",
        "oxfmt": "//examples/ox/checks:format_test",
        "oxlint": "//examples/ox/checks:lint_test",
        "prettier": "//examples/prettier/checks:format_test",
        "vitest": "//examples/vite/app:unit_test",
    }.items():
        test = name + "_" + family + "_test"
        _execution_test(name = test, target_under_test = target)
        tests.append(test)
    binary = name + "_binary_test"
    _execution_test(name = binary, target_under_test = "//examples/basics:hello", is_test = False)
    tests.append(binary)
    native.test_suite(name = name, tests = tests)

def _execution_test_impl(ctx):
    env = analysistest.begin(ctx)
    target = analysistest.target_under_test(env)
    linux = ctx.target_platform_has_constraint(ctx.attr._linux[platform_common.ConstraintValueInfo])
    required = ctx.attr.is_test and linux
    asserts.equals(env, required, testing.ExecutionInfo in target, "only Linux bundled tests declare execution requirements")
    if testing.ExecutionInfo in target:
        info = target[testing.ExecutionInfo]
        asserts.equals(env, {"no-sandbox": "1"}, info.requirements)
        asserts.equals(env, "test", info.exec_group)
    return analysistest.end(env)

_execution_test = analysistest.make(
    _execution_test_impl,
    attrs = {
        "is_test": attr.bool(default = True),
        "_linux": attr.label(default = Label("@platforms//os:linux")),
    },
)
