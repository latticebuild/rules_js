"""Providers shared by the JavaScript rules.

A package record is a struct describing one package directory in a staged
tree:

- name: the node_modules key Node resolves it by.
- repository: the canonical Bazel repository that owns its files.
- package: its repository-relative directory. A package under a node_modules
  directory is installed; any other package belongs to the workspace.
- files: depset of its own files, at repository paths under package.
- links: sorted (link path, package path) pairs binding its workspace imports.
- installs: sorted repository paths of observed node_modules symlinks to it.

Records are immutable so that depsets can hold them. Consumers merge the
records of one directory and reject conflicting identities, files or links.
"""

visibility("public")

JsTsconfigInfo = provider(
    doc = """A declared TypeScript project, independent of compilation.

    DefaultInfo.files holds the project's own files (configuration, sources,
    data and manifest) and what the configurations it extends pass on, without
    its package dependencies.""",
    fields = {
        "compiler_options": "Effective output-affecting options, with config-relative paths: what the configurations it extends pass on, overlaid by its own.",
        "config": "Original configuration File.",
        "inheritance": "depset of the files a configuration extending this one stages: this configuration, its data and what the configurations it extends pass on, such as SvelteKit's generated declarations. Never its sources.",
        "links": "tuple of (link path, package path) import bindings, including those of the configurations it extends.",
        "packages": "depset of the merged package records of deps, aliases and the configurations it extends, one per package directory.",
        "sources": "depset of the project's source inventory.",
    },
)

JsViteConfigInfo = provider(
    doc = """A reusable Vite or Vitest configuration and its declared inputs, without evaluation.

    DefaultInfo.files holds the configuration's own files: configuration,
    manifest, data and the files of non-package dependencies.""",
    fields = {
        "config": "Original configuration File.",
        "links": "tuple of (link path, package path) import bindings.",
        "packages": "depset of the merged package records of deps and aliases, one per package directory.",
    },
)

JsPackageInfo = provider(
    doc = "An npm package: where it lives and the package records a tree needs to hold it.",
    fields = {
        "closure": "depset of package records for this package and everything it reaches. Generated platform edges select the required records.",
        "name": "The node_modules key, such as @latticebuild/base or string-width-cjs.",
        "package": "Repository directory of the package: node_modules/<name> (or a workspace package's own node_modules/<name>) for an installed package, the Bazel package for a workspace package.",
    },
)

JsBinaryInfo = provider(
    doc = "A published package.json bin script, for rules that run it with node in a tree of their own.",
    fields = {
        "bin": "File of the published bin script. Build actions pass this path to node.",
        "files": "depset of script and data files, excluding the executable and Node runtime.",
        "links": "tuple of direct (link path, package path) bindings for a workspace executable's data dependencies.",
        "packages": "depset of the merged package records the script needs, one per package directory.",
    },
)
