"""Supported action primitives for JavaScript adapters."""

load("//js/private:action.bzl", _ACTION_ATTRS = "ACTION_ATTRS", _NODE_TOOLCHAIN_TYPE = "NODE_TOOLCHAIN_TYPE", _action_inputs = "action_inputs", _run_in_tree = "run_in_tree")

visibility("public")

ACTION_ATTRS = _ACTION_ATTRS
NODE_TOOLCHAIN_TYPE = _NODE_TOOLCHAIN_TYPE
action_inputs = _action_inputs
run_in_tree = _run_in_tree
