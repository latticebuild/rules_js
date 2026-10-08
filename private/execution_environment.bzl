"""Capture declared execution-platform facilities for staged actions."""

visibility("//...")

ExecutionEnvironmentInfo = provider(doc = "Explicit OS facilities for tools on the execution platform.", fields = {"values": "Explicit OS facilities in the execution configuration."})

def _execution_environment_impl(ctx):
    return [ExecutionEnvironmentInfo(values = {
        key: value
        for key, value in ctx.configuration.default_shell_env.items()
        if key.upper() in ["SYSTEMROOT", "WINDIR", "COMSPEC", "PATHEXT", "LD_LIBRARY_PATH"]
    })]

execution_environment = rule(
    implementation = _execution_environment_impl,
    doc = "Captures declared execution-platform OS facilities for private action staging.",
)
