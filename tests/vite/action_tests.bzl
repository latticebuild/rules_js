"""Analysis contracts that real builds cannot observe: refusals and check wiring."""

load("@bazel_skylib//lib:unittest.bzl", "analysistest", "asserts")

visibility("private")

_FIXTURES = "//testdata/vite/actions:"

def action_test_suite(name):
    """Check refusals, and that kept checks still run for their consumers.

    Args:
      name: Test suite name.
    """
    tests = []
    for target, (mnemonic, validation) in {
        "tool_data": ("ViteBuild", "typescript.tsconfig_check"),
        "typescript": ("Tsc", "tool_tsc.tsconfig_check"),
    }.items():
        test = name + "_" + target + "_test"
        _hermetic_action_test(name = test, target_under_test = _FIXTURES + target, mnemonic = mnemonic, validation = validation)
        tests.append(test)
    for target in ["conflicting_configurations", "shared_tool_bundle"]:
        shared = name + "_" + target + "_test"
        _shared_configurations_test(name = shared, target_under_test = _FIXTURES + target)
        tests.append(shared)
    for case, message in {
        "aliases": "resolves to both",
        "bindings": "conflicting target and execution package bindings",
        "identity": "conflicting package identities",
        "inventory": "conflicting target and execution package inventories",
        "tool_data": "dist/index.js is both",
    }.items():
        test = name + "_conflicting_" + case + "_test"
        _conflict_test(name = test, target_under_test = _FIXTURES + "conflicting_" + case, message = message)
        tests.append(test)
    native.test_suite(name = name, tests = tests)

def _hermetic_action_test_impl(ctx):
    env = analysistest.begin(ctx)

    # Exec tool validations reach consumers only through this output group;
    # nothing downstream fails when they stop running.
    validations = analysistest.target_under_test(env)[OutputGroupInfo]._validation.to_list()
    asserts.true(env, any([file.basename == ctx.attr.validation and "-exec-" in file.path for file in validations]), ctx.attr.validation + " must reach its consumer")
    runs = [action for action in analysistest.target_actions(env) if action.mnemonic == ctx.attr.mnemonic]
    asserts.equals(env, 1, len(runs))
    for run in runs:
        asserts.false(env, "NODE_OPTIONS" in run.env, "host NODE_OPTIONS must not reach Node actions")
    return analysistest.end(env)

_hermetic_action_test = analysistest.make(
    _hermetic_action_test_impl,
    config_settings = {
        "//command_line_option:action_env": ["NODE_OPTIONS=--undeclared-option"],
        "//command_line_option:extra_execution_platforms": [_FIXTURES + "execution_platform"],
        "//command_line_option:host_action_env": ["NODE_OPTIONS=--undeclared-option"],
    },
    attrs = {
        "mnemonic": attr.string(),
        "validation": attr.string(),
    },
)

def _shared_configurations_test_impl(ctx):
    env = analysistest.begin(ctx)
    actions = analysistest.target_actions(env)
    comparisons = [action for action in actions if action.mnemonic == "VitePackageCheck"]
    bundles = [action for action in actions if action.mnemonic == "ViteBuild"]
    asserts.equals(env, 1, len(comparisons))
    asserts.equals(env, 1, len(bundles))
    if comparisons and bundles:
        stamp = comparisons[0].outputs.to_list()[0]
        asserts.true(env, stamp in bundles[0].inputs.to_list(), "equivalence must succeed before Vite runs")
        files = [file for file in comparisons[0].inputs.to_list() if file.basename == "value.js"]
        asserts.equals(env, 2, len(files), "both configured outputs must be declared checker inputs")
        asserts.true(env, "compare-packages" in comparisons[0].argv[0], "the dedicated executable compares the copies: {}".format(comparisons[0].argv))
    return analysistest.end(env)

_shared_configurations_test = analysistest.make(_shared_configurations_test_impl)

def _conflict_test_impl(ctx):
    env = analysistest.begin(ctx)
    asserts.expect_failure(env, ctx.attr.message)
    return analysistest.end(env)

_conflict_test = analysistest.make(_conflict_test_impl, expect_failure = True, attrs = {"message": attr.string()})
