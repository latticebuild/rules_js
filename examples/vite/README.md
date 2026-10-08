# Runnable examples

Prepare the repository with `mise run bootstrap`, then run:

```sh
bazel build //examples/vite:artifacts
bazel test //examples/vite:test
```

| Feature | Source | Command | Expected result |
| --- | --- | --- | --- |
| Reusable config, custom build output/mode/env | [app/BUILD.bazel](app/BUILD.bazel) | `bazel test //examples/vite/app:bundle_test` | Builds web/index.html and verifies the published assets. |
| Vitest node inputs and coverage provider | [app/answer.test.mjs](app/answer.test.mjs) | `bazel test //examples/vite/app:unit_test` | Runs the authored test and declared runtime module; bazel coverage selects the declared provider. |
| Native Chromium browser test | [browser/BUILD.bazel](browser/BUILD.bazel) | `bazel test //examples/vite/browser:browser_test` | Runs a real browser using the pinned payload and isolated test lifetime. |
| Tool/platform separation, aliases, reporting and sharding | [../tests/vite/BUILD.bazel](../../tests/vite/BUILD.bazel) | `bazel test //:test` | Existing cases retain exec-plugin/tool-data reconciliation, reports, filters and cancellation coverage. |

The root artifact/test gates include these examples. Deliberately invalid subjects
remain in test fixtures; their owner tests require the expected refusals. Fix and
editor-write commands modify the invoking checkout only when run explicitly.
Automated mutation cases use disposable invoking workspaces.
