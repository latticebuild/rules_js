"""Analysis tests for the tsconfig a build or a compile takes."""

load("@bazel_skylib//lib:unittest.bzl", "analysistest", "asserts")

visibility("private")

_FIXTURES = "//testdata/ts/tsconfig_guards"

def tsconfig_guard_test_suite(name):
    """Check that compile and bundle rules reject unsupported configurations.

    Args:
        name: Test suite name.
    """

    tests = []
    for case, message in _CASES.items():
        test = "%s_%s" % (name, case)
        _refused_test(
            name = test,
            target_under_test = "%s:%s_build" % (_FIXTURES, case),
            message = message,
        )
        tests.append(":" + test)
    native.test_suite(name = name, tests = tests)

def _refused_test_impl(ctx):
    env = analysistest.begin(ctx)
    asserts.expect_failure(env, ctx.attr.message)
    return analysistest.end(env)

_refused_test = analysistest.make(
    _refused_test_impl,
    expect_failure = True,
    attrs = {"message": attr.string(mandatory = True)},
)

_CASES = {
    "other_package_tsconfig": "is not in package //",
    "validation_collision": "collides with private validation output",
}
