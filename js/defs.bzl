"""JavaScript package, execution, compiler, framework and check rules."""

load("//js/private:binary.bzl", _js_binary = "js_binary")
load("//js/private:package.bzl", _js_package = "js_package")
load("//js/private:test.bzl", _js_test = "js_test")
load("//js/private:tree.bzl", _js_tree = "js_tree")
load("//js/private/knip:knip_test.bzl", _js_knip_test = "js_knip_test")
load("//js/private/ox:oxfmt_test.bzl", _js_oxfmt_test = "js_oxfmt_test")
load("//js/private/ox:oxlint_test.bzl", _js_oxlint_test = "js_oxlint_test")
load("//js/private/prettier:prettier_test.bzl", _js_prettier_test = "js_prettier_test")
load("//js/private/storybook:storybook.bzl", _js_storybook = "js_storybook")
load("//js/private/svelte:svelte_kit.bzl", _js_svelte_kit = "js_svelte_kit")
load("//js/private/svelte:svelte_kit_write.bzl", _js_svelte_kit_write = "js_svelte_kit_write")
load("//js/private/ts:tsc.bzl", _js_tsc = "js_tsc")
load("//js/private/ts:tsconfig.bzl", _js_tsconfig = "js_tsconfig")
load("//js/private/vite:vite.bzl", _js_vite = "js_vite")
load("//js/private/vite:vite_config.bzl", _js_vite_config = "js_vite_config")
load("//js/private/vite:vitest.bzl", _js_vitest_test = "js_vitest_test")

visibility("public")

js_package = _js_package
js_binary = _js_binary
js_tree = _js_tree
js_test = _js_test

js_tsconfig = _js_tsconfig
js_tsc = _js_tsc
js_vite_config = _js_vite_config
js_vite = _js_vite
js_vitest = _js_vitest_test
js_svelte_kit = _js_svelte_kit
js_svelte_kit_write = _js_svelte_kit_write
js_storybook = _js_storybook
js_oxlint_test = _js_oxlint_test
js_oxfmt_test = _js_oxfmt_test
js_prettier_test = _js_prettier_test
js_knip_test = _js_knip_test
