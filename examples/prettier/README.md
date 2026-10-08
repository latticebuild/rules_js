# Runnable examples

Prepare the repository with `mise run bootstrap`, then run:

```sh
bazel build //examples/prettier:artifacts
bazel test //examples/prettier:test
```

| Feature | Source | Command | Expected result |
| --- | --- | --- | --- |
| Passing checks with declared paths and inputs | [checks/BUILD.bazel](checks/BUILD.bazel) | `bazel test //examples/prettier:test` | Checks clean JS/CSS and a real declared Svelte plugin. |
| Generated fix twins | [checks/BUILD.bazel](checks/BUILD.bazel) | `bazel run //examples/prettier/checks:format_fix` | Formats these checkout paths explicitly; automated fix tests use disposable invoking workspaces. |
| Failures, successful fixes and workspace guards | [../tests/prettier/BUILD.bazel](../../tests/prettier/BUILD.bazel) | `bazel test //tests/prettier:checks_test //tests/prettier:fix_guard_test` | Checks rejection and real writes without editing checked-in fixtures. |

The root artifact/test gates include these examples. Deliberately invalid subjects
remain in test fixtures; their owner tests require the expected refusals. Fix and
editor-write commands modify the invoking checkout only when run explicitly.
Automated mutation cases use disposable invoking workspaces.
