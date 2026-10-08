"""Repository selection refuses unsupported declarations and exclusion patterns."""

load("@bazel_skylib//lib:partial.bzl", "partial")
load("@bazel_skylib//lib:unittest.bzl", "analysistest", "asserts", "unittest")
load("//private/pnpm:selection.bzl", "compile_patterns", "ignored_directories")

def selection_test_suite(name):
    """Create the repository-selection syntax and exclusion-pattern refusals.

    Args:
        name: Test suite name and prefix for expected-failure subjects.
    """
    tests = []
    for index, source in enumerate([
        'repo(features=[ignore_directories(["nested"])])',
        'f = ignore_directories; f(["aliased"])',
        'ignore_directories(["first"]); ignore_directories(["second"])',
        "ignore_directories(PATTERNS)",
    ]):
        target = name + "_invalid_" + str(index)
        _selection_input(name = target, source = source, tags = ["manual"])
        _invalid_selection_test(name = target + "_test", target_under_test = ":" + target)
        tests.append(":" + target + "_test")
    pattern_failures = []
    for case, pattern, message in [
        ("open_class", "[", "Empty character class"),
        ("empty_class", "[]", "Empty character class"),
        ("trailing_escape", "a\\", "Trailing escape"),
        ("open_brace", "{a", "Unmatched brace"),
        ("close_brace", "a}", "Unmatched brace"),
        ("absolute", "/outside", "Invalid Gazelle exclusion pattern"),
        ("empty", "", "Invalid Gazelle exclusion pattern"),
        ("length", "a" * 4097, "Invalid Gazelle exclusion pattern"),
        ("expansion", "{a,b}" * 13, "exceeds 4096 brace expansions"),
    ]:
        target = name + "_pattern_" + case
        _invalid_pattern(name = target, pattern = pattern, tags = ["manual"])
        pattern_failures.append(partial.make(_pattern_failure_test, target_under_test = ":" + target, message = message))
    unittest.suite(name + "_patterns", *pattern_failures)
    native.test_suite(name = name, tests = tests + [":" + name + "_patterns"])

def _selection_input_impl(ctx):
    ignored_directories(ctx.attr.source)
    return []

_selection_input = rule(implementation = _selection_input_impl, attrs = {"source": attr.string()})

def _invalid_selection_test_impl(ctx):
    env = analysistest.begin(ctx)
    asserts.expect_failure(env, "REPO.bazel ignore_directories requires")
    return analysistest.end(env)

_invalid_selection_test = analysistest.make(_invalid_selection_test_impl, expect_failure = True)

def _invalid_pattern_impl(ctx):
    compile_patterns(ctx.attr.pattern)
    return []

_invalid_pattern = rule(implementation = _invalid_pattern_impl, attrs = {"pattern": attr.string(mandatory = True)})

def _pattern_failure_test_impl(ctx):
    env = analysistest.begin(ctx)
    asserts.expect_failure(env, ctx.attr.message)
    return analysistest.end(env)

_pattern_failure_test = analysistest.make(_pattern_failure_test_impl, expect_failure = True, attrs = {"message": attr.string(mandatory = True)})
