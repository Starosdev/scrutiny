# Changelog

All notable changes to Scrutiny will be documented in this file.

## [1.20.2](https://github.com/Starosdev/scrutiny/compare/v1.20.1...v1.20.2) (2026-02-07)

### Bug Fixes

* **ui:** version display clipped on mobile viewports ([#184](https://github.com/Starosdev/scrutiny/issues/184)) ([893fae7](https://github.com/Starosdev/scrutiny/commit/893fae773b52f1359b28c630a95dead4d84d570c)), closes [#167](https://github.com/Starosdev/scrutiny/issues/167)

## [1.20.1](https://github.com/Starosdev/scrutiny/compare/v1.20.0...v1.20.1) (2026-02-06)

### Bug Fixes

* **smart:** Patriot Burst Elite temperature and data unit fixes ([#183](https://github.com/Starosdev/scrutiny/issues/183)) ([f6cb42f](https://github.com/Starosdev/scrutiny/commit/f6cb42ff0bb92353e6e1da4096007bdfd65eec20)), closes [#179](https://github.com/Starosdev/scrutiny/issues/179) [#179](https://github.com/Starosdev/scrutiny/issues/179)

## [1.20.0](https://github.com/Starosdev/scrutiny/compare/v1.19.2...v1.20.0) (2026-02-05)

### Features

* manual release trigger ([00a55c3](https://github.com/Starosdev/scrutiny/commit/00a55c3cbfe1fb1a9df14a8cbcaa40c700158cf4))

## [1.19.2](https://github.com/Starosdev/scrutiny/compare/v1.19.1...v1.19.2) (2026-01-31)

### Bug Fixes

* **overrides:** force_status=failed bypasses threshold filter ([#164](https://github.com/Starosdev/scrutiny/issues/164)) ([#175](https://github.com/Starosdev/scrutiny/issues/175)) ([dc7b914](https://github.com/Starosdev/scrutiny/commit/dc7b914aadaf0dd87f9d87c7d0284b1e7ae0301c))

## [1.19.1](https://github.com/Starosdev/scrutiny/compare/v1.19.0...v1.19.1) (2026-01-31)

### Bug Fixes

* **notify:** fix missed ping notifications not being sent ([#126](https://github.com/Starosdev/scrutiny/issues/126)) ([#174](https://github.com/Starosdev/scrutiny/issues/174)) ([f5bc135](https://github.com/Starosdev/scrutiny/commit/f5bc135f280254e0f8d878ddd76d425faac2cb43))

## [1.19.0](https://github.com/Starosdev/scrutiny/compare/v1.18.0...v1.19.0) (2026-01-30)

### Features

* dark mode toggle, temperature chart visibility, and override recalculation ([#165](https://github.com/Starosdev/scrutiny/issues/165), [#171](https://github.com/Starosdev/scrutiny/issues/171), [#164](https://github.com/Starosdev/scrutiny/issues/164)) ([#173](https://github.com/Starosdev/scrutiny/issues/173)) ([eec5bc4](https://github.com/Starosdev/scrutiny/commit/eec5bc4959458abfdc4b88a342adf07cbcf5f8cc)), closes [#126](https://github.com/Starosdev/scrutiny/issues/126) [#126](https://github.com/Starosdev/scrutiny/issues/126) [#163](https://github.com/Starosdev/scrutiny/issues/163) [#163](https://github.com/Starosdev/scrutiny/issues/163) [#128](https://github.com/Starosdev/scrutiny/issues/128) [#163](https://github.com/Starosdev/scrutiny/issues/163)

## [1.18.0](https://github.com/Starosdev/scrutiny/compare/v1.17.2...v1.18.0) (2026-01-30)

### Features

* **ui:** SMART display mode toggle and Docker fixes ([#163](https://github.com/Starosdev/scrutiny/issues/163)) ([#172](https://github.com/Starosdev/scrutiny/issues/172)) ([ac74f3d](https://github.com/Starosdev/scrutiny/commit/ac74f3d213efde7177078ca328e48e54d4c38c47)), closes [#128](https://github.com/Starosdev/scrutiny/issues/128) [#164](https://github.com/Starosdev/scrutiny/issues/164) [#164](https://github.com/Starosdev/scrutiny/issues/164)

## [1.17.2](https://github.com/Starosdev/scrutiny/compare/v1.17.1...v1.17.2) (2026-01-30)

### Bug Fixes

* **ui:** dark mode improvements and drive filter toggle ([#165](https://github.com/Starosdev/scrutiny/issues/165)) ([#170](https://github.com/Starosdev/scrutiny/issues/170)) ([bc39a72](https://github.com/Starosdev/scrutiny/commit/bc39a723018b4052a2110b1184b9062625d7e072))

## [1.17.1](https://github.com/Starosdev/scrutiny/compare/v1.17.0...v1.17.1) (2026-01-30)

### Bug Fixes

* **diagnostics:** fix missed ping monitor initialization and interface ([#126](https://github.com/Starosdev/scrutiny/issues/126)) ([a8375e6](https://github.com/Starosdev/scrutiny/commit/a8375e6467034ec4004288100123f420a5d143a8))

## [1.17.0](https://github.com/Starosdev/scrutiny/compare/v1.16.3...v1.17.0) (2026-01-27)

### Features

* **api:** improve health check depth with structured response ([#139](https://github.com/Starosdev/scrutiny/issues/139)) ([#153](https://github.com/Starosdev/scrutiny/issues/153)) ([494f8f9](https://github.com/Starosdev/scrutiny/commit/494f8f98050316515ed0fe7126e967d2bd87c0ed))
* **backend:** add container CPU quota awareness with automaxprocs ([#133](https://github.com/Starosdev/scrutiny/issues/133)) ([45a8838](https://github.com/Starosdev/scrutiny/commit/45a88385bdc95a8198c51c145fd7b98c7344ce58)), closes [#72](https://github.com/Starosdev/scrutiny/issues/72) [#82](https://github.com/Starosdev/scrutiny/issues/82) [#71](https://github.com/Starosdev/scrutiny/issues/71) [#71](https://github.com/Starosdev/scrutiny/issues/71) [#71](https://github.com/Starosdev/scrutiny/issues/71) [#71](https://github.com/Starosdev/scrutiny/issues/71) [#71](https://github.com/Starosdev/scrutiny/issues/71) [#71](https://github.com/Starosdev/scrutiny/issues/71) [#74](https://github.com/Starosdev/scrutiny/issues/74)
* **config:** make InfluxDB retention periods configurable ([#138](https://github.com/Starosdev/scrutiny/issues/138)) ([#152](https://github.com/Starosdev/scrutiny/issues/152)) ([b4c25b1](https://github.com/Starosdev/scrutiny/commit/b4c25b114184a1af510c402dd5940d4ba444016a))
* **diagnostics:** add comprehensive diagnostics for missed ping monitoring ([#160](https://github.com/Starosdev/scrutiny/issues/160)) ([4a30a50](https://github.com/Starosdev/scrutiny/commit/4a30a5047c7eb3094835f7dcef698fc30ea0898b)), closes [#126](https://github.com/Starosdev/scrutiny/issues/126) [#126](https://github.com/Starosdev/scrutiny/issues/126)
* **frontend:** improve temperature graph UX ([#40](https://github.com/Starosdev/scrutiny/issues/40)) ([#145](https://github.com/Starosdev/scrutiny/issues/145)) ([23912a5](https://github.com/Starosdev/scrutiny/commit/23912a5152317e89ad399b65edbc74d8516c818a))
* **notify:** add missed collector ping notifications ([#140](https://github.com/Starosdev/scrutiny/issues/140)) ([c2d8bb4](https://github.com/Starosdev/scrutiny/commit/c2d8bb45013a9a7a1cef6fd378d9881466c3ab17)), closes [#72](https://github.com/Starosdev/scrutiny/issues/72) [#82](https://github.com/Starosdev/scrutiny/issues/82) [#71](https://github.com/Starosdev/scrutiny/issues/71) [#71](https://github.com/Starosdev/scrutiny/issues/71) [#71](https://github.com/Starosdev/scrutiny/issues/71) [#71](https://github.com/Starosdev/scrutiny/issues/71) [#71](https://github.com/Starosdev/scrutiny/issues/71) [#71](https://github.com/Starosdev/scrutiny/issues/71) [#74](https://github.com/Starosdev/scrutiny/issues/74)

### Bug Fixes

* **api:** return attribute override ID after save for UI deletion ([#142](https://github.com/Starosdev/scrutiny/issues/142)) ([5ef3d0f](https://github.com/Starosdev/scrutiny/commit/5ef3d0fdc5fbf12c213416a1cd38923a5e632a52)), closes [#141](https://github.com/Starosdev/scrutiny/issues/141)
* **docker:** add SYS_ADMIN capability for NVMe device support ([#159](https://github.com/Starosdev/scrutiny/issues/159)) ([bfddd96](https://github.com/Starosdev/scrutiny/commit/bfddd967588a78cac129fd0f92638883430f3eff)), closes [#26](https://github.com/Starosdev/scrutiny/issues/26) [#209](https://github.com/Starosdev/scrutiny/issues/209)
* **security:** prevent Flux query injection via parameterized queries ([#149](https://github.com/Starosdev/scrutiny/issues/149)) ([0fcb6f5](https://github.com/Starosdev/scrutiny/commit/0fcb6f5c920ef327879581a0765cc00791d6c2d8)), closes [#135](https://github.com/Starosdev/scrutiny/issues/135) [#135](https://github.com/Starosdev/scrutiny/issues/135)
* **validation:** accept serial numbers as WWN fallback for NVMe/SCSI devices ([#158](https://github.com/Starosdev/scrutiny/issues/158)) ([c4daf08](https://github.com/Starosdev/scrutiny/commit/c4daf082b684272733f3735ae4382c5ed4dbc4d8)), closes [#144](https://github.com/Starosdev/scrutiny/issues/144) [#133](https://github.com/Starosdev/scrutiny/issues/133) [#72](https://github.com/Starosdev/scrutiny/issues/72) [#82](https://github.com/Starosdev/scrutiny/issues/82) [#71](https://github.com/Starosdev/scrutiny/issues/71) [#71](https://github.com/Starosdev/scrutiny/issues/71) [#71](https://github.com/Starosdev/scrutiny/issues/71) [#71](https://github.com/Starosdev/scrutiny/issues/71) [#71](https://github.com/Starosdev/scrutiny/issues/71) [#71](https://github.com/Starosdev/scrutiny/issues/71) [#74](https://github.com/Starosdev/scrutiny/issues/74)

### Refactoring

* **backend:** replace fmt.Printf with structured logging ([#136](https://github.com/Starosdev/scrutiny/issues/136)) ([#150](https://github.com/Starosdev/scrutiny/issues/150)) ([4369e9a](https://github.com/Starosdev/scrutiny/commit/4369e9a953de3436efd9677f61f6431b1cd5dec2))
* migrate from moment.js to dayjs for date handling ([#147](https://github.com/Starosdev/scrutiny/issues/147)) ([b6463cc](https://github.com/Starosdev/scrutiny/commit/b6463ccf701196e116ab3f13a6425d95a806755a))

## [1.16.3](https://github.com/Starosdev/scrutiny/compare/v1.16.2...v1.16.3) (2026-01-26)

### Bug Fixes

* **database:** revert parameterized queries for InfluxDB OSS compatibility ([#157](https://github.com/Starosdev/scrutiny/issues/157)) ([6deb0bc](https://github.com/Starosdev/scrutiny/commit/6deb0bc414f1c4274111b40bc7d3f28446c15228)), closes [#155](https://github.com/Starosdev/scrutiny/issues/155)

## [1.16.2](https://github.com/Starosdev/scrutiny/compare/v1.16.1...v1.16.2) (2026-01-26)

### Bug Fixes

* **ci:** improve release notes script output ([accb28d](https://github.com/Starosdev/scrutiny/commit/accb28d794eb9bd9f3433d6744ebc68f9a07f2b9))

## [1.16.1](https://github.com/Starosdev/scrutiny/compare/v1.16.0...v1.16.1) (2026-01-26)

### Bug Fixes

* **ci:** generate release notes from merged PRs ([5b8ff60](https://github.com/Starosdev/scrutiny/commit/5b8ff60e10b2a28f2327766a5f3c8e65475d264b))

## [1.16.0](https://github.com/Starosdev/scrutiny/compare/v1.15.8...v1.16.0) (2026-01-26)

### Features

* manual release trigger ([d7e7bd2](https://github.com/Starosdev/scrutiny/commit/d7e7bd2e6bdfd231d59348186e69b2052fb81512))
## [1.15.8](https://github.com/Starosdev/scrutiny/compare/v1.15.7...v1.15.8) (2026-01-25)

### Refactoring

* migrate from moment.js to dayjs for date handling ([#147](https://github.com/Starosdev/scrutiny/issues/147)) ([ab6584d](https://github.com/Starosdev/scrutiny/commit/ab6584db48f761f332074672d2c11cfeaff36ed6))

## [1.15.7](https://github.com/Starosdev/scrutiny/compare/v1.15.6...v1.15.7) (2026-01-24)

### Bug Fixes

* **docker:** use exec in service scripts to reduce process overhead ([#131](https://github.com/Starosdev/scrutiny/issues/131)) ([43eba12](https://github.com/Starosdev/scrutiny/commit/43eba12b7e574dede4fd2599a14b856369a8e713)), closes [#111](https://github.com/Starosdev/scrutiny/issues/111)

## [1.15.6](https://github.com/Starosdev/scrutiny/compare/v1.15.5...v1.15.6) (2026-01-24)

### Bug Fixes

* **notify:** handle Zulip 60-character topic limit and add force_topic support ([#132](https://github.com/Starosdev/scrutiny/issues/132)) ([a6b45cd](https://github.com/Starosdev/scrutiny/commit/a6b45cd7d6309e74e2fdf934fc3b9d8165c52b95)), closes [#110](https://github.com/Starosdev/scrutiny/issues/110)

## [1.15.5](https://github.com/Starosdev/scrutiny/compare/v1.15.4...v1.15.5) (2026-01-24)

### Bug Fixes

* **backend:** use safe type assertions for SMART metrics parsing ([#130](https://github.com/Starosdev/scrutiny/issues/130)) ([2ec3eb1](https://github.com/Starosdev/scrutiny/commit/2ec3eb1160e072f3c58bc5e7c1583648b1dba412)), closes [#107](https://github.com/Starosdev/scrutiny/issues/107)

## [1.15.4](https://github.com/Starosdev/scrutiny/compare/v1.15.3...v1.15.4) (2026-01-24)

### Bug Fixes

* **notify:** correct repeat notification detection to compare against previous submission ([#129](https://github.com/Starosdev/scrutiny/issues/129)) ([9930980](https://github.com/Starosdev/scrutiny/commit/993098092759d9568aba14de4dfa01188296d6a6)), closes [#67](https://github.com/Starosdev/scrutiny/issues/67)

## [1.15.3](https://github.com/Starosdev/scrutiny/compare/v1.15.2...v1.15.3) (2026-01-24)

### Build

* update Go 1.23 and dependencies for CVE fixes ([bb36d66](https://github.com/Starosdev/scrutiny/commit/bb36d665bc3d04d9d8714152158943596d8e232c))

## [1.15.2](https://github.com/Starosdev/scrutiny/compare/v1.15.1...v1.15.2) (2026-01-24)

### Bug Fixes

* **backend:** scsi wrongly uses nvme metadata ([#124](https://github.com/Starosdev/scrutiny/issues/124)) ([fac6c3e](https://github.com/Starosdev/scrutiny/commit/fac6c3ecbaad55ae78c31e5f3549222a25bc9ae2))

## [1.15.1](https://github.com/Starosdev/scrutiny/compare/v1.15.0...v1.15.1) (2026-01-24)

### Bug Fixes

* **frontend:** improve detail view table layout for issue [#122](https://github.com/Starosdev/scrutiny/issues/122) ([#127](https://github.com/Starosdev/scrutiny/issues/127)) ([b0907f8](https://github.com/Starosdev/scrutiny/commit/b0907f839478455888dc41a70bdb0da7406fe6fe))

## [1.15.0](https://github.com/Starosdev/scrutiny/compare/v1.14.0...v1.15.0) (2026-01-23)

### Features

* **frontend:** add UI for configuring SMART attribute overrides ([#120](https://github.com/Starosdev/scrutiny/issues/120)) ([fa9b54d](https://github.com/Starosdev/scrutiny/commit/fa9b54d2839a9d0621fbda32eb23407b0b896c3a)), closes [#97](https://github.com/Starosdev/scrutiny/issues/97)

## [1.14.0](https://github.com/Starosdev/scrutiny/compare/v1.13.6...v1.14.0) (2026-01-23)

### Features

* **backend:** add SMART attribute overrides support ([#118](https://github.com/Starosdev/scrutiny/issues/118)) ([e113d1f](https://github.com/Starosdev/scrutiny/commit/e113d1f7ab1d2a022e9d12a380abd229aeaad022))

## [1.13.6](https://github.com/Starosdev/scrutiny/compare/v1.13.5...v1.13.6) (2026-01-23)

### Bug Fixes

* **backend:** skip web integration tests when InfluxDB unavailable ([#116](https://github.com/Starosdev/scrutiny/issues/116)) ([ed386d2](https://github.com/Starosdev/scrutiny/commit/ed386d2819b0f0271b6f1471f4be4fa2d1de0b0c)), closes [#78](https://github.com/Starosdev/scrutiny/issues/78)

## [1.13.5](https://github.com/Starosdev/scrutiny/compare/v1.13.4...v1.13.5) (2026-01-23)

### Bug Fixes

* **backend:** reset device status when SMART data passes and add notification logging ([#105](https://github.com/Starosdev/scrutiny/issues/105)) ([72d1773](https://github.com/Starosdev/scrutiny/commit/72d1773439f5a3f2b47456d977ec117d83c090af)), closes [#67](https://github.com/Starosdev/scrutiny/issues/67) [#67](https://github.com/Starosdev/scrutiny/issues/67)

## [1.13.4](https://github.com/Starosdev/scrutiny/compare/v1.13.3...v1.13.4) (2026-01-23)

### Bug Fixes

* **frontend:** remove unused Quill dependency (XSS vulnerability) ([#104](https://github.com/Starosdev/scrutiny/issues/104)) ([15e2a62](https://github.com/Starosdev/scrutiny/commit/15e2a62d1bac0f7674da9c4bf82797ec3575730c)), closes [#69](https://github.com/Starosdev/scrutiny/issues/69)

## [1.13.3](https://github.com/Starosdev/scrutiny/compare/v1.13.2...v1.13.3) (2026-01-23)

### Bug Fixes

* **frontend:** update zfs pool model scrub property names to match the backend response ([#103](https://github.com/Starosdev/scrutiny/issues/103)) ([edce49b](https://github.com/Starosdev/scrutiny/commit/edce49b4e24da4d8698fb446093278b8fde8cb7e))

## [1.13.2](https://github.com/Starosdev/scrutiny/compare/v1.13.1...v1.13.2) (2026-01-22)

### Bug Fixes

* **smart:** correct TB written/read calculation for Intel SSDs ([#101](https://github.com/Starosdev/scrutiny/issues/101)) ([ebef580](https://github.com/Starosdev/scrutiny/commit/ebef580da4e35aa403d374bf150c4b81b304fb1c)), closes [#95](https://github.com/Starosdev/scrutiny/issues/95) [#96](https://github.com/Starosdev/scrutiny/issues/96) [#95](https://github.com/Starosdev/scrutiny/issues/95) [#93](https://github.com/Starosdev/scrutiny/issues/93) [#93](https://github.com/Starosdev/scrutiny/issues/93)

## [1.13.1](https://github.com/Starosdev/scrutiny/compare/v1.13.0...v1.13.1) (2026-01-22)

### Bug Fixes

* **zfs:** ensure pool data updates are persisted to database ([#100](https://github.com/Starosdev/scrutiny/issues/100)) ([e37a924](https://github.com/Starosdev/scrutiny/commit/e37a924bf2f85e4f7a39b9bc5254d6e99d0fcb42)), closes [#91](https://github.com/Starosdev/scrutiny/issues/91)

## [1.13.0](https://github.com/Starosdev/scrutiny/compare/v1.12.1...v1.13.0) (2026-01-22)

### Features

* **dashboard:** add SSD health metrics to dashboard cards ([#99](https://github.com/Starosdev/scrutiny/issues/99)) ([d615d78](https://github.com/Starosdev/scrutiny/commit/d615d78bc626630b9accdec39c9e8b6eefcc71ce)), closes [#95](https://github.com/Starosdev/scrutiny/issues/95) [#96](https://github.com/Starosdev/scrutiny/issues/96) [#95](https://github.com/Starosdev/scrutiny/issues/95)

## [1.12.1](https://github.com/Starosdev/scrutiny/compare/v1.12.0...v1.12.1) (2026-01-22)

### Bug Fixes

* **smart:** prevent false failures from corrupted ATA device statistics ([#98](https://github.com/Starosdev/scrutiny/issues/98)) ([126307f](https://github.com/Starosdev/scrutiny/commit/126307f3301960ef989bf0da1b2bb60cd3f547cd)), closes [#84](https://github.com/Starosdev/scrutiny/issues/84) [#84](https://github.com/Starosdev/scrutiny/issues/84)

## [1.12.0](https://github.com/Starosdev/scrutiny/compare/v1.11.0...v1.12.0) (2026-01-22)

### Features

* **detail:** add SSD health metrics to detail component ([#96](https://github.com/Starosdev/scrutiny/issues/96)) ([4713199](https://github.com/Starosdev/scrutiny/commit/4713199cdae5ac0d4c35d9443ad811b735412ade))

## [1.11.0](https://github.com/Starosdev/scrutiny/compare/v1.10.1...v1.11.0) (2026-01-21)

### Features

* written and read TBs ([#74](https://github.com/Starosdev/scrutiny/issues/74)) ([10698c3](https://github.com/Starosdev/scrutiny/commit/10698c32e089fca86f4ab0d2d0ade1b53cdec94b))

## [1.10.1](https://github.com/Starosdev/scrutiny/compare/v1.10.0...v1.10.1) (2026-01-20)

### Bug Fixes

* **mock:** enhance ZFS pool management methods in MockDeviceRepo ([c8b22fd](https://github.com/Starosdev/scrutiny/commit/c8b22fdcd6a8fc4f75a09ddb42453318110d2062))

## [1.10.0](https://github.com/Starosdev/scrutiny/compare/v1.9.1...v1.10.0) (2026-01-19)

### Features

* **dashboard:** add more sorting options ([#80](https://github.com/Starosdev/scrutiny/issues/80)) ([88ef36e](https://github.com/Starosdev/scrutiny/commit/88ef36e0d227437a9cbc4d8da9ac01bb6a80d5d5)), closes [#72](https://github.com/Starosdev/scrutiny/issues/72)
* **docker:** add ZFS collector to omnibus image ([#82](https://github.com/Starosdev/scrutiny/issues/82)) ([9ed219d](https://github.com/Starosdev/scrutiny/commit/9ed219d0e9779682f661c7dd073360f52179ee5e))
* **frontend:** add attribute history dialog for sparkline charts ([a1a67cf](https://github.com/Starosdev/scrutiny/commit/a1a67cf01063ac31c135ba39b9b4b88f994e7ed9)), closes [#71](https://github.com/Starosdev/scrutiny/issues/71)

### Bug Fixes

* **ci:** limit ZFS collector to amd64 only ([46e4938](https://github.com/Starosdev/scrutiny/commit/46e4938b52fdbd98497a4d3ba4e6ea4e432d84ba))
* **ci:** remove arm/v7 from ZFS collector platforms ([d5ce8be](https://github.com/Starosdev/scrutiny/commit/d5ce8bee221b446592abe9fdd1e960563f44f551))
* **docker:** enable contrib repo for zfsutils-linux package ([c9fc565](https://github.com/Starosdev/scrutiny/commit/c9fc565b39d529f1f64397784fa4acef3edf9fb1))
* **frontend:** add debounce to sparkline hover to prevent flickering ([e987360](https://github.com/Starosdev/scrutiny/commit/e987360312be7de732a180bb70d6011ad70b7c52)), closes [#71](https://github.com/Starosdev/scrutiny/issues/71)
* **frontend:** disable tooltips on sparkline charts ([853d580](https://github.com/Starosdev/scrutiny/commit/853d5802b6cc9812ca2a1f611f7f9ccb5db91bda)), closes [#71](https://github.com/Starosdev/scrutiny/issues/71)
* **frontend:** prevent tooltip cutoff on sparkline charts ([1377179](https://github.com/Starosdev/scrutiny/commit/13771792c4c8474249bed66618c5430f58124ae3)), closes [#71](https://github.com/Starosdev/scrutiny/issues/71)
* **frontend:** use ApexCharts native events for tooltip overflow fix ([0a16d4e](https://github.com/Starosdev/scrutiny/commit/0a16d4e127a2128d14bb5385f1d5c49819fa86ee)), closes [#71](https://github.com/Starosdev/scrutiny/issues/71)
* **frontend:** use fixed tooltip position for sparkline charts ([a81e08a](https://github.com/Starosdev/scrutiny/commit/a81e08a6a9331daf1fb3a04d0005da2b909ca04a)), closes [#71](https://github.com/Starosdev/scrutiny/issues/71)

## [1.9.1](https://github.com/Starosdev/scrutiny/compare/v1.9.0...v1.9.1) (2026-01-19)

### Bug Fixes

* **frontend:** resolve issues related to display zfs pool data ([#87](https://github.com/Starosdev/scrutiny/issues/87)) ([00f9011](https://github.com/Starosdev/scrutiny/commit/00f9011b3c745b91fd8749bb77fbfc8c0012b37a))

## [1.8.0](https://github.com/Starosdev/scrutiny/compare/v1.7.2...v1.8.0) (2026-01-17)

### Features

* **thresholds:** add metadata for all ATA Device Statistics ([6602bf8](https://github.com/Starosdev/scrutiny/commit/6602bf83aea4fbc625a71e1232b732078b511694))
* **thresholds:** add metadata for all remaining unknown attributes ([163284c](https://github.com/Starosdev/scrutiny/commit/163284c196eca32a4ed25beaefb9c6bc536f91be))
* **thresholds:** add metadata for Page 3 and Page 5 device statistics ([72a7ea8](https://github.com/Starosdev/scrutiny/commit/72a7ea84ad8d60a1c66d7d9adf42c254bc4dd863))
* **thresholds:** add metadata for remaining unknown attributes ([7b0acd0](https://github.com/Starosdev/scrutiny/commit/7b0acd0b572a392c9c74c5f61f2860e671b44b3e))

### Bug Fixes

* **frontend:** handle device statistics display in detail view ([1f81689](https://github.com/Starosdev/scrutiny/commit/1f816898da5c628bdd29ef7825e1e38c5e3c9ecc))
* **smart:** add support for ATA Device Statistics (enterprise SSD metrics) ([79d7841](https://github.com/Starosdev/scrutiny/commit/79d784140d7faee7c979047843d4825316bf3603)), closes [#7](https://github.com/Starosdev/scrutiny/issues/7)

## [1.7.2](https://github.com/Starosdev/scrutiny/compare/v1.7.1...v1.7.2) (2026-01-17)

### Bug Fixes

* **mock:** add ZFS pool management methods to MockDeviceRepo ([af2d4bd](https://github.com/Starosdev/scrutiny/commit/af2d4bdb78c9b932a5ba63b73e274212b4386d8e))

## [1.7.1](https://github.com/Starosdev/scrutiny/compare/v1.7.0...v1.7.1) (2026-01-09)

### Bug Fixes

* **deps:** security audit and dependency inventory ([b42e940](https://github.com/Starosdev/scrutiny/commit/b42e94059ac19db10794517ec3bef027558d03e8)), closes [#69](https://github.com/Starosdev/scrutiny/issues/69) [#70](https://github.com/Starosdev/scrutiny/issues/70) [#36](https://github.com/Starosdev/scrutiny/issues/36)

## [1.7.0](https://github.com/Starosdev/scrutiny/compare/v1.6.2...v1.7.0) (2026-01-08)

### Features

* **zfs:** add ZFS pool monitoring support ([6df294a](https://github.com/Starosdev/scrutiny/commit/6df294a8c208f2c2db3a8fecb49d764d47704bbf)), closes [#66](https://github.com/Starosdev/scrutiny/issues/66)

## [1.6.2](https://github.com/Starosdev/scrutiny/compare/v1.6.1...v1.6.2) (2026-01-08)

### Bug Fixes

* **docker:** correct Angular 21 frontend build paths ([18d464b](https://github.com/Starosdev/scrutiny/commit/18d464bad430cb7e3d36f97cd55a7829a79b040f)), closes [#59](https://github.com/Starosdev/scrutiny/issues/59)

## [1.6.1](https://github.com/Starosdev/scrutiny/compare/v1.6.0...v1.6.1) (2026-01-08)

### Bug Fixes

* **ci:** correct frontend tarball path in release workflow ([d46b8f0](https://github.com/Starosdev/scrutiny/commit/d46b8f0696c71ac64af9fd8c3fb7890e44e9db3d)), closes [#59](https://github.com/Starosdev/scrutiny/issues/59)
* **ci:** make frontend coverage upload optional ([5cc5ed1](https://github.com/Starosdev/scrutiny/commit/5cc5ed134044a6dc6d2d2df521e662083bb0b696))

## [1.6.0](https://github.com/Starosdev/scrutiny/compare/v1.5.0...v1.6.0) (2026-01-08)

### Features

* **frontend:** Upgrade Angular 13 to Angular 21 ([d9e4b6a](https://github.com/Starosdev/scrutiny/commit/d9e4b6ad5753a9c1d343a7a44b1bd145dafb92ba)), closes [#9](https://github.com/Starosdev/scrutiny/issues/9)

## [1.5.0](https://github.com/Starosdev/scrutiny/compare/v1.4.1...v1.5.0) (2026-01-08)

### Features

* **notify:** add device label to notification payload ([#48](https://github.com/Starosdev/scrutiny/issues/48)) ([231cc4c](https://github.com/Starosdev/scrutiny/commit/231cc4c2e11d3d04df4aa1a076f9fe839ca5bc56))

### Bug Fixes

* batch of quick wins from GitHub issues ([5eef50e](https://github.com/Starosdev/scrutiny/commit/5eef50e13ba71f400a9846004fedff05e431afed)), closes [#47](https://github.com/Starosdev/scrutiny/issues/47) [#50](https://github.com/Starosdev/scrutiny/issues/50) [#47](https://github.com/Starosdev/scrutiny/issues/47) [#50](https://github.com/Starosdev/scrutiny/issues/50) [#8](https://github.com/Starosdev/scrutiny/issues/8) [#56](https://github.com/Starosdev/scrutiny/issues/56) [#59](https://github.com/Starosdev/scrutiny/issues/59) [#26](https://github.com/Starosdev/scrutiny/issues/26)
* **collector:** populate DeviceType from smartctl info when not set ([6704245](https://github.com/Starosdev/scrutiny/commit/670424567ab2f9d262fc1f3b04fbbf081fc0267e))
* **tests:** add GetString notify.urls mock for notify.Send() ([a2a9f71](https://github.com/Starosdev/scrutiny/commit/a2a9f7109cfe17dce91b036b5b5ad906ea477a55))
* **tests:** add index.html to all web tests for health check ([e1ee2c3](https://github.com/Starosdev/scrutiny/commit/e1ee2c31c5fc341eb23225dde9733a0602c43ffe))
* **tests:** add missing config mock expectations for GORM logging ([5a74c7c](https://github.com/Starosdev/scrutiny/commit/5a74c7c2b050661a45b5aa80779eea36ff318ea2))
* **tests:** add missing config mocks for GORM logging ([2f312cf](https://github.com/Starosdev/scrutiny/commit/2f312cf52ab0f52b5c4f49ad52fd45eefdf53fa8))
* **tests:** add web.metrics.enabled mock ([6418581](https://github.com/Starosdev/scrutiny/commit/6418581a96b26291788beb938f504e1026b93995))
* **tests:** add web.metrics.enabled mock to all test blocks ([3b297c2](https://github.com/Starosdev/scrutiny/commit/3b297c27eff799aae02e8b86d90f9aa418eba8a8))

### Build

* disable VCS stamping in binary builds ([010d287](https://github.com/Starosdev/scrutiny/commit/010d287a108ec4a9069f5d683b33a05f91ec9e81))

## [1.4.3](https://github.com/Starosdev/scrutiny/compare/v1.4.2...v1.4.3) (2026-01-08)

### Build

* disable VCS stamping in binary builds ([010d287](https://github.com/Starosdev/scrutiny/commit/010d287a108ec4a9069f5d683b33a05f91ec9e81))

## [1.4.2](https://github.com/Starosdev/scrutiny/compare/v1.4.1...v1.4.2) (2026-01-08)

### Bug Fixes

* batch of quick wins from GitHub issues ([5eef50e](https://github.com/Starosdev/scrutiny/commit/5eef50e13ba71f400a9846004fedff05e431afed)), closes [#47](https://github.com/Starosdev/scrutiny/issues/47) [#50](https://github.com/Starosdev/scrutiny/issues/50) [#47](https://github.com/Starosdev/scrutiny/issues/47) [#50](https://github.com/Starosdev/scrutiny/issues/50) [#8](https://github.com/Starosdev/scrutiny/issues/8) [#56](https://github.com/Starosdev/scrutiny/issues/56) [#59](https://github.com/Starosdev/scrutiny/issues/59) [#26](https://github.com/Starosdev/scrutiny/issues/26)
* **collector:** populate DeviceType from smartctl info when not set ([6704245](https://github.com/Starosdev/scrutiny/commit/670424567ab2f9d262fc1f3b04fbbf081fc0267e))
* **tests:** add GetString notify.urls mock for notify.Send() ([a2a9f71](https://github.com/Starosdev/scrutiny/commit/a2a9f7109cfe17dce91b036b5b5ad906ea477a55))
* **tests:** add index.html to all web tests for health check ([e1ee2c3](https://github.com/Starosdev/scrutiny/commit/e1ee2c31c5fc341eb23225dde9733a0602c43ffe))
* **tests:** add missing config mock expectations for GORM logging ([5a74c7c](https://github.com/Starosdev/scrutiny/commit/5a74c7c2b050661a45b5aa80779eea36ff318ea2))
* **tests:** add missing config mocks for GORM logging ([2f312cf](https://github.com/Starosdev/scrutiny/commit/2f312cf52ab0f52b5c4f49ad52fd45eefdf53fa8))
* **tests:** add web.metrics.enabled mock ([6418581](https://github.com/Starosdev/scrutiny/commit/6418581a96b26291788beb938f504e1026b93995))
* **tests:** add web.metrics.enabled mock to all test blocks ([3b297c2](https://github.com/Starosdev/scrutiny/commit/3b297c27eff799aae02e8b86d90f9aa418eba8a8))

## [1.4.1](https://github.com/Starosdev/scrutiny/compare/v1.4.0...v1.4.1) (2026-01-08)

### Bug Fixes

* batch of quick wins from GitHub issues ([#60](https://github.com/Starosdev/scrutiny/issues/60)) ([a11d619](https://github.com/Starosdev/scrutiny/commit/a11d619a893458949e67560ff96ee6881dcf13b5))

## [1.3.0](https://github.com/Starosdev/scrutiny/compare/v1.2.0...v1.3.0) (2025-12-20)

### Features

* add device label editing and API timeout configuration ([75050d5](https://github.com/Starosdev/scrutiny/commit/75050d57fa28fe59e833c417671667f43effc472))

## [1.2.0](https://github.com/Starosdev/scrutiny/compare/v1.1.2...v1.2.0) (2025-12-19)

### Features

* **ci:** add SHA256 checksums to GitHub releases ([367a2dc](https://github.com/Starosdev/scrutiny/commit/367a2dc27e95cf17b95f4ea672154c0f8d871cbf)), closes [#28](https://github.com/Starosdev/scrutiny/issues/28)

### Bug Fixes

* Frontend Demo Mode now loads ([#57](https://github.com/Starosdev/scrutiny/issues/57)) ([462a0c3](https://github.com/Starosdev/scrutiny/commit/462a0c362ce5a7b8f5f04a81fe3076fbce4073a8))

## [1.1.2](https://github.com/Starosdev/scrutiny/compare/v1.1.1...v1.1.2) (2025-12-18)

### Refactoring

* **database:** extract hardcoded time ranges to constants ([deb2df0](https://github.com/Starosdev/scrutiny/commit/deb2df0bc718461c5a9826d6b6c1c1307b7122e8)), closes [#49](https://github.com/Starosdev/scrutiny/issues/49)

## [1.1.1](https://github.com/Starosdev/scrutiny/compare/v1.1.0...v1.1.1) (2025-12-09)

### Bug Fixes

* **collector:** handle large LBA values in SMART data parsing ([7f4bceb](https://github.com/Starosdev/scrutiny/commit/7f4bceb85506606d6318253fd406da4b55921383)), closes [#24](https://github.com/Starosdev/scrutiny/issues/24) [AnalogJ/scrutiny#800](https://github.com/AnalogJ/scrutiny/issues/800)
* **collector:** ignore bit 6 in smartctl exit-code during detect ([735fe2e](https://github.com/Starosdev/scrutiny/commit/735fe2e57d9afc9d32832619d6c3c758ec91eb11))
* **collector:** keep existing device type ([b5bb1a2](https://github.com/Starosdev/scrutiny/commit/b5bb1a232a2e38e6bbffb041ffa397b54999fc02))
* **config:** use structured logging for config file messages ([03513b7](https://github.com/Starosdev/scrutiny/commit/03513b742622b77d27cd08b941147eadf35bec91)), closes [#22](https://github.com/Starosdev/scrutiny/issues/22) [AnalogJ/scrutiny#814](https://github.com/AnalogJ/scrutiny/issues/814)
* **database:** use WAL mode to prevent readonly errors in restricted Docker ([1db337d](https://github.com/Starosdev/scrutiny/commit/1db337d872b655e0c68a4a506f9706f0cb7d4a79)), closes [#25](https://github.com/Starosdev/scrutiny/issues/25) [AnalogJ/scrutiny#772](https://github.com/AnalogJ/scrutiny/issues/772)
* **notify:** try to unmarshal notify.urls as JSON array ([9109fb5](https://github.com/Starosdev/scrutiny/commit/9109fb5447080b5faab3377721b830f1e0266500))
* **thresholds:** add observed threshold for attribute 188 with value 0 ([c86ee89](https://github.com/Starosdev/scrutiny/commit/c86ee894468068830fa9e8cf93cde3ef6df1f5d0))
* **thresholds:** mark wear leveling count (attr 177) as critical ([c072119](https://github.com/Starosdev/scrutiny/commit/c0721199b86b02ae398afcc439f4162a760f1d5e)), closes [#21](https://github.com/Starosdev/scrutiny/issues/21) [AnalogJ/scrutiny#818](https://github.com/AnalogJ/scrutiny/issues/818)
* **ui:** display temperature graph times in local timezone ([6123347](https://github.com/Starosdev/scrutiny/commit/6123347165794a5de177248802229c9ea0ea4a9f)), closes [#30](https://github.com/Starosdev/scrutiny/issues/30)

## [1.1.0](https://github.com/Starosdev/scrutiny/compare/v1.0.0...v1.1.0) (2025-11-30)

### Features

* Add "day" as resolution for temperature graph ([2670af2](https://github.com/Starosdev/scrutiny/commit/2670af216d491c478b36f8ef20497c5cb6002801))
* add day resolution for temperature graph (upstream PR [#823](https://github.com/Starosdev/scrutiny/issues/823)) ([2d6ffa7](https://github.com/Starosdev/scrutiny/commit/2d6ffa732cda4583c0f867540bed87a331fbb6d4))
* add setting to enable/disable SCT temperature history (upstream PR [#557](https://github.com/Starosdev/scrutiny/issues/557)) ([c3692ac](https://github.com/Starosdev/scrutiny/commit/c3692acd17e310e1c5d1470404566ae13e67d9a5))
* Implement device-wise notification mute/unmute ([925e86d](https://github.com/Starosdev/scrutiny/commit/925e86d461fc2bfe4f318851d790a08d99eb6bde))
* implement device-wise notification mute/unmute (upstream PR [#822](https://github.com/Starosdev/scrutiny/issues/822)) ([ea7102e](https://github.com/Starosdev/scrutiny/commit/ea7102e9297aeb011a808f1133fbf03114176900))
* implement Prometheus metrics support (upstream PR [#830](https://github.com/Starosdev/scrutiny/issues/830)) ([7384f7d](https://github.com/Starosdev/scrutiny/commit/7384f7de6ebf8f6c3936fb52d19ffe3b805bae0c))
* support SAS temperature (upstream PR [#816](https://github.com/Starosdev/scrutiny/issues/816)) ([f954cc8](https://github.com/Starosdev/scrutiny/commit/f954cc815f756bef8842f026a5a0e554bfd5ba80))

### Bug Fixes

* better handling of ata_sct_temperature_history (upstream PR [#825](https://github.com/Starosdev/scrutiny/issues/825)) ([d134ad7](https://github.com/Starosdev/scrutiny/commit/d134ad7160b754ad25d10d600a6fc8e56c0d5914))
* **database:** add missing temperature parameter in SCSI migration ([df7da88](https://github.com/Starosdev/scrutiny/commit/df7da8824c3cd3745f66ae426bcec1db7844e840))
* support transient SMART failures (upstream PR [#375](https://github.com/Starosdev/scrutiny/issues/375)) ([601775e](https://github.com/Starosdev/scrutiny/commit/601775e462f6cd56d442386071c6499dfba3cc39))
* **ui:** fix temperature conversion in temperature.pipe.ts (upstream PR [#815](https://github.com/Starosdev/scrutiny/issues/815)) ([e0f2781](https://github.com/Starosdev/scrutiny/commit/e0f27819facc20c6f04c8903f2ebb85035475b47))

### Refactoring

* use limit() instead of tail() for fetching smart attributes (upstream PR [#829](https://github.com/Starosdev/scrutiny/issues/829)) ([2849531](https://github.com/Starosdev/scrutiny/commit/2849531d3893028861cec68f862d4ed32bedbb0c))

## 1.0.0 (2025-11-29)

### Features

* Ability to override commands args ([604dcf3](https://github.com/Starosdev/scrutiny/commit/604dcf355ce387de5b5030473163838c5855fa31))
* create allow-list for filtering down devices to only a subset ([c9429c6](https://github.com/Starosdev/scrutiny/commit/c9429c61b2aa7dbea9ed412bd9d49326cf408e94))
* dynamic line stroke settings ([536b590](https://github.com/Starosdev/scrutiny/commit/536b590080b589a807765b69612990d41ae97773))
* Update dashboard.component.ts ([bb98b8c](https://github.com/Starosdev/scrutiny/commit/bb98b8c45b13d9b01c3a543022608fb746b207d6))

### Bug Fixes

* **collector:** show correct nvme capacity ([db86bac](https://github.com/Starosdev/scrutiny/commit/db86bac9efb10ca11177a1cf00621a8ea91dc6aa)), closes [#466](https://github.com/Starosdev/scrutiny/issues/466)
* https://github.com/AnalogJ/scrutiny/issues/643 ([50561f3](https://github.com/Starosdev/scrutiny/commit/50561f34ead034c118dd7ea5f1d1f067b0d1d97a))
* igeneric types ([e9cf8a9](https://github.com/Starosdev/scrutiny/commit/e9cf8a9180e5d181f62076bb602888e34596885b))
* increase timeout ([222b810](https://github.com/Starosdev/scrutiny/commit/222b8103d635ddfafd29ac93ea110c3d851a3112))
* prod build command ([50321d8](https://github.com/Starosdev/scrutiny/commit/50321d897a21faa515b142f4b2e285ba16815acd))
* remove fullcalendar ([64ad353](https://github.com/Starosdev/scrutiny/commit/64ad3536284f67cb4652a9e83a02f0024b7dcde9))
* remove outdated option ([5518865](https://github.com/Starosdev/scrutiny/commit/5518865bc69f0a9906977facfa4be8895a7b12d9))

### Refactoring

* update dependencies version ([e18a7e9](https://github.com/Starosdev/scrutiny/commit/e18a7e9ce08e9172853f7bd5f6a6388e278ee4e2))
