"""Keep declared browser payloads native while their installation metadata stays private."""

load("@rules_bound//bound:defs.bzl", "bundle_path", "rlocation")
load("//js/support:layout.bzl", "repository_path")

visibility("//...")

def browser_inputs(ctx):
    """Returns the native inputs and private metadata settings of a declared browser.

    Args:
      ctx: Vitest rule context with browser and runtime environment attributes.

    Returns:
      Private layout files and adapter arguments, plus immutable native runfiles.
    """
    empty = struct(files = [], adapter_args = [], runfiles = None)
    browser = ctx.attr.browser
    if browser == None:
        return empty
    for name in ctx.attr.env.keys() + ctx.attr.env_paths.keys() + ctx.attr.env_inherit:
        if name.upper() == "PLAYWRIGHT_BROWSERS_PATH":
            fail("%s: browser owns PLAYWRIGHT_BROWSERS_PATH; remove its env, env_paths or env_inherit declaration" % ctx.label)

    files = browser[DefaultInfo].files.to_list()
    markers = [file for file in files if file.basename == "INSTALLATION_COMPLETE"]
    if len(markers) != 1:
        fail("%s: browser requires exactly one INSTALLATION_COMPLETE marker" % ctx.label)
    marker = markers[0]
    parts = marker.short_path.split("/")
    if len(parts) < 3:
        fail("%s: browser marker must belong to one .playwright/<browser>-<revision> installation" % ctx.label)
    installation = parts[-2]
    name, separator, revision = installation.rpartition("-")
    if parts[-3] != ".playwright" or name != "chromium" or not separator or not revision.isdigit():
        fail("%s: browser marker must belong to one .playwright/<browser>-<revision> installation" % ctx.label)
    prefix = marker.short_path[:-len(marker.basename)]
    payload = []
    for file in files:
        if not file.short_path.startswith(prefix):
            fail("%s: browser file %s is outside its installation" % (ctx.label, file.short_path))
        if file != marker:
            relative = file.short_path[len(prefix):]
            if relative == "DEPENDENCIES_VALIDATED":
                fail("%s: browser validation metadata must be private, not a declared payload" % ctx.label)
            payload.append(relative)
    if not payload:
        fail("%s: browser declares no native payload" % ctx.label)

    layout = ctx.actions.declare_file(ctx.label.name + ".browser.json")
    ctx.actions.write(layout, json.encode({"marker": rlocation(marker), "files": sorted(payload)}))
    return struct(
        files = [layout],
        adapter_args = ["--browser-layout", bundle_path("tree/" + repository_path(layout))],
        runfiles = ctx.runfiles(files = files).merge(browser[DefaultInfo].default_runfiles),
    )
