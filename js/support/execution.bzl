"""Supported execution primitives for JavaScript adapters."""

load("//js/private:package_execution.bzl", _PACKAGE_EXECUTABLE_ATTRS = "PACKAGE_EXECUTABLE_ATTRS", _executable_inputs = "executable_inputs", _package_executable = "package_executable")
load("//js/private:runtime.bzl", _EXECUTABLE_TOOLCHAINS = "EXECUTABLE_TOOLCHAINS", _TEST_RUNTIME_ATTRS = "TEST_RUNTIME_ATTRS", _node_executable = "node_executable", _tree_directory = "tree_directory")

visibility("public")

EXECUTABLE_TOOLCHAINS = _EXECUTABLE_TOOLCHAINS
PACKAGE_EXECUTABLE_ATTRS = _PACKAGE_EXECUTABLE_ATTRS
TEST_RUNTIME_ATTRS = _TEST_RUNTIME_ATTRS
executable_inputs = _executable_inputs
node_executable = _node_executable
package_executable = _package_executable
tree_directory = _tree_directory
