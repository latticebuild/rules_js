"""Analysis refusals for package executables and mapped runtime inputs, and
the bound arguments that set up an executable's process."""

load("@bazel_skylib//lib:unittest.bzl", "analysistest", "asserts", "unittest")
load("//js:defs.bzl", "js_vite_config", "js_vitest")
load("//js/support:execution.bzl", "tree_directory")
load("//js/support:fix.bzl", "workspace_directory")

visibility("private")

_RUNNERS = "//testdata/vite/runners:"
_OXLINT = "//testdata/vite/checks/oxlint:"

# The map with which a tool that writes LCOV has its coverage translated.
_COVERAGE_MAP = "@bundle:coverage_map.json"

# Native adapters are distinct programs in the bound layout.
_FIX = "@bundle:adapter/run-fix"
_NODE = "@bundle:adapter/run-node-tests"
_VITEST = "@bundle:adapter/run-vitest"

def runner_test_suite(name):
    """Checks package executables' runtime paths and bound arguments.

    Executables reject ambiguous or escaping runtime paths, and their bind
    actions set up the process: working directory, PATH, home, temporary
    directory, and the adapter's options: a fix twin's workspace package, the
    coverage map and the test runner.

    Args:
      name: Test suite name.
    """
    tests = []
    for target, message in [("overlap_test", "is both"), ("escape_test", "leaves or replaces")]:
        test = name + "_" + target
        _failure_test(name = test, target_under_test = _RUNNERS + target, message = message)
        tests.append(test)

    # Real browser suites cannot reveal a silently ignored conflicting binding
    # or a payload accidentally copied into their private application tree.
    browser = name + "_browser_inputs"
    installation = ".playwright/chromium-1247/"
    payload = [installation + "INSTALLATION_COMPLETE", installation + "native/browser"]
    _browser_fixture(name = browser, paths = payload, tags = ["manual"])
    config_source = name + "_browser_config_source"
    _browser_fixture(name = config_source, paths = ["vitest.config.mjs"], tags = ["manual"])
    base_config = name + "_browser_config"
    js_vite_config(name = base_config, config = ":" + config_source, target_compatible_with = [], tags = ["manual"])
    config = name + "_browser_config_overlap"
    js_vite_config(
        name = config,
        config = ":" + config_source,
        data = [":" + browser],
        target_compatible_with = [],
        tags = ["manual"],
    )
    for suffix, attrs, message in [
        ("env", {"env": {"PLAYWRIGHT_BROWSERS_PATH": "elsewhere"}}, "browser owns PLAYWRIGHT_BROWSERS_PATH"),
        ("env_paths", {"env_paths": {"playwright_browsers_path": "elsewhere"}}, "browser owns PLAYWRIGHT_BROWSERS_PATH"),
        ("env_inherit", {"env_inherit": ["Playwright_Browsers_Path"]}, "browser owns PLAYWRIGHT_BROWSERS_PATH"),
        ("data", {"data": [":" + browser]}, "also appears in application data or configuration"),
        ("config", {"config": ":" + config}, "also appears in application data or configuration"),
    ]:
        target = name + "_browser_" + suffix + "_test"
        settings = dict(vitest = "@pnpm//node_modules/vitest:bin", coverage_provider = "@pnpm//node_modules/@vitest/coverage-v8", config = ":" + base_config, browser = ":" + browser, tags = ["manual"], target_compatible_with = [])
        settings.update(attrs)
        js_vitest(name = target, **settings)
        test = target + "_refusal_test"
        _failure_test(name = test, target_under_test = ":" + target, message = message)
        tests.append(test)
    for suffix, paths, message in [
        ("missing_marker", [installation + "native/browser"], "exactly one INSTALLATION_COMPLETE marker"),
        ("ambiguous_marker", payload + [".playwright/chromium-1248/INSTALLATION_COMPLETE"], "exactly one INSTALLATION_COMPLETE marker"),
        ("invalid_marker", ["other/INSTALLATION_COMPLETE", "other/native/browser"], "marker must belong to one .playwright"),
        ("outside_payload", payload + ["outside/browser"], "outside its installation"),
        ("empty_payload", payload[:1], "declares no native payload"),
        ("validation_payload", payload + [installation + "DEPENDENCIES_VALIDATED"], "browser validation metadata must be private"),
    ]:
        fixture = name + "_browser_" + suffix + "_inputs"
        _browser_fixture(name = fixture, paths = paths, tags = ["manual"])
        target = name + "_browser_" + suffix + "_test"
        js_vitest(name = target, vitest = "@pnpm//node_modules/vitest:bin", coverage_provider = "@pnpm//node_modules/@vitest/coverage-v8", config = ":" + base_config, browser = ":" + fixture, tags = ["manual"], target_compatible_with = [])
        test = target + "_refusal_test"
        _failure_test(name = test, target_under_test = ":" + target, message = message)
        tests.append(test)

    # The working directory bound starts node in, and whole arguments of the
    # bind action, present or absent. A package executable runs in its package
    # with Node first on PATH and a private home and temporary directory;
    # declared and inherited names replace those defaults. Check tests start
    # their tool directly; fix twins start the adapter with their package, and
    # tests with their runner and, as tools that write LCOV, the coverage map.
    for target, cwd, present, absent in [
        (
            _OXLINT + "violation_test",
            "@bundle:tree/testdata/vite/checks/oxlint",
            ["PATH=@bundle:node", "HOME=@bundle:tree/.home", "TMPDIR=@bundle:tree/.tmp"],
            [_FIX, _NODE, _VITEST, _COVERAGE_MAP],
        ),
        (
            _OXLINT + "violation_fix",
            "inherit",
            [_FIX, "--node", "--workspace", "testdata/vite/checks/oxlint", "PATH=@bundle:node"],
            ["HOME=@bundle:tree/.home", "--test-runner", _COVERAGE_MAP],
        ),
        (
            _RUNNERS + "node_reports_test",
            "inherit",
            [_NODE, "--node", "--coverage", _COVERAGE_MAP, "PATH=@bundle:node", "NODE_V8_COVERAGE"],
            ["--workspace", "HOME=@bundle:tree/.home"],
        ),
        (
            _RUNNERS + "declared_env_test",
            "@bundle:tree/testdata/vite/runners",
            ["PATH=/declared", "HOME=/declared-home", "TMPDIR=@bundle:tree/.tmp", _VITEST, "--node", "--coverage", _COVERAGE_MAP],
            ["PATH=@bundle:node", "HOME=@bundle:tree/.home", "USERPROFILE=@bundle:tree/.home", "--workspace"],
        ),
    ]:
        test = name + "_bind_" + target.split(":")[1]
        _bind_arguments_test(name = test, target_under_test = target, cwd = cwd, present = present, absent = absent)
        tests.append(test)
    directories_test = name + "_package_directories"
    _package_directories_test(name = directories_test)
    tests.append(directories_test)
    native.test_suite(name = name, tests = tests)

