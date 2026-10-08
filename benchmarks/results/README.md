# Benchmark snapshot

Measured on 2026-10-08 at [3cfdea10f885](https://github.com/latticebuild/rules_js/commit/3cfdea10f885b5182d864cf90925107a1032f598), using [native CI](https://github.com/latticebuild/rules_js/actions/runs/37777521987). All three hosts passed the correctness, dependency parity, source identity, native runner and cleanup gates.

| Platform | Application dependencies | Latticebuild median | Aspect median | Ratio (Latticebuild / Aspect) | Paired 95% interval |
| --- | --- | --- | --- | --- | --- |
| Linux x64 | None | 35.5 ms | 41 ms | 0.866× | [0.833, 0.900] |
| Linux x64 | 54 package instances | 338.5 ms | 127 ms | 2.665× | [2.625, 2.717] |
| macOS 27 ARM64 | None | 85.5 ms | 89 ms | 0.961× | [0.863, 1.141] |
| macOS 27 ARM64 | 55 package instances | 1099 ms | 231.5 ms | 4.747× | [4.198, 5.229] |
| Windows x64 | None | 117 ms | 334.5 ms | 0.350× | [0.345, 0.359] |
| Windows x64 | 54 package instances | 1489.5 ms | 549 ms | 2.713× | [2.623, 2.788] |

Each median uses all 30 retained matched pairs after three warmup pairs. Lower ratios favor Latticebuild. The interval comes from 10,000 paired bootstrap resamples. The macOS no-dependency result is inconclusive; its interval overlaps a tie. These short actions measure staging and package resolution overhead. Compiler and bundler workloads have their own timings.

The JSON files are complete, unmodified reports with all warmup and retained pair timings, backend order, input nonces, output hashes, preparation/first/no-op timings, host details, pinned versions and inventory summaries. [manifest.json](manifest.json) records their SHA-256 digests. The CI artifacts contain the full execution logs, package inventories, payload hashes and Linux host preflight diagnostics.

- [Linux report](2026-10-08-ubuntu-24.04.json)
- [macOS 27 report](2026-10-08-xcode-27.json)
- [Windows report](2026-10-08-windows-2025.json)
- [Fixture and methodology](../../README.md#action-benchmark)

This snapshot records the measured source revision. Documentation commits can follow it; current CI continues to run both complete benchmark cases.
