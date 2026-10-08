# Runnable examples

Prepare the repository with `mise run bootstrap`, then run:

```sh
bazel build //examples/storybook:artifacts
bazel test //examples/storybook:test
```

| Feature | Source | Command | Expected result |
| --- | --- | --- | --- |
| Svelte story and declared framework dependencies | [svelte/BUILD.bazel](svelte/BUILD.bazel) | `bazel run //examples/storybook/svelte:server` | Starts the configured loopback Storybook server on port 46137. |
| Rendered story and server lifetime | [svelte/Button.stories.ts](svelte/Button.stories.ts) | `bazel test //tests/storybook:server_test` | Chromium observes and clicks the ready button, then shutdown releases the port. |

The root artifact/test gates include these examples. Deliberately invalid subjects
remain in test fixtures; their owner tests require the expected refusals. Fix and
editor-write commands modify the invoking checkout only when run explicitly.
Automated mutation cases use disposable invoking workspaces.