def _failure_test_impl(ctx):
    env = analysistest.begin(ctx)
    asserts.expect_failure(env, ctx.attr.message)
    return analysistest.end(env)

_failure_test = analysistest.make(_failure_test_impl, expect_failure = True, attrs = {"message": attr.string()})

def _browser_fixture_impl(ctx):
    files = []
    for path in ctx.attr.paths:
        file = ctx.actions.declare_file(ctx.label.name + "/" + path)
        ctx.actions.write(file, "")
        files.append(file)
    return [DefaultInfo(files = depset(files))]

_browser_fixture = rule(implementation = _browser_fixture_impl, attrs = {"paths": attr.string_list()})

def _bind_arguments_test_impl(ctx):
    env = analysistest.begin(ctx)
    executable = analysistest.target_under_test(env)[DefaultInfo].files_to_run.executable

    # Fix twins follow the host platform; tests follow the target platform.
    suffix = ".exe" if executable.basename.endswith(".exe") else ""
    binds = [action for action in analysistest.target_actions(env) if action.mnemonic == "Bound"]
    asserts.equals(env, 1, len(binds))
    for bind in binds:
        argv = bind.argv
        if "--cwd" in argv:
            asserts.equals(env, ctx.attr.cwd, argv[argv.index("--cwd") + 1])
        else:
            asserts.true(env, False, "--cwd must be an argument of {}".format(argv))
        for argument in ctx.attr.present:
            argument = argument + suffix if argument.startswith("@bundle:adapter/") else argument
            asserts.true(env, argument in argv, "{} must be an argument of {}".format(argument, argv))
        for argument in ctx.attr.absent:
            argument = argument + suffix if argument.startswith("@bundle:adapter/") else argument
            asserts.false(env, argument in argv, "{} must not be an argument of {}".format(argument, argv))
    return analysistest.end(env)

_bind_arguments_test = analysistest.make(
    _bind_arguments_test_impl,
    attrs = {
        "absent": attr.string_list(),
        "cwd": attr.string(),
        "present": attr.string_list(),
    },
)

def _package_directories_test_impl(ctx):
    env = unittest.begin(ctx)

    # The root package, where no package executable or fix twin lives yet,
    # runs in the tree itself and enters the workspace itself.
    asserts.equals(env, "tree", tree_directory(""))
    asserts.equals(env, "tree", tree_directory("."))
    asserts.equals(env, "tree/javascript/app", tree_directory("javascript/app"))
    asserts.equals(env, ".", workspace_directory(""))
    asserts.equals(env, ".", workspace_directory("."))
    asserts.equals(env, "javascript/app", workspace_directory("javascript/app"))
    return unittest.end(env)

_package_directories_test = unittest.make(_package_directories_test_impl)
