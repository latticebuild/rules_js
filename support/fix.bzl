"""Supported fix primitives for JavaScript adapters."""

load("//private:fix.bzl", _fix_name = "fix_name", _host_transition = "host_transition", _workspace_directory = "workspace_directory")

visibility("public")

fix_name = _fix_name
host_transition = _host_transition
workspace_directory = _workspace_directory
