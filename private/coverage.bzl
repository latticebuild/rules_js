"""Coverage source inventories and maps shared by Node and Vitest tests."""

visibility("//...")

# Test rules whose tools write LCOV under `bazel coverage` pass `coverage`
# to their executable and declare these attributes: Bazel sets LCOV_MERGER, and
# so merges the test's tracefiles, only for a rule that has `_lcov_merger`. The
# entry (runtime adapter attributes) translates them.
COVERAGE_ATTRS = {
    "_lcov_merger": attr.label(
        default = configuration_field(fragment = "coverage", name = "output_generator"),
        executable = True,
        cfg = "exec",
    ),
}

# The files such a test's tools instrument; Bazel's merger keeps records only
# for files its coverage manifest lists.
_SOURCE_EXTENSIONS = ["cjs", "cts", "js", "jsx", "mjs", "mts", "svelte", "ts", "tsx"]

def _unreported(basename):
    # Declarations hold no code; Vitest and Node exclude test files by default.
    parts = basename.split(".")
    return (len(parts) > 2 and parts[-2] == "d") or "test" in parts[1:-1] or "spec" in parts[1:-1]

def instrumented_files(ctx, attributes):
    """The coverage provider of a test whose tools instrument its own inputs.

    The test's attributes carry the code under test, so they are its sources
    in Bazel's coverage manifest (under `--instrument_test_targets`), and its
    dependencies' providers join through the same attributes. Bazel's own
    baseline would give every installed package file a zero-coverage record in
    the merged report, so the baseline lists only first-party sources, without
    the declaration and test files the tools never report.

    Args:
      ctx: Test rule context.
      attributes: Names of the attributes that hold the code under test.

    Returns:
      InstrumentedFilesInfo.
    """
    baseline = []
    if ctx.configuration.coverage_enabled and ctx.coverage_instrumented():
        sources = depset([
            file
            for attribute in attributes
            for file in getattr(ctx.files, attribute)
            if not file.owner.repo_name and file.extension in _SOURCE_EXTENSIONS and not _unreported(file.basename)
        ]).to_list()
        record = "SF:{}\nFNF:0\nFNH:0\nLH:0\nLF:0\nend_of_record\n"
        file = ctx.actions.declare_file(ctx.label.name + ".baseline_coverage.dat")
        ctx.actions.write(file, "".join([record.format(source.path) for source in sorted(sources, key = lambda source: source.path)]))
        baseline = [file]
    return coverage_common.instrumented_files_info(
        ctx,
        source_attributes = attributes,
        dependency_attributes = attributes,
        extensions = _SOURCE_EXTENSIONS,
        baseline_coverage_files = baseline,
    )

def coverage_file(ctx, layout, mapped):
    """Write the tree-to-execution-path map consumed by the runtime coverage library.

    Args:
      ctx: Consuming test rule context.
      layout: Validated runtime layout.
      mapped: Artifacts explicitly relocated by the consumer, excluded as sources.

    Returns:
      The declared coverage map file.
    """
    output = ctx.actions.declare_file(ctx.label.name + ".coverage_map.json")
    ctx.actions.write(output, json.encode({
        path: file.path
        for path, file in layout.files.items()
        if not file.owner.repo_name and not file.is_directory and file not in mapped
    }))
    return output
