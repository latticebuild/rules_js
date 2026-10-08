"""Analysis test that tsconfig inheritance passes on configurations, never sources."""

load("@bazel_skylib//lib:unittest.bzl", "analysistest", "asserts")
load("//js:providers.bzl", "JsTsconfigInfo")

visibility("private")

def _names(files):
    return sorted([file.basename for file in files.to_list()])

def _tsconfig_inheritance_test_impl(ctx):
    env = analysistest.begin(ctx)
    target = analysistest.target_under_test(env)
    info = target[JsTsconfigInfo]
    inherited = ["base.data.json", "base.json", "middle.json", "tsconfig.json"]
    asserts.equals(env, inherited, _names(info.inheritance))
    asserts.equals(env, ["leaf.ts"], _names(info.sources))
    asserts.equals(env, sorted(inherited + ["leaf.ts"]), _names(target[DefaultInfo].files))
    staged = _names(target[DefaultInfo].default_runfiles.files)
    for source in ["base.ts", "middle.ts"]:
        asserts.false(env, source in staged, "a base's source %s reached the runfiles" % source)
    return analysistest.end(env)

tsconfig_inheritance_test = analysistest.make(_tsconfig_inheritance_test_impl)
