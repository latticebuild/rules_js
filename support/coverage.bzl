"""Supported coverage primitives for JavaScript adapters."""

load("//private:coverage.bzl", _COVERAGE_ATTRS = "COVERAGE_ATTRS", _instrumented_files = "instrumented_files")

visibility("public")

COVERAGE_ATTRS = _COVERAGE_ATTRS
instrumented_files = _instrumented_files
