# Changelog

## [1.5.1](https://github.com/peter-trerotola/go-postgres-mcp/compare/v1.5.0...v1.5.1) (2026-03-08)


### Bug Fixes

* report CI status to PRs on workflow_dispatch runs ([#45](https://github.com/peter-trerotola/go-postgres-mcp/issues/45)) ([937a46f](https://github.com/peter-trerotola/go-postgres-mcp/commit/937a46ffc22be7e104588bc56d44045e24c311d3))

## [1.5.0](https://github.com/peter-trerotola/go-postgres-mcp/compare/v1.4.0...v1.5.0) (2026-03-08)


### Features

* add automatic releases via release-please ([a7c1d16](https://github.com/peter-trerotola/go-postgres-mcp/commit/a7c1d16585454eab4a8a8c0c43d54400808b3279))
* inject knowledge map context into MCP responses ([88b94c0](https://github.com/peter-trerotola/go-postgres-mcp/commit/88b94c0dfe9499d3bf7592d2eb4b66fbf8eb4349))
* inject knowledge map context into MCP responses ([ea9b8d2](https://github.com/peter-trerotola/go-postgres-mcp/commit/ea9b8d25c50d5f44e1385a3267505e155d0ded31))


### Bug Fixes

* add actions:write permission to trigger-release workflow ([#35](https://github.com/peter-trerotola/go-postgres-mcp/issues/35)) ([d3ad890](https://github.com/peter-trerotola/go-postgres-mcp/commit/d3ad890cb5df7cfcd7e95c2b7f9db9ef668b24dd))
* add workflow_dispatch trigger to release workflow ([#24](https://github.com/peter-trerotola/go-postgres-mcp/issues/24)) ([af2b57c](https://github.com/peter-trerotola/go-postgres-mcp/commit/af2b57c26764c3ba30062b2382e5ede4d96c4758))
* address PR review comments ([2c61301](https://github.com/peter-trerotola/go-postgres-mcp/commit/2c613015628d6a97a72523f1949e3b581734da46))
* address PR review feedback ([50f501c](https://github.com/peter-trerotola/go-postgres-mcp/commit/50f501c187e8ad181a0f68959f4ddb57b9c45965))
* address PR review feedback ([ab4bada](https://github.com/peter-trerotola/go-postgres-mcp/commit/ab4badac8e9b6e6931a0e2c9c3363ab461a979f4))
* address PR review feedback ([116e087](https://github.com/peter-trerotola/go-postgres-mcp/commit/116e087fa3491873376f6a1fe133a2979a4d6695))
* address PR review feedback (round 2) ([1833ef9](https://github.com/peter-trerotola/go-postgres-mcp/commit/1833ef934c18167f6bd797fcfe58ffeb3304d07b))
* auto-trigger CI on release-please PRs ([#32](https://github.com/peter-trerotola/go-postgres-mcp/issues/32)) ([6e0d526](https://github.com/peter-trerotola/go-postgres-mcp/commit/6e0d526643ddbf043c843559dbc8906dfaf83709))
* auto-trigger release workflow on PR merge ([#30](https://github.com/peter-trerotola/go-postgres-mcp/issues/30)) ([4a62f1e](https://github.com/peter-trerotola/go-postgres-mcp/commit/4a62f1e80b85ea8c73263a5b12bf0f6eb62ff4c6))
* avoid fromJSON crash when release-please pr output is empty ([#37](https://github.com/peter-trerotola/go-postgres-mcp/issues/37)) ([e483070](https://github.com/peter-trerotola/go-postgres-mcp/commit/e4830701e7ba8c158433cc0445c516fb999237ab))
* create release with assets in single step for immutable releases ([#39](https://github.com/peter-trerotola/go-postgres-mcp/issues/39)) ([f13d312](https://github.com/peter-trerotola/go-postgres-mcp/commit/f13d312d142387d74b15a26ba183217f4c0d8d08))
* delete and recreate release to work with immutable releases ([#41](https://github.com/peter-trerotola/go-postgres-mcp/issues/41)) ([39224ef](https://github.com/peter-trerotola/go-postgres-mcp/commit/39224ef0f559343d1749d1db27eb0ad7155b9414))
* discovery FK violations and concurrent schema crawling ([190f3c9](https://github.com/peter-trerotola/go-postgres-mcp/commit/190f3c9288e7e0ed2225430d35069349614c5515))
* discovery FK violations and concurrent schema crawling ([be504d7](https://github.com/peter-trerotola/go-postgres-mcp/commit/be504d7ed4d397fb55ab921b08b16135fec0f7f9))
* parse release-please pr output as JSON ([#34](https://github.com/peter-trerotola/go-postgres-mcp/issues/34)) ([1023b7c](https://github.com/peter-trerotola/go-postgres-mcp/commit/1023b7c07fbf18746a50994848df3020cd93c5cf))
* simplify release asset upload ([#43](https://github.com/peter-trerotola/go-postgres-mcp/issues/43)) ([74d6b98](https://github.com/peter-trerotola/go-postgres-mcp/commit/74d6b98e1c67da824229675166e17a81ac0b0003))
* upgrade all dependencies and add macOS builds ([#22](https://github.com/peter-trerotola/go-postgres-mcp/issues/22)) ([563da01](https://github.com/peter-trerotola/go-postgres-mcp/commit/563da01a234d0d06af5d1749757733c4c217fb70))
* upgrade all dependencies and GitHub Actions ([#20](https://github.com/peter-trerotola/go-postgres-mcp/issues/20)) ([1c238f3](https://github.com/peter-trerotola/go-postgres-mcp/commit/1c238f3266deb1e6cf8a8852690d940e92ef80d1))
* use draft releases to support immutable release assets ([#23](https://github.com/peter-trerotola/go-postgres-mcp/issues/23)) ([b8cc3c6](https://github.com/peter-trerotola/go-postgres-mcp/commit/b8cc3c6ce422971bc4da513035c6c0c36857f104))

## [1.4.0](https://github.com/peter-trerotola/go-postgres-mcp/compare/v1.3.0...v1.4.0) (2026-03-08)


### Features

* add automatic releases via release-please ([a7c1d16](https://github.com/peter-trerotola/go-postgres-mcp/commit/a7c1d16585454eab4a8a8c0c43d54400808b3279))
* inject knowledge map context into MCP responses ([88b94c0](https://github.com/peter-trerotola/go-postgres-mcp/commit/88b94c0dfe9499d3bf7592d2eb4b66fbf8eb4349))
* inject knowledge map context into MCP responses ([ea9b8d2](https://github.com/peter-trerotola/go-postgres-mcp/commit/ea9b8d25c50d5f44e1385a3267505e155d0ded31))


### Bug Fixes

* add actions:write permission to trigger-release workflow ([#35](https://github.com/peter-trerotola/go-postgres-mcp/issues/35)) ([d3ad890](https://github.com/peter-trerotola/go-postgres-mcp/commit/d3ad890cb5df7cfcd7e95c2b7f9db9ef668b24dd))
* add workflow_dispatch trigger to release workflow ([#24](https://github.com/peter-trerotola/go-postgres-mcp/issues/24)) ([af2b57c](https://github.com/peter-trerotola/go-postgres-mcp/commit/af2b57c26764c3ba30062b2382e5ede4d96c4758))
* address PR review comments ([2c61301](https://github.com/peter-trerotola/go-postgres-mcp/commit/2c613015628d6a97a72523f1949e3b581734da46))
* address PR review feedback ([50f501c](https://github.com/peter-trerotola/go-postgres-mcp/commit/50f501c187e8ad181a0f68959f4ddb57b9c45965))
* address PR review feedback ([ab4bada](https://github.com/peter-trerotola/go-postgres-mcp/commit/ab4badac8e9b6e6931a0e2c9c3363ab461a979f4))
* address PR review feedback ([116e087](https://github.com/peter-trerotola/go-postgres-mcp/commit/116e087fa3491873376f6a1fe133a2979a4d6695))
* address PR review feedback (round 2) ([1833ef9](https://github.com/peter-trerotola/go-postgres-mcp/commit/1833ef934c18167f6bd797fcfe58ffeb3304d07b))
* auto-trigger CI on release-please PRs ([#32](https://github.com/peter-trerotola/go-postgres-mcp/issues/32)) ([6e0d526](https://github.com/peter-trerotola/go-postgres-mcp/commit/6e0d526643ddbf043c843559dbc8906dfaf83709))
* auto-trigger release workflow on PR merge ([#30](https://github.com/peter-trerotola/go-postgres-mcp/issues/30)) ([4a62f1e](https://github.com/peter-trerotola/go-postgres-mcp/commit/4a62f1e80b85ea8c73263a5b12bf0f6eb62ff4c6))
* avoid fromJSON crash when release-please pr output is empty ([#37](https://github.com/peter-trerotola/go-postgres-mcp/issues/37)) ([e483070](https://github.com/peter-trerotola/go-postgres-mcp/commit/e4830701e7ba8c158433cc0445c516fb999237ab))
* create release with assets in single step for immutable releases ([#39](https://github.com/peter-trerotola/go-postgres-mcp/issues/39)) ([f13d312](https://github.com/peter-trerotola/go-postgres-mcp/commit/f13d312d142387d74b15a26ba183217f4c0d8d08))
* delete and recreate release to work with immutable releases ([#41](https://github.com/peter-trerotola/go-postgres-mcp/issues/41)) ([39224ef](https://github.com/peter-trerotola/go-postgres-mcp/commit/39224ef0f559343d1749d1db27eb0ad7155b9414))
* discovery FK violations and concurrent schema crawling ([190f3c9](https://github.com/peter-trerotola/go-postgres-mcp/commit/190f3c9288e7e0ed2225430d35069349614c5515))
* discovery FK violations and concurrent schema crawling ([be504d7](https://github.com/peter-trerotola/go-postgres-mcp/commit/be504d7ed4d397fb55ab921b08b16135fec0f7f9))
* parse release-please pr output as JSON ([#34](https://github.com/peter-trerotola/go-postgres-mcp/issues/34)) ([1023b7c](https://github.com/peter-trerotola/go-postgres-mcp/commit/1023b7c07fbf18746a50994848df3020cd93c5cf))
* upgrade all dependencies and add macOS builds ([#22](https://github.com/peter-trerotola/go-postgres-mcp/issues/22)) ([563da01](https://github.com/peter-trerotola/go-postgres-mcp/commit/563da01a234d0d06af5d1749757733c4c217fb70))
* upgrade all dependencies and GitHub Actions ([#20](https://github.com/peter-trerotola/go-postgres-mcp/issues/20)) ([1c238f3](https://github.com/peter-trerotola/go-postgres-mcp/commit/1c238f3266deb1e6cf8a8852690d940e92ef80d1))
* use draft releases to support immutable release assets ([#23](https://github.com/peter-trerotola/go-postgres-mcp/issues/23)) ([b8cc3c6](https://github.com/peter-trerotola/go-postgres-mcp/commit/b8cc3c6ce422971bc4da513035c6c0c36857f104))

## [1.3.0](https://github.com/peter-trerotola/go-postgres-mcp/compare/v1.2.0...v1.3.0) (2026-03-08)


### Features

* add automatic releases via release-please ([a7c1d16](https://github.com/peter-trerotola/go-postgres-mcp/commit/a7c1d16585454eab4a8a8c0c43d54400808b3279))
* inject knowledge map context into MCP responses ([88b94c0](https://github.com/peter-trerotola/go-postgres-mcp/commit/88b94c0dfe9499d3bf7592d2eb4b66fbf8eb4349))
* inject knowledge map context into MCP responses ([ea9b8d2](https://github.com/peter-trerotola/go-postgres-mcp/commit/ea9b8d25c50d5f44e1385a3267505e155d0ded31))


### Bug Fixes

* add actions:write permission to trigger-release workflow ([#35](https://github.com/peter-trerotola/go-postgres-mcp/issues/35)) ([d3ad890](https://github.com/peter-trerotola/go-postgres-mcp/commit/d3ad890cb5df7cfcd7e95c2b7f9db9ef668b24dd))
* add workflow_dispatch trigger to release workflow ([#24](https://github.com/peter-trerotola/go-postgres-mcp/issues/24)) ([af2b57c](https://github.com/peter-trerotola/go-postgres-mcp/commit/af2b57c26764c3ba30062b2382e5ede4d96c4758))
* address PR review comments ([2c61301](https://github.com/peter-trerotola/go-postgres-mcp/commit/2c613015628d6a97a72523f1949e3b581734da46))
* address PR review feedback ([50f501c](https://github.com/peter-trerotola/go-postgres-mcp/commit/50f501c187e8ad181a0f68959f4ddb57b9c45965))
* address PR review feedback ([ab4bada](https://github.com/peter-trerotola/go-postgres-mcp/commit/ab4badac8e9b6e6931a0e2c9c3363ab461a979f4))
* address PR review feedback ([116e087](https://github.com/peter-trerotola/go-postgres-mcp/commit/116e087fa3491873376f6a1fe133a2979a4d6695))
* address PR review feedback (round 2) ([1833ef9](https://github.com/peter-trerotola/go-postgres-mcp/commit/1833ef934c18167f6bd797fcfe58ffeb3304d07b))
* auto-trigger CI on release-please PRs ([#32](https://github.com/peter-trerotola/go-postgres-mcp/issues/32)) ([6e0d526](https://github.com/peter-trerotola/go-postgres-mcp/commit/6e0d526643ddbf043c843559dbc8906dfaf83709))
* auto-trigger release workflow on PR merge ([#30](https://github.com/peter-trerotola/go-postgres-mcp/issues/30)) ([4a62f1e](https://github.com/peter-trerotola/go-postgres-mcp/commit/4a62f1e80b85ea8c73263a5b12bf0f6eb62ff4c6))
* avoid fromJSON crash when release-please pr output is empty ([#37](https://github.com/peter-trerotola/go-postgres-mcp/issues/37)) ([e483070](https://github.com/peter-trerotola/go-postgres-mcp/commit/e4830701e7ba8c158433cc0445c516fb999237ab))
* create release with assets in single step for immutable releases ([#39](https://github.com/peter-trerotola/go-postgres-mcp/issues/39)) ([f13d312](https://github.com/peter-trerotola/go-postgres-mcp/commit/f13d312d142387d74b15a26ba183217f4c0d8d08))
* delete and recreate release to work with immutable releases ([#41](https://github.com/peter-trerotola/go-postgres-mcp/issues/41)) ([39224ef](https://github.com/peter-trerotola/go-postgres-mcp/commit/39224ef0f559343d1749d1db27eb0ad7155b9414))
* discovery FK violations and concurrent schema crawling ([190f3c9](https://github.com/peter-trerotola/go-postgres-mcp/commit/190f3c9288e7e0ed2225430d35069349614c5515))
* discovery FK violations and concurrent schema crawling ([be504d7](https://github.com/peter-trerotola/go-postgres-mcp/commit/be504d7ed4d397fb55ab921b08b16135fec0f7f9))
* parse release-please pr output as JSON ([#34](https://github.com/peter-trerotola/go-postgres-mcp/issues/34)) ([1023b7c](https://github.com/peter-trerotola/go-postgres-mcp/commit/1023b7c07fbf18746a50994848df3020cd93c5cf))
* upgrade all dependencies and add macOS builds ([#22](https://github.com/peter-trerotola/go-postgres-mcp/issues/22)) ([563da01](https://github.com/peter-trerotola/go-postgres-mcp/commit/563da01a234d0d06af5d1749757733c4c217fb70))
* upgrade all dependencies and GitHub Actions ([#20](https://github.com/peter-trerotola/go-postgres-mcp/issues/20)) ([1c238f3](https://github.com/peter-trerotola/go-postgres-mcp/commit/1c238f3266deb1e6cf8a8852690d940e92ef80d1))
* use draft releases to support immutable release assets ([#23](https://github.com/peter-trerotola/go-postgres-mcp/issues/23)) ([b8cc3c6](https://github.com/peter-trerotola/go-postgres-mcp/commit/b8cc3c6ce422971bc4da513035c6c0c36857f104))

## [1.2.0](https://github.com/peter-trerotola/go-postgres-mcp/compare/v1.1.4...v1.2.0) (2026-03-08)


### Features

* add automatic releases via release-please ([a7c1d16](https://github.com/peter-trerotola/go-postgres-mcp/commit/a7c1d16585454eab4a8a8c0c43d54400808b3279))
* inject knowledge map context into MCP responses ([88b94c0](https://github.com/peter-trerotola/go-postgres-mcp/commit/88b94c0dfe9499d3bf7592d2eb4b66fbf8eb4349))
* inject knowledge map context into MCP responses ([ea9b8d2](https://github.com/peter-trerotola/go-postgres-mcp/commit/ea9b8d25c50d5f44e1385a3267505e155d0ded31))


### Bug Fixes

* add actions:write permission to trigger-release workflow ([#35](https://github.com/peter-trerotola/go-postgres-mcp/issues/35)) ([d3ad890](https://github.com/peter-trerotola/go-postgres-mcp/commit/d3ad890cb5df7cfcd7e95c2b7f9db9ef668b24dd))
* add workflow_dispatch trigger to release workflow ([#24](https://github.com/peter-trerotola/go-postgres-mcp/issues/24)) ([af2b57c](https://github.com/peter-trerotola/go-postgres-mcp/commit/af2b57c26764c3ba30062b2382e5ede4d96c4758))
* address PR review comments ([2c61301](https://github.com/peter-trerotola/go-postgres-mcp/commit/2c613015628d6a97a72523f1949e3b581734da46))
* address PR review feedback ([50f501c](https://github.com/peter-trerotola/go-postgres-mcp/commit/50f501c187e8ad181a0f68959f4ddb57b9c45965))
* address PR review feedback ([ab4bada](https://github.com/peter-trerotola/go-postgres-mcp/commit/ab4badac8e9b6e6931a0e2c9c3363ab461a979f4))
* address PR review feedback ([116e087](https://github.com/peter-trerotola/go-postgres-mcp/commit/116e087fa3491873376f6a1fe133a2979a4d6695))
* address PR review feedback (round 2) ([1833ef9](https://github.com/peter-trerotola/go-postgres-mcp/commit/1833ef934c18167f6bd797fcfe58ffeb3304d07b))
* auto-trigger CI on release-please PRs ([#32](https://github.com/peter-trerotola/go-postgres-mcp/issues/32)) ([6e0d526](https://github.com/peter-trerotola/go-postgres-mcp/commit/6e0d526643ddbf043c843559dbc8906dfaf83709))
* auto-trigger release workflow on PR merge ([#30](https://github.com/peter-trerotola/go-postgres-mcp/issues/30)) ([4a62f1e](https://github.com/peter-trerotola/go-postgres-mcp/commit/4a62f1e80b85ea8c73263a5b12bf0f6eb62ff4c6))
* avoid fromJSON crash when release-please pr output is empty ([#37](https://github.com/peter-trerotola/go-postgres-mcp/issues/37)) ([e483070](https://github.com/peter-trerotola/go-postgres-mcp/commit/e4830701e7ba8c158433cc0445c516fb999237ab))
* discovery FK violations and concurrent schema crawling ([190f3c9](https://github.com/peter-trerotola/go-postgres-mcp/commit/190f3c9288e7e0ed2225430d35069349614c5515))
* discovery FK violations and concurrent schema crawling ([be504d7](https://github.com/peter-trerotola/go-postgres-mcp/commit/be504d7ed4d397fb55ab921b08b16135fec0f7f9))
* parse release-please pr output as JSON ([#34](https://github.com/peter-trerotola/go-postgres-mcp/issues/34)) ([1023b7c](https://github.com/peter-trerotola/go-postgres-mcp/commit/1023b7c07fbf18746a50994848df3020cd93c5cf))
* upgrade all dependencies and add macOS builds ([#22](https://github.com/peter-trerotola/go-postgres-mcp/issues/22)) ([563da01](https://github.com/peter-trerotola/go-postgres-mcp/commit/563da01a234d0d06af5d1749757733c4c217fb70))
* upgrade all dependencies and GitHub Actions ([#20](https://github.com/peter-trerotola/go-postgres-mcp/issues/20)) ([1c238f3](https://github.com/peter-trerotola/go-postgres-mcp/commit/1c238f3266deb1e6cf8a8852690d940e92ef80d1))
* use draft releases to support immutable release assets ([#23](https://github.com/peter-trerotola/go-postgres-mcp/issues/23)) ([b8cc3c6](https://github.com/peter-trerotola/go-postgres-mcp/commit/b8cc3c6ce422971bc4da513035c6c0c36857f104))

## [1.1.4](https://github.com/peter-trerotola/go-postgres-mcp/compare/v1.1.3...v1.1.4) (2026-03-08)


### Bug Fixes

* add actions:write permission to trigger-release workflow ([#35](https://github.com/peter-trerotola/go-postgres-mcp/issues/35)) ([d3ad890](https://github.com/peter-trerotola/go-postgres-mcp/commit/d3ad890cb5df7cfcd7e95c2b7f9db9ef668b24dd))
* auto-trigger CI on release-please PRs ([#32](https://github.com/peter-trerotola/go-postgres-mcp/issues/32)) ([6e0d526](https://github.com/peter-trerotola/go-postgres-mcp/commit/6e0d526643ddbf043c843559dbc8906dfaf83709))
* parse release-please pr output as JSON ([#34](https://github.com/peter-trerotola/go-postgres-mcp/issues/34)) ([1023b7c](https://github.com/peter-trerotola/go-postgres-mcp/commit/1023b7c07fbf18746a50994848df3020cd93c5cf))

## [1.1.3](https://github.com/peter-trerotola/go-postgres-mcp/compare/v1.1.2...v1.1.3) (2026-03-08)


### Bug Fixes

* auto-trigger release workflow on PR merge ([#30](https://github.com/peter-trerotola/go-postgres-mcp/issues/30)) ([4a62f1e](https://github.com/peter-trerotola/go-postgres-mcp/commit/4a62f1e80b85ea8c73263a5b12bf0f6eb62ff4c6))

## [1.1.2](https://github.com/peter-trerotola/go-postgres-mcp/compare/v1.1.1...v1.1.2) (2026-03-07)


### Bug Fixes

* add workflow_dispatch trigger to release workflow ([#24](https://github.com/peter-trerotola/go-postgres-mcp/issues/24)) ([af2b57c](https://github.com/peter-trerotola/go-postgres-mcp/commit/af2b57c26764c3ba30062b2382e5ede4d96c4758))
* use draft releases to support immutable release assets ([#23](https://github.com/peter-trerotola/go-postgres-mcp/issues/23)) ([b8cc3c6](https://github.com/peter-trerotola/go-postgres-mcp/commit/b8cc3c6ce422971bc4da513035c6c0c36857f104))

## [1.1.1](https://github.com/peter-trerotola/go-postgres-mcp/compare/v1.1.0...v1.1.1) (2026-03-07)


### Bug Fixes

* upgrade all dependencies and add macOS builds ([#22](https://github.com/peter-trerotola/go-postgres-mcp/issues/22)) ([563da01](https://github.com/peter-trerotola/go-postgres-mcp/commit/563da01a234d0d06af5d1749757733c4c217fb70))
* upgrade all dependencies and GitHub Actions ([#20](https://github.com/peter-trerotola/go-postgres-mcp/issues/20)) ([1c238f3](https://github.com/peter-trerotola/go-postgres-mcp/commit/1c238f3266deb1e6cf8a8852690d940e92ef80d1))

## [1.1.0](https://github.com/peter-trerotola/go-postgres-mcp/compare/v1.0.1...v1.1.0) (2026-03-07)


### Features

* inject knowledge map context into MCP responses ([88b94c0](https://github.com/peter-trerotola/go-postgres-mcp/commit/88b94c0dfe9499d3bf7592d2eb4b66fbf8eb4349))


### Bug Fixes

* address PR review feedback ([ab4bada](https://github.com/peter-trerotola/go-postgres-mcp/commit/ab4badac8e9b6e6931a0e2c9c3363ab461a979f4))

## [1.0.1](https://github.com/peter-trerotola/go-postgres-mcp/compare/v1.0.0...v1.0.1) (2026-03-07)


### Bug Fixes

* address PR review feedback ([116e087](https://github.com/peter-trerotola/go-postgres-mcp/commit/116e087fa3491873376f6a1fe133a2979a4d6695))
* address PR review feedback (round 2) ([1833ef9](https://github.com/peter-trerotola/go-postgres-mcp/commit/1833ef934c18167f6bd797fcfe58ffeb3304d07b))
* discovery FK violations and concurrent schema crawling ([190f3c9](https://github.com/peter-trerotola/go-postgres-mcp/commit/190f3c9288e7e0ed2225430d35069349614c5515))
* discovery FK violations and concurrent schema crawling ([be504d7](https://github.com/peter-trerotola/go-postgres-mcp/commit/be504d7ed4d397fb55ab921b08b16135fec0f7f9))

## 1.0.0 (2026-03-07)


### Features

* add automatic releases via release-please ([a7c1d16](https://github.com/peter-trerotola/go-postgres-mcp/commit/a7c1d16585454eab4a8a8c0c43d54400808b3279))


### Bug Fixes

* address PR review comments ([2c61301](https://github.com/peter-trerotola/go-postgres-mcp/commit/2c613015628d6a97a72523f1949e3b581734da46))
