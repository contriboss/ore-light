# Changelog

## [0.20.0](https://github.com/contriboss/ore-light/compare/v0.19.0...v0.20.0) (2026-01-23)


### Features

* generate lockfile when needed ([#58](https://github.com/contriboss/ore-light/issues/58)) ([2e105c3](https://github.com/contriboss/ore-light/commit/2e105c3280504f47d93263f41e6eb82f0d939ae6))


### Bug Fixes

* ore -v shows detected Ruby version and arch instead of hardcoded default ([237e9af](https://github.com/contriboss/ore-light/commit/237e9af82480493160c965eaeafe4ef48464db81))
* ore install not generating lockfile with BUNDLE_GEMFILE ([#60](https://github.com/contriboss/ore-light/issues/60)) ([eb15ffc](https://github.com/contriboss/ore-light/commit/eb15ffcf7f5f10cc2baa6140d22c2d47b4e9de5b))
* skip extension building for precompiled platform-specific gems ([b6575c4](https://github.com/contriboss/ore-light/commit/b6575c465c743e8d55bdc972fab7ecebd6d0c748))
* use gemspec extensions metadata for build detection ([7be4fae](https://github.com/contriboss/ore-light/commit/7be4fae34c725c4f49c80fe94761322b60303f71))

## [0.19.0](https://github.com/contriboss/ore-light/compare/v0.18.0...v0.19.0) (2026-01-20)


### Features

* Add BUNDLE_GEMFILE environment variable support ([#56](https://github.com/contriboss/ore-light/issues/56)) ([c69f8ed](https://github.com/contriboss/ore-light/commit/c69f8edd19586b27577362410eb5e57cac5bd69a))

## [0.18.0](https://github.com/contriboss/ore-light/compare/v0.17.3...v0.18.0) (2026-01-20)


### Features

* replace go-github-selfupdate with contriboss/go-update ([#54](https://github.com/contriboss/ore-light/issues/54)) ([aaf70a1](https://github.com/contriboss/ore-light/commit/aaf70a1628b4e64d1262c45d1b070ea70fb9d2be))


### Bug Fixes

* Linux libc variant (gnu/musl) - platform selection ([#49](https://github.com/contriboss/ore-light/issues/49)) ([6c950bf](https://github.com/contriboss/ore-light/commit/6c950bfc29c7ce6ade5d46e7b8742ec0dafcfbf5))
* mirror bundler install detection for native gems ([#53](https://github.com/contriboss/ore-light/issues/53)) ([434a1d0](https://github.com/contriboss/ore-light/commit/434a1d0a4435043252128f3678f9d38f5933a0a1))

## [0.17.3](https://github.com/contriboss/ore-light/compare/v0.17.2...v0.17.3) (2026-01-15)


### Bug Fixes

* match bundler 4 structure ([#46](https://github.com/contriboss/ore-light/issues/46)) ([cef507d](https://github.com/contriboss/ore-light/commit/cef507db2321af2fe315c3ffa52bae8bd452d339))

## [0.17.2](https://github.com/contriboss/ore-light/compare/v0.17.1...v0.17.2) (2026-01-13)


### Bug Fixes

* avoid mutation of gnu/musl ([be45de5](https://github.com/contriboss/ore-light/commit/be45de53eb08c8548c0566486bdbfcb40771f4fe))

## [0.17.1](https://github.com/contriboss/ore-light/compare/v0.17.0...v0.17.1) (2026-01-13)


### Bug Fixes

* trigger release ([5690de3](https://github.com/contriboss/ore-light/commit/5690de3364035dbcc34a142ccebf67e0dba6c2cd))

## [0.17.0](https://github.com/contriboss/ore-light/compare/v0.16.1...v0.17.0) (2026-01-13)


### Features

* improve lock mechanism to work with bundler 4 ([#42](https://github.com/contriboss/ore-light/issues/42)) ([a4bacdf](https://github.com/contriboss/ore-light/commit/a4bacdf328abbc463f927489b17f30e236908034))

## [0.16.1](https://github.com/contriboss/ore-light/compare/v0.16.0...v0.16.1) (2026-01-05)


### Bug Fixes

* exit non-zero on extension failures and support version-constrained build deps ([#39](https://github.com/contriboss/ore-light/issues/39)) ([218e976](https://github.com/contriboss/ore-light/commit/218e976e137972fe24b6502738af656c20ca62ab))

## [0.16.0](https://github.com/contriboss/ore-light/compare/v0.15.1...v0.16.0) (2026-01-05)


### Features

* replace exec git commands with go-git library for shallow clone support ([134e9a6](https://github.com/contriboss/ore-light/commit/134e9a611641ca567bbd1e9ad8a52cf7477a9b2e))


### Bug Fixes

* increase download timeout to 2.5 minutes for large gems ([00e77c1](https://github.com/contriboss/ore-light/commit/00e77c1d67151bd5d7c26cb110ef13d3e8f156a5))

## [0.15.1](https://github.com/contriboss/ore-light/compare/v0.15.0...v0.15.1) (2026-01-04)


### Bug Fixes

* fix detection version in docker ([ee583d7](https://github.com/contriboss/ore-light/commit/ee583d71f74a5fb7a27e555429da14380c32ee09))

## [0.15.0](https://github.com/contriboss/ore-light/compare/v0.14.2...v0.15.0) (2026-01-04)


### Features

* update gemlock format and fix scope ([#35](https://github.com/contriboss/ore-light/issues/35)) ([8af7c55](https://github.com/contriboss/ore-light/commit/8af7c5570b5791b3897a4fbf281e761486c856e4))

## [0.14.2](https://github.com/contriboss/ore-light/compare/v0.14.1...v0.14.2) (2026-01-04)


### Bug Fixes

* bug in github fetch and improve downloader. ([#34](https://github.com/contriboss/ore-light/issues/34)) ([2800cf4](https://github.com/contriboss/ore-light/commit/2800cf4210f310290f0b52248079b3cdb35d00b9))
* unify config resolution and CLI help ([#32](https://github.com/contriboss/ore-light/issues/32)) ([5f93327](https://github.com/contriboss/ore-light/commit/5f9332765ea8d6d8f795fcec324f100ef82e9d5d))

## [0.14.1](https://github.com/contriboss/ore-light/compare/v0.14.0...v0.14.1) (2026-01-01)


### Bug Fixes

* re-export default command wrappers from commands package to avoid internal/ import restrictions ([1f8c117](https://github.com/contriboss/ore-light/commit/1f8c117e7884ae72aafa206d8441c51f8897e3d8))

## [0.14.0](https://github.com/contriboss/ore-light/compare/v0.13.0...v0.14.0) (2026-01-01)


### Features

* export default command wrappers via internal/runtime package ([#29](https://github.com/contriboss/ore-light/issues/29)) ([d5df403](https://github.com/contriboss/ore-light/commit/d5df40350d69e14f117d788300d50e85520971f1))

## [0.13.0](https://github.com/contriboss/ore-light/compare/v0.12.0...v0.13.0) (2026-01-01)


### Features

* refactor all command handlers into commands package ([#27](https://github.com/contriboss/ore-light/issues/27)) ([b13dce9](https://github.com/contriboss/ore-light/commit/b13dce964fca6e962f28c915af4a794de36f15db))

## [0.12.0](https://github.com/contriboss/ore-light/compare/v0.11.0...v0.12.0) (2025-12-17)


### Features

* **sources:** add mirrors and conditional downloads ([b6116da](https://github.com/contriboss/ore-light/commit/b6116da233ff46e715d809b2bf4b61d94b9ee018))

## [0.11.0](https://github.com/contriboss/ore-light/compare/v0.10.0...v0.11.0) (2025-12-17)


### Features

* unify vendor dir detection with BUNDLE_PATH support ([6704748](https://github.com/contriboss/ore-light/commit/67047486b67df006a75bed332d2ac467b0e67cc7))


### Bug Fixes

* detect gem paths for mise/asdf/rbenv version managers ([354afb1](https://github.com/contriboss/ore-light/commit/354afb14cea325682455a1649ee23bc36b9f3bd5))

## [0.10.0](https://github.com/contriboss/ore-light/compare/v0.9.1...v0.10.0) (2025-11-21)


### ⚠ BREAKING CHANGES

* drop Windows support and implement XDG Base Directory Specification ([#19](https://github.com/contriboss/ore-light/issues/19))

### Features

* drop Windows support and implement XDG Base Directory Specification ([#19](https://github.com/contriboss/ore-light/issues/19)) ([a713d55](https://github.com/contriboss/ore-light/commit/a713d55425bb34e9e2cf7b55927f0e3c729944c8))

## [0.9.1](https://github.com/contriboss/ore-light/compare/v0.9.0...v0.9.1) (2025-11-11)


### Bug Fixes

* relax Go version requirement to 1.25 ([86738e1](https://github.com/contriboss/ore-light/commit/86738e1f3f695102dc86c652bcdc0bb770243cf0))

## [0.9.0](https://github.com/contriboss/ore-light/compare/v0.8.0...v0.9.0) (2025-11-11)


### Features

* add interactive TUI for outdated command with performance improvements ([7705ed3](https://github.com/contriboss/ore-light/commit/7705ed342dc3ae68991730c9e51f2165f99b0e00))
* add structured logging with slog ([46b29b8](https://github.com/contriboss/ore-light/commit/46b29b847013680a357fb9588941e834e651e1a1))

## [0.8.0](https://github.com/contriboss/ore-light/compare/v0.7.3...v0.8.0) (2025-11-01)


### Features

* enhance browse command with summaries and detail view ([adf7acf](https://github.com/contriboss/ore-light/commit/adf7acf0915b4f3bb964ad08df2c046b3659e97c))

## [0.7.3](https://github.com/contriboss/ore-light/compare/v0.7.2...v0.7.3) (2025-11-01)


### Bug Fixes

* embed release builds directly in release-please workflow ([3c8dcef](https://github.com/contriboss/ore-light/commit/3c8dcef00b68d8c87e17e7abdc5b503758915141))

## [0.7.2](https://github.com/contriboss/ore-light/compare/v0.7.1...v0.7.2) (2025-11-01)


### Bug Fixes

* trigger Release workflow after release-please creates release ([6f8f612](https://github.com/contriboss/ore-light/commit/6f8f6122642ac976f7df633cc9d6b4a0cc9cd36b))

## [0.7.1](https://github.com/contriboss/ore-light/compare/v0.7.0...v0.7.1) (2025-11-01)


### Bug Fixes

* trigger release workflow on release events ([c407dec](https://github.com/contriboss/ore-light/commit/c407dec81ccc39212ec069f6e0686bdd5de37005))

## [0.7.0](https://github.com/contriboss/ore-light/compare/v0.6.0...v0.7.0) (2025-11-01)


### Features

* add self-update command ([9077b93](https://github.com/contriboss/ore-light/commit/9077b93c923aa5581db7a8a52c04c13bc4969ea9))


### Bug Fixes

* respect configured Gemfile sources in resolver ([b8fa602](https://github.com/contriboss/ore-light/commit/b8fa60213377dff0e4d2ce6e6736f4b83cb2cd73))

## [0.6.0](https://github.com/contriboss/ore-light/compare/v0.5.1...v0.6.0) (2025-11-01)


### Features

* add compact index support with cache freshness optimization ([dba03fb](https://github.com/contriboss/ore-light/commit/dba03fb87e1ef464f9b5fb9aa2678b1e95b173fe))
