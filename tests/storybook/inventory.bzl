"""Resolve the selected native browser marker without a large environment value."""

def _inventory_impl(ctx):
    markers = [file for file in ctx.attr.browser[DefaultInfo].files.to_list() if file.basename == "INSTALLATION_COMPLETE"]
    if len(markers) != 1:
        fail("browser must declare one installation marker")
    marker = markers[0].short_path
    if not marker.startswith("../"):
        fail("browser must come from its declared external repository")
    output = ctx.actions.declare_file(ctx.label.name + ".json")
    ctx.actions.write(output, json.encode({"marker": marker[3:]}))
    return [DefaultInfo(files = depset([output]))]

browser_inventory = rule(implementation = _inventory_impl, attrs = {"browser": attr.label(mandatory = True)})
