"""Analysis tests for what js_svelte_kit refuses to load or stage."""

load("@bazel_skylib//lib:unittest.bzl", "analysistest", "asserts")

visibility("private")

_FIXTURES = "//testdata/svelte/svelte_kit"

_CASES = {
    "other_config": "config must be the package's Vite configuration",
    "staged_jsconfig": "sync reads it only to warn about its options",
    "staged_svelte_config": "SvelteKit configuration belongs in the declared Vite configuration",
    "staged_tsconfig": "sync reads it only to warn about its options",
    "staged_vite_config": "srcs must not stage another candidate",
}

def svelte_kit_test_suite(name):
    """Check that js_svelte_kit refuses configurations and sources sync would misread.

    Args:
        name: Test suite name.
    """
    tests = []
    for case, message in _CASES.items():
        test = "%s_%s" % (name, case)
        _refused_test(
            name = test,
            target_under_test = "%s:%s" % (_FIXTURES, case),
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
