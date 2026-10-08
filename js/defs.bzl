"""Supported rules for this toolkit."""

load("//js/private:binary.bzl", _js_binary = "js_binary")
load("//js/private:package.bzl", _js_package = "js_package")
load("//js/private:test.bzl", _js_test = "js_test")
load("//js/private:tree.bzl", _js_tree = "js_tree")

visibility("public")

js_package = _js_package
js_binary = _js_binary
js_tree = _js_tree
js_test = _js_test
