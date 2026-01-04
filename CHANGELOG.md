# Changelog

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
