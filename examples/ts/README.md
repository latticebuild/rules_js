# Runnable examples

Prepare the repository with `mise run bootstrap`, then run:

```sh
bazel build //examples/ts:artifacts
bazel test //examples/ts:test
```

| Feature | Source | Command | Expected result |
| --- | --- | --- | --- |
| Config inheritance and standard emissions | [library/BUILD.bazel](library/BUILD.bazel) | `bazel test //examples/ts/library:emissions_test` | Compiles source, executes the JS and checks emitted declarations. |
| noEmit and package aliases | [check/BUILD.bazel](check/BUILD.bazel) | `bazel build //examples/ts/check:typecheck` | Checks an imported first-party declaration without publishing JS. |
| Emission options and refusal boundaries | [../tests/ts/BUILD.bazel](../../tests/ts/BUILD.bazel) | `bazel test //:test` | Retained cases cover output options, collisions and inheritance guards. |

The root artifact/test gates include these examples. Deliberately invalid subjects
remain in test fixtures; their owner tests require the expected refusals. Fix and
editor-write commands modify the invoking checkout only when run explicitly.
Automated mutation cases use disposable invoking workspaces.
