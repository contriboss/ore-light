# Changelog

## Unreleased

### Bug Fixes

* Fixed relative path resolution for "path" gems in the lockfile to match Bundler's behavior. Relative paths are now correctly resolved relative to the directory containing the lockfile, rather than the current working directory, in both `install` and `check` commands.
* 🐛 relative path resolution for "path" gems in lockfile ([84ebdc1](https://github.com/contriboss/ore-light/commit/84ebdc192bad7e2699981fd5ed4381ed42b8c568))
* 🐛 relative path resolution for "path" gems in lockfile ([d8f5cc2](https://github.com/contriboss/ore-light/commit/d8f5cc2a30ab70e4d52655abdcdb11f66bf03625))


## [0.25.2](https://github.com/contriboss/ore-light/compare/v0.25.1...v0.25.2) (2026-02-10)


### Bug Fixes

* remove incorrect rubyScope path duplication in git/path gem installation ([e91846b](https://github.com/contriboss/ore-light/commit/e91846b4bc5cee2a9796826c941cd96a22a048cb))

## [0.25.1](https://github.com/contriboss/ore-light/compare/v0.25.1...v0.25.1) (2026-02-10)


### ⚠ BREAKING CHANGES

* drop Windows support and implement XDG Base Directory Specification ([#19](https://github.com/contriboss/ore-light/issues/19))

### Features

* ⬆️ Consistent support for `-gemfile` (and `-lockfile`) ([b080b01](https://github.com/contriboss/ore-light/commit/b080b01818c63f4de78f9f4b0fbf835153613d09))
* ⬆️ Support `-gemfile` (and `-lockfile` where applicable) flag across all relevant commands ([db6a56f](https://github.com/contriboss/ore-light/commit/db6a56f607ec05d3b9c20b68599d16ebce1d0f18))
* ⬆️ Support `gems.rb` =&gt; `gems.locked` ([14de047](https://github.com/contriboss/ore-light/commit/14de0479787a5a53671d8e7a4a52e1748bc1fd17))
* ⬆️ Support `gems.rb` =&gt; `gems.locked` ([33b52ee](https://github.com/contriboss/ore-light/commit/33b52ee061663a3190b6293a8adb4236e215deb1))
* ⬆️ Upgrade to gemfile-go v0.10.0 w/ single group/platform support ([77f7c9f](https://github.com/contriboss/ore-light/commit/77f7c9f29ce5c9a790edf6e1d338116be6d53081))
* ⬆️ Upgrade to gemfile-go v0.10.0 w/ single group/platform support ([3bbdd6e](https://github.com/contriboss/ore-light/commit/3bbdd6ea62fbce0a30edd446198f7699c4405916))
* ⬆️ Upgrade to gemfile-go v0.9.0 w/ hash rocket support ([bfaf3f5](https://github.com/contriboss/ore-light/commit/bfaf3f51e0afd598d1e15cc30905cced77f000e3))
* ⬆️ Upgrade to gemfile-go v0.9.0 w/ hash rocket support ([86ce552](https://github.com/contriboss/ore-light/commit/86ce5520fe587387a226e730c0515c015ea87355))
* Add BUNDLE_GEMFILE environment variable support ([#56](https://github.com/contriboss/ore-light/issues/56)) ([c69f8ed](https://github.com/contriboss/ore-light/commit/c69f8edd19586b27577362410eb5e57cac5bd69a))
* add compact index support with cache freshness optimization ([dba03fb](https://github.com/contriboss/ore-light/commit/dba03fb87e1ef464f9b5fb9aa2678b1e95b173fe))
* Add gem source fallback mechanism for v0.1.1 ([df0af4f](https://github.com/contriboss/ore-light/commit/df0af4f2eefec288ecb605dbfa189cca26b536ac))
* add gemspec directive support ([378d6b3](https://github.com/contriboss/ore-light/commit/378d6b3d41bb3f9fdb475fcf2ebd99150e90aa60))
* add interactive TUI for outdated command with performance improvements ([7705ed3](https://github.com/contriboss/ore-light/commit/7705ed342dc3ae68991730c9e51f2165f99b0e00))
* Add ore audit licenses command ([02c21bd](https://github.com/contriboss/ore-light/commit/02c21bdf1cbb347496468e4afa7e47fc6a27b419))
* Add ore browse command ([59270ec](https://github.com/contriboss/ore-light/commit/59270ecf2ef429f174b852c3dfdc21da39b94584))
* Add ore gems command ([0812ecf](https://github.com/contriboss/ore-light/commit/0812ecfa2d46e8b9ac2d8ea1ad6dc3466d75644c))
* Add ore open command ([d38ed8d](https://github.com/contriboss/ore-light/commit/d38ed8d5d7e0b95bdcedb129aa812d4d8edd523a))
* Add ore pristine command ([43caf97](https://github.com/contriboss/ore-light/commit/43caf97598a4de3cacd65135a994dc08c7edc491))
* Add ore search command ([a57dc88](https://github.com/contriboss/ore-light/commit/a57dc88595a8e4e910282a314b998f785d848538))
* Add ore stats command ([f983553](https://github.com/contriboss/ore-light/commit/f983553723623a10a4be7371c653d319d8304c41))
* Add ore why command ([271ce3a](https://github.com/contriboss/ore-light/commit/271ce3aa2467514b1b208f7580d802106088be0d))
* add self-update command ([9077b93](https://github.com/contriboss/ore-light/commit/9077b93c923aa5581db7a8a52c04c13bc4969ea9))
* add structured logging with slog ([46b29b8](https://github.com/contriboss/ore-light/commit/46b29b847013680a357fb9588941e834e651e1a1))
* Add tests & documentation ([5d9978f](https://github.com/contriboss/ore-light/commit/5d9978fe1f6f53256a168ae9fd4b332d91a4f578))
* Default mage install to ~/.local/bin for better UX ([d7d7c73](https://github.com/contriboss/ore-light/commit/d7d7c732bed5880d961a9964c279ea8dc6c698bc))
* Display post-install messages after gem installation ([e4e3288](https://github.com/contriboss/ore-light/commit/e4e328883730d1774fe3dd36d28483b240b51c09))
* drop Windows support and implement XDG Base Directory Specification ([#19](https://github.com/contriboss/ore-light/issues/19)) ([a713d55](https://github.com/contriboss/ore-light/commit/a713d55425bb34e9e2cf7b55927f0e3c729944c8))
* enhance browse command with summaries and detail view ([adf7acf](https://github.com/contriboss/ore-light/commit/adf7acf0915b4f3bb964ad08df2c046b3659e97c))
* export default command wrappers via internal/runtime package ([#29](https://github.com/contriboss/ore-light/issues/29)) ([d5df403](https://github.com/contriboss/ore-light/commit/d5df40350d69e14f117d788300d50e85520971f1))
* generate lockfile when needed ([#58](https://github.com/contriboss/ore-light/issues/58)) ([2e105c3](https://github.com/contriboss/ore-light/commit/2e105c3280504f47d93263f41e6eb82f0d939ae6))
* improve lock mechanism to work with bundler 4 ([#42](https://github.com/contriboss/ore-light/issues/42)) ([a4bacdf](https://github.com/contriboss/ore-light/commit/a4bacdf328abbc463f927489b17f30e236908034))
* migrate from tinyrange/pubgrub to pubgrub-go ([b61091c](https://github.com/contriboss/ore-light/commit/b61091c55ef6fa532b6175ff3fd79f76a114f018))
* refactor all command handlers into commands package ([#27](https://github.com/contriboss/ore-light/issues/27)) ([b13dce9](https://github.com/contriboss/ore-light/commit/b13dce964fca6e962f28c915af4a794de36f15db))
* replace exec git commands with go-git library for shallow clone support ([134e9a6](https://github.com/contriboss/ore-light/commit/134e9a611641ca567bbd1e9ad8a52cf7477a9b2e))
* replace go-github-selfupdate with contriboss/go-update ([#54](https://github.com/contriboss/ore-light/issues/54)) ([aaf70a1](https://github.com/contriboss/ore-light/commit/aaf70a1628b4e64d1262c45d1b070ea70fb9d2be))
* Resolve all TODO items in codebase ([0e5b785](https://github.com/contriboss/ore-light/commit/0e5b785b0c6f9a39a0f6962ab8b869eadfb7f3b3))
* **sources:** add mirrors and conditional downloads ([b6116da](https://github.com/contriboss/ore-light/commit/b6116da233ff46e715d809b2bf4b61d94b9ee018))
* unify vendor dir detection with BUNDLE_PATH support ([6704748](https://github.com/contriboss/ore-light/commit/67047486b67df006a75bed332d2ac467b0e67cc7))
* update gemlock format and fix scope ([#35](https://github.com/contriboss/ore-light/issues/35)) ([8af7c55](https://github.com/contriboss/ore-light/commit/8af7c5570b5791b3897a4fbf281e761486c856e4))
* upgrade to ruby-extension-go v0.1.0 with tool checking ([651ae6d](https://github.com/contriboss/ore-light/commit/651ae6defef5dfce0e26d1b52158b33123bbce4d))
* use global gems by default like Bundler (no Ruby required) ([a93795a](https://github.com/contriboss/ore-light/commit/a93795a707f3982fb3a50cf43da197c11ad6232f))


### Bug Fixes

* 🐛 debugging ([fcc7f43](https://github.com/contriboss/ore-light/commit/fcc7f43baef5a76b5254f366fd42025968d833cd))
* 🐛 Keep platform aligned with lockfile during gemspec generation ([260d498](https://github.com/contriboss/ore-light/commit/260d498656965baef533a37a09e97804b9ee7936))
* 🐛 make ore binstubs Ruby and bundle-aware ([bb93707](https://github.com/contriboss/ore-light/commit/bb937071134d6c72b6e79bbb7c789a0c9ebd963b))
* 🐛 Prevent default_executable from polluting gemspecs ([d9f4084](https://github.com/contriboss/ore-light/commit/d9f40844cb676a1653ecec424d4f91c45a009d2b))
* 🐛 relative path resolution for "path" gems in lockfile ([84ebdc1](https://github.com/contriboss/ore-light/commit/84ebdc192bad7e2699981fd5ed4381ed42b8c568))
* 🐛 relative path resolution for "path" gems in lockfile ([d8f5cc2](https://github.com/contriboss/ore-light/commit/d8f5cc2a30ab70e4d52655abdcdb11f66bf03625))
* 🐛 Treat BUNDLE_PATH gem homes as fully scoped ([d16d452](https://github.com/contriboss/ore-light/commit/d16d452e83299caa85c0d3a3bd67e42293f8a468))
* 🐛 validate gemspec stub platform during install ([fe69fcd](https://github.com/contriboss/ore-light/commit/fe69fcd57b00769dc053b0b1aa96ecbf7d94d47a))
* 🐛 wrap ore binstubs with sh exec ([9516df1](https://github.com/contriboss/ore-light/commit/9516df12511cb462ab2e9b16dbc21571f1b847af))
* Add Bundler-compatible installation paths and config command ([3647131](https://github.com/contriboss/ore-light/commit/36471314a7cffc7b4a1557d5ec7c63398c67af1e))
* Add missing go.sum entries for bubbles dependencies ([eb653cc](https://github.com/contriboss/ore-light/commit/eb653cc46dbb3617fce70d554b8446392462fdd8))
* avoid mutation of gnu/musl ([be45de5](https://github.com/contriboss/ore-light/commit/be45de53eb08c8548c0566486bdbfcb40771f4fe))
* bug in github fetch and improve downloader. ([#34](https://github.com/contriboss/ore-light/issues/34)) ([2800cf4](https://github.com/contriboss/ore-light/commit/2800cf4210f310290f0b52248079b3cdb35d00b9))
* correct ore exec description - requires bundle exec ([5595f66](https://github.com/contriboss/ore-light/commit/5595f66f55dc47da9abc3d01a2254e0927fd497f))
* detect gem paths for mise/asdf/rbenv version managers ([354afb1](https://github.com/contriboss/ore-light/commit/354afb14cea325682455a1649ee23bc36b9f3bd5))
* embed release builds directly in release-please workflow ([3c8dcef](https://github.com/contriboss/ore-light/commit/3c8dcef00b68d8c87e17e7abdc5b503758915141))
* exit non-zero on extension failures and support version-constrained build deps ([#39](https://github.com/contriboss/ore-light/issues/39)) ([218e976](https://github.com/contriboss/ore-light/commit/218e976e137972fe24b6502738af656c20ca62ab))
* Extension Building Logic ([883ebab](https://github.com/contriboss/ore-light/commit/883ebabc38a6faca7997c3301f53e77ac910d262))
* fix detection version in docker ([ee583d7](https://github.com/contriboss/ore-light/commit/ee583d71f74a5fb7a27e555429da14380c32ee09))
* increase download timeout to 2.5 minutes for large gems ([00e77c1](https://github.com/contriboss/ore-light/commit/00e77c1d67151bd5d7c26cb110ef13d3e8f156a5))
* Linux libc variant (gnu/musl) - platform selection ([#49](https://github.com/contriboss/ore-light/issues/49)) ([6c950bf](https://github.com/contriboss/ore-light/commit/6c950bfc29c7ce6ade5d46e7b8742ec0dafcfbf5))
* match bundler 4 structure ([#46](https://github.com/contriboss/ore-light/issues/46)) ([cef507d](https://github.com/contriboss/ore-light/commit/cef507db2321af2fe315c3ffa52bae8bd452d339))
* mirror bundler install detection for native gems ([#53](https://github.com/contriboss/ore-light/issues/53)) ([434a1d0](https://github.com/contriboss/ore-light/commit/434a1d0a4435043252128f3678f9d38f5933a0a1))
* ore -v shows detected Ruby version and arch instead of hardcoded default ([237e9af](https://github.com/contriboss/ore-light/commit/237e9af82480493160c965eaeafe4ef48464db81))
* ore install not generating lockfile with BUNDLE_GEMFILE ([#60](https://github.com/contriboss/ore-light/issues/60)) ([eb15ffc](https://github.com/contriboss/ore-light/commit/eb15ffcf7f5f10cc2baa6140d22c2d47b4e9de5b))
* prevent path gem infinite recursion and add gem environment for extensions ([742a745](https://github.com/contriboss/ore-light/commit/742a745de257388a6d2750b37d68eaf41a024445))
* re-export default command wrappers from commands package to avoid internal/ import restrictions ([1f8c117](https://github.com/contriboss/ore-light/commit/1f8c117e7884ae72aafa206d8441c51f8897e3d8))
* relax Go version requirement to 1.25 ([86738e1](https://github.com/contriboss/ore-light/commit/86738e1f3f695102dc86c652bcdc0bb770243cf0))
* Respect BUNDLE_GEMFILE when resolving lockfile path ([acb03b5](https://github.com/contriboss/ore-light/commit/acb03b546f5c5d987e25929cd50c313213a1909e))
* respect configured Gemfile sources in resolver ([b8fa602](https://github.com/contriboss/ore-light/commit/b8fa60213377dff0e4d2ce6e6736f4b83cb2cd73))
* skip extension building for precompiled platform-specific gems ([b6575c4](https://github.com/contriboss/ore-light/commit/b6575c465c743e8d55bdc972fab7ecebd6d0c748))
* trigger release ([5690de3](https://github.com/contriboss/ore-light/commit/5690de3364035dbcc34a142ccebf67e0dba6c2cd))
* trigger Release workflow after release-please creates release ([6f8f612](https://github.com/contriboss/ore-light/commit/6f8f6122642ac976f7df633cc9d6b4a0cc9cd36b))
* trigger release workflow on release events ([c407dec](https://github.com/contriboss/ore-light/commit/c407dec81ccc39212ec069f6e0686bdd5de37005))
* unify config resolution and CLI help ([#32](https://github.com/contriboss/ore-light/issues/32)) ([5f93327](https://github.com/contriboss/ore-light/commit/5f9332765ea8d6d8f795fcec324f100ef82e9d5d))
* update syntax ([807b983](https://github.com/contriboss/ore-light/commit/807b983ae7a26830e11b540b1f8d6a87821d8a91))
* use -mod=mod for test commands ([7cc1517](https://github.com/contriboss/ore-light/commit/7cc1517ef4d1d25c7310009203618897b0e2d98d))
* use gemspec extensions metadata for build detection ([7be4fae](https://github.com/contriboss/ore-light/commit/7be4fae34c725c4f49c80fe94761322b60303f71))


### Miscellaneous Chores

* release 0.25.1 ([e4ea5f2](https://github.com/contriboss/ore-light/commit/e4ea5f2477faa681b83332088676b85799eb3b4e))

## [0.25.0](https://github.com/contriboss/ore-light/compare/v0.24.0...v0.25.0) (2026-02-08)

### Features

* ⬆️ Support `gems.rb` =&gt; `gems.locked` ([14de047](https://github.com/contriboss/ore-light/commit/14de0479787a5a53671d8e7a4a52e1748bc1fd17))
* ⬆️ Support `gems.rb` =&gt; `gems.locked` ([33b52ee](https://github.com/contriboss/ore-light/commit/33b52ee061663a3190b6293a8adb4236e215deb1))
* Centralized and hardened lockfile resolution logic into reusable helpers (`resolveLockfilePath`, `resolveLockfilePathWithDerivedFallback`).
* Read-only commands now fail fast with a clear error if the required lockfile is missing.
* Expanded and strengthened the regression test suite to cover `gems.rb` and `gems.locked` interactions with more robust assertions.

### Bug Fixes

* Fixed lockfile derivation logic to correctly support Bundler's `gems.rb` -> `gems.locked` convention across all commands.
* Resolved issues where commands would incorrectly search for `gems.rb.lock` or `Gemfile.lock.lock`.
* Tightened flag parsing and error handling in `outdated` command tests to prevent silent regression skips.

## [0.24.0](https://github.com/contriboss/ore-light/compare/v0.23.0...v0.24.0) (2026-02-08)


### Features

* ⬆️ Consistent support for `-gemfile` (and `-lockfile`) ([b080b01](https://github.com/contriboss/ore-light/commit/b080b01818c63f4de78f9f4b0fbf835153613d09))
* ⬆️ Support `-gemfile` (and `-lockfile` where applicable) flag across all relevant commands ([db6a56f](https://github.com/contriboss/ore-light/commit/db6a56f607ec05d3b9c20b68599d16ebce1d0f18))

## [0.23.0](https://github.com/contriboss/ore-light/compare/v0.22.0...v0.23.0) (2026-02-08)


### Features

* Support `-gemfile` (and `-lockfile` where applicable) flag across all relevant commands: `install`, `add`, `remove`, `update`, `lock`, `check`, `outdated`, `exec`, `audit`, `tree`, `clean`, `show`, and `platform`.
* ⬆️ Upgrade to gemfile-go v0.10.0 w/ single group/platform support ([77f7c9f](https://github.com/contriboss/ore-light/commit/77f7c9f29ce5c9a790edf6e1d338116be6d53081))
* ⬆️ Upgrade to gemfile-go v0.10.0 w/ single group/platform support ([3bbdd6e](https://github.com/contriboss/ore-light/commit/3bbdd6ea62fbce0a30edd446198f7699c4405916))

## [0.22.0](https://github.com/contriboss/ore-light/compare/v0.21.0...v0.22.0) (2026-02-08)


### Features

* ⬆️ Upgrade to gemfile-go v0.9.0 w/ hash rocket support ([bfaf3f5](https://github.com/contriboss/ore-light/commit/bfaf3f51e0afd598d1e15cc30905cced77f000e3))
* ⬆️ Upgrade to gemfile-go v0.9.0 w/ hash rocket support ([86ce552](https://github.com/contriboss/ore-light/commit/86ce5520fe587387a226e730c0515c015ea87355))

## [0.21.0](https://github.com/contriboss/ore-light/compare/v0.20.2...v0.21.0) (2026-01-29)


### Features

* Add tests & documentation ([5d9978f](https://github.com/contriboss/ore-light/commit/5d9978fe1f6f53256a168ae9fd4b332d91a4f578))


### Bug Fixes

* 🐛 debugging ([fcc7f43](https://github.com/contriboss/ore-light/commit/fcc7f43baef5a76b5254f366fd42025968d833cd))
* 🐛 Keep platform aligned with lockfile during gemspec generation ([260d498](https://github.com/contriboss/ore-light/commit/260d498656965baef533a37a09e97804b9ee7936))
* 🐛 make ore binstubs Ruby and bundle-aware ([bb93707](https://github.com/contriboss/ore-light/commit/bb937071134d6c72b6e79bbb7c789a0c9ebd963b))
* 🐛 Treat BUNDLE_PATH gem homes as fully scoped ([d16d452](https://github.com/contriboss/ore-light/commit/d16d452e83299caa85c0d3a3bd67e42293f8a468))
* 🐛 validate gemspec stub platform during install ([fe69fcd](https://github.com/contriboss/ore-light/commit/fe69fcd57b00769dc053b0b1aa96ecbf7d94d47a))
* 🐛 wrap ore binstubs with sh exec ([9516df1](https://github.com/contriboss/ore-light/commit/9516df12511cb462ab2e9b16dbc21571f1b847af))

## [0.20.2](https://github.com/contriboss/ore-light/compare/v0.20.1...v0.20.2) (2026-01-28)


### Bug Fixes

* 🐛 Prevent default_executable from polluting gemspecs ([d9f4084](https://github.com/contriboss/ore-light/commit/d9f40844cb676a1653ecec424d4f91c45a009d2b))

## [0.20.1](https://github.com/contriboss/ore-light/compare/v0.20.0...v0.20.1) (2026-01-24)


### Bug Fixes

* Extension Building Logic ([883ebab](https://github.com/contriboss/ore-light/commit/883ebabc38a6faca7997c3301f53e77ac910d262))
* Respect BUNDLE_GEMFILE when resolving lockfile path ([acb03b5](https://github.com/contriboss/ore-light/commit/acb03b546f5c5d987e25929cd50c313213a1909e))
* use -mod=mod for test commands ([7cc1517](https://github.com/contriboss/ore-light/commit/7cc1517ef4d1d25c7310009203618897b0e2d98d))

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
