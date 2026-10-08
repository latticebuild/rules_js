# Runnable examples

Prepare the repository with `mise run bootstrap`, then run:

```sh
bazel build //examples/knip:artifacts
bazel test //examples/knip:test
```

| Feature | Source | Command | Expected result |
| --- | --- | --- | --- |
| Default and production checks | [library/BUILD.bazel](library/BUILD.bazel) | `bazel test //examples/knip:test` | Both modes pass on a real named ESM package; the root selector is ".". |
| Default-mode fix twin | [library/BUILD.bazel](library/BUILD.bazel) | `bazel run //examples/knip/library:unused_fix` | Applies fixes in this checkout; production mode deliberately has no fix target. |
| Configuration hints, unused exports and guarded fixes | [../tests/knip/BUILD.bazel](../../tests/knip/BUILD.bazel) | `bazel test //tests/knip:checks_test //tests/knip:fix_guard_test` | Retained cases prove failures reach the selected Knip mode and guarded workspace handling. |

The root artifact/test gates include these examples. Deliberately invalid subjects
remain in test fixtures; their owner tests require the expected refusals. Fix and
editor-write commands modify the invoking checkout only when run explicitly.
Automated mutation cases use disposable invoking workspaces.
