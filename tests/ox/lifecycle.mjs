import { inspect } from "node:util";

import { action, call, createContext, ensure, race, run, scoped } from "effection";

// Node stops awaiting a timed-out callback before its task finishes cleanup.
// An after hook joins that same task before the next case can acquire resources.
export function joinTest(context, task) {
  context.after(() =>
    run(function* () {
      try {
        yield* call(() => task);
      } catch (cause) {
        // The callback already reports ordinary body and cleanup failures.
        // A timeout hides hook errors, so also expose its secondary failures.
        if (context.signal.aborted && cause !== context.signal.reason) {
          context.diagnostic(inspect(cause, { depth: null }));
          throw cause;
        }
      }
    }),
  );
  return task;
}

// Cleanup follows registration order: process records must outlive reaping.
const Cleanup = createContext("native test cleanup");

export function* onCleanup(dispose) {
  const state = yield* Cleanup.expect();
  state.disposals.push(dispose);
}

// One failed disposal must not prevent another resource from being released.
export function* disposeAll(disposals) {
  const errors = [];
  while (disposals.length > 0) {
    const dispose = disposals.shift();
    try {
      yield* call(dispose);
    } catch (error) {
      errors.push(error);
    }
  }
  if (errors.length > 0) {
    throw new AggregateError(errors, "Native test cleanup failed");
  }
}

export function* lifecycle(signal, body) {
  const parent = yield* Cleanup.get();
  const state = {
    disposals: [],
    errors: [],
    aborted: false,
    reason: undefined,
    finished: false,
    failed: false,
    primary: undefined,
  };
  return yield* Cleanup.with(state, function* () {
    // A halted nested scope reports disposal failures to its enclosing owner.
    // Throwing here would let ensure replace the native cancellation reason.
    yield* ensure(function* () {
      try {
        yield* disposeAll(state.disposals);
      } catch (cleanup) {
        state.errors.push(...cleanup.errors);
      }
      if (!state.finished) {
        if (parent) {
          // A nested body can fail before cancellation arrives during its drain.
          // Retain it once, unless it is the enclosing cancellation itself.
          if (state.failed && !(parent.aborted && state.primary === parent.reason)) {
            parent.errors.push(state.primary);
          }
          parent.errors.push(...state.errors);
        } else if (state.errors.length > 0) {
          const primary = state.failed ? state.primary : state.reason;
          throw state.failed || state.aborted
            ? new AggregateError(
                [primary, ...state.errors],
                "Native test body and cleanup failed",
                { cause: primary },
              )
            : new AggregateError(state.errors, "Native test cleanup failed");
        } else if (state.failed) {
          throw state.primary;
        }
      }
    });
    let result;
    try {
      result = signal
        ? yield* race([
            body(),
            (function* () {
              yield* action((resolve) => {
                const aborted = () => {
                  state.aborted = true;
                  state.reason = signal.reason;
                  resolve();
                };
                signal.addEventListener("abort", aborted, { once: true });
                if (signal.aborted) {
                  aborted();
                }
                return () => signal.removeEventListener("abort", aborted);
              });
              throw signal.reason;
            })(),
          ])
        : yield* body();
    } catch (error) {
      state.failed = true;
      state.primary = error;
    }
    // Scoped destruction protects the whole drain, including a disposer already
    // removed from the queue, if an enclosing operation is halted mid-cleanup.
    yield* scoped(function* () {
      yield* ensure(function* () {
        try {
          yield* disposeAll(state.disposals);
        } catch (cleanup) {
          state.errors.push(...cleanup.errors);
        }
      });
    });
    if (!state.failed && signal?.aborted) {
      state.failed = true;
      state.primary = signal.reason;
    }
    // Normally caught nested failures are owned by this result, not its parent.
    state.finished = true;
    if (state.errors.length > 0) {
      throw state.failed
        ? new AggregateError(
            [state.primary, ...state.errors],
            "Native test body and cleanup failed",
            {
              cause: state.primary,
            },
          )
        : new AggregateError(state.errors, "Native test cleanup failed");
    }
    if (state.failed) {
      throw state.primary;
    }
    return result;
  });
}

// Adapt an operation result to node:assert's promise-only rejection boundary.
export function* settled(operation) {
  try {
    return Promise.resolve(yield* operation);
  } catch (error) {
    return Promise.reject(error);
  }
}
