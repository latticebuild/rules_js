"""Supported layout primitives for JavaScript adapters."""

load("//private:layout.bzl", _checked_path = "checked_path", _config_providers = "config_providers", _dependency_links = "dependency_links", _merge_record = "merge_record", _package_records = "package_records", _package_relative_path = "package_relative_path", _place = "place", _relpath = "relpath", _repository_path = "repository_path", _validation_outputs = "validation_outputs")

visibility("public")

checked_path = _checked_path
config_providers = _config_providers
dependency_links = _dependency_links
merge_record = _merge_record
package_records = _package_records
package_relative_path = _package_relative_path
place = _place
relpath = _relpath
repository_path = _repository_path
validation_outputs = _validation_outputs
