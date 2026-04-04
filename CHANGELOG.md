# Changelog

## [0.6.0](https://github.com/thesimonho/warden/compare/v0.5.2...v0.6.0) (2026-04-04)


### Features

* audit access item CRUD operations ([#60](https://github.com/thesimonho/warden/issues/60)) ([e7ee75d](https://github.com/thesimonho/warden/commit/e7ee75d79e24956a6914cbe6c8b3f5db137af6f6))
* hot-reload allowed domains without container recreate ([#75](https://github.com/thesimonho/warden/issues/75)) ([7542ae6](https://github.com/thesimonho/warden/commit/7542ae64a75e496deda712f2353d2279449c8873))
* image paste/drag-and-drop + remove Podman support ([#67](https://github.com/thesimonho/warden/issues/67)) ([39f8503](https://github.com/thesimonho/warden/commit/39f850331c453d6900422fd3b9dfd5dbb09c182b))
* language runtime declarations for containers ([#76](https://github.com/thesimonho/warden/issues/76)) ([0e1da2a](https://github.com/thesimonho/warden/commit/0e1da2a79979bf59f5d1c840e91cd8294998af80))
* scope restricted network domains by agent type ([#74](https://github.com/thesimonho/warden/issues/74)) ([8c61d5d](https://github.com/thesimonho/warden/commit/8c61d5df3b0e4159efb0a63314e59983f55c6790))
* version check and display at startup ([#66](https://github.com/thesimonho/warden/issues/66)) ([39ff17e](https://github.com/thesimonho/warden/commit/39ff17e7d161b8cc9e6c8209c7bf29d5a6e28e7d))


### Bug Fixes

* clarify container name-taken error message ([#69](https://github.com/thesimonho/warden/issues/69)) ([c866faa](https://github.com/thesimonho/warden/commit/c866faa0d1674043cd7ea75867e46121b3887d1b))
* deduplicate browser notifications ([#71](https://github.com/thesimonho/warden/issues/71)) ([c900f70](https://github.com/thesimonho/warden/commit/c900f7074eebd9677101a0bcda9c2d0b849e1c5d))
* default audit timeline brush to past 7 days ([#72](https://github.com/thesimonho/warden/issues/72)) ([560be3e](https://github.com/thesimonho/warden/commit/560be3ebae62b7cd6b241b20e40d3df47c17f0f0))
* log discarded errors and fix errcheck lint ([#62](https://github.com/thesimonho/warden/issues/62)) ([0f1f039](https://github.com/thesimonho/warden/commit/0f1f039d60e28f16871ba92e1dde9f66c87b11bc))
* prevent stale needs-input state on container start ([#73](https://github.com/thesimonho/warden/issues/73)) ([34525d6](https://github.com/thesimonho/warden/commit/34525d6cd14fe7e7baecbe50b1d63eac796e12b6))
* sanitize worktree names instead of rejecting invalid characters ([#70](https://github.com/thesimonho/warden/issues/70)) ([5d640ec](https://github.com/thesimonho/warden/commit/5d640ecf2280716b922c05ddb81d6608b25574e8))
* synthesize session_start from JSONL for Claude Code ([#68](https://github.com/thesimonho/warden/issues/68)) ([ebaab68](https://github.com/thesimonho/warden/commit/ebaab68f6f7fd53923476f09041b93af9499897e))
* worktree status indicators and state rename ([#64](https://github.com/thesimonho/warden/issues/64)) ([29332e8](https://github.com/thesimonho/warden/commit/29332e8f30e39f79a9836955f81c774fe09f0c59))

## [0.5.2](https://github.com/thesimonho/warden/compare/v0.5.1...v0.5.2) (2026-04-03)


### Bug Fixes

* always show cost dashboard on home page ([#57](https://github.com/thesimonho/warden/issues/57)) ([0a4053e](https://github.com/thesimonho/warden/commit/0a4053eefb6ab0467d27bb1238db90a05de1684c))
* dereference nix symlinks in agent config dirs at startup ([#54](https://github.com/thesimonho/warden/issues/54)) ([ead62d4](https://github.com/thesimonho/warden/commit/ead62d403cba30d4b24275c69785743554d00126))
* E2E agent type matrix and stale container cleanup ([#49](https://github.com/thesimonho/warden/issues/49)) ([43a7667](https://github.com/thesimonho/warden/commit/43a7667686430cc41589f805b7bae11d82f0b274))
* E2E stale project collisions and missing agentType in navigation ([#51](https://github.com/thesimonho/warden/issues/51)) ([c252fd8](https://github.com/thesimonho/warden/commit/c252fd8e07ede84f9f9c48755389fcc3456fabf8))
* prevent dev server conflicts and silent proxy failures ([#55](https://github.com/thesimonho/warden/issues/55)) ([dafe291](https://github.com/thesimonho/warden/commit/dafe291f95152c5e64b989a235a3ec73979652cb))

## [0.5.1](https://github.com/thesimonho/warden/compare/v0.5.0...v0.5.1) (2026-04-03)


### Bug Fixes

* remove unused imports from worktree-list ([#45](https://github.com/thesimonho/warden/issues/45)) ([064b10c](https://github.com/thesimonho/warden/commit/064b10cb21fa7dd10022d3d586a184e27789d206))

## [0.5.0](https://github.com/thesimonho/warden/compare/v0.4.2...v0.5.0) (2026-04-03)


### Features

* multi-agent support with compound primary keys ([#44](https://github.com/thesimonho/warden/issues/44)) ([a2277f8](https://github.com/thesimonho/warden/commit/a2277f8fc1716d2edcc37d7f816435f5ef00d1e1))


### Performance Improvements

* reduce notification latency in hook-to-SSE chain ([8dab3ce](https://github.com/thesimonho/warden/commit/8dab3cedc7097539b0935109960a6072556d4b48))

## [0.4.2](https://github.com/thesimonho/warden/compare/v0.4.1...v0.4.2) (2026-03-29)


### Bug Fixes

* build container from source during release instead of retagging latest ([#37](https://github.com/thesimonho/warden/issues/37)) ([e78f18a](https://github.com/thesimonho/warden/commit/e78f18a1d8dc4abb793382adc6a70a2b914a26f7))
* Windows desktop build failure from goversioninfo relocation error ([#36](https://github.com/thesimonho/warden/issues/36)) ([fe3ba11](https://github.com/thesimonho/warden/commit/fe3ba1130dd2ac4f80224946fd3c15ec980653ef))

## [0.4.1](https://github.com/thesimonho/warden/compare/v0.4.0...v0.4.1) (2026-03-29)


### Bug Fixes

* Windows build failure due to Unix-only syscall.Stat_t ([#34](https://github.com/thesimonho/warden/issues/34)) ([f4a1382](https://github.com/thesimonho/warden/commit/f4a1382ec636fc6bb3f2b009eef5500d4ea6dfe2))

## [0.4.0](https://github.com/thesimonho/warden/compare/v0.3.0...v0.4.0) (2026-03-29)


### Features

* access system — general credential passthrough replacing presets ([#24](https://github.com/thesimonho/warden/issues/24)) ([5b4b87c](https://github.com/thesimonho/warden/commit/5b4b87c491a96ac4a7340df41fe3004b287b42f3))
* container hardening — PidsLimit, gosu entrypoint, host UID passthrough ([e6b6278](https://github.com/thesimonho/warden/commit/e6b627855bdc3782947c21e4c3653fe011d61236))
* simplify bind mount UX with Git/SSH passthrough toggles ([#21](https://github.com/thesimonho/warden/issues/21)) ([c685377](https://github.com/thesimonho/warden/commit/c6853770a3f3a8f362139bf8ecd33900296113c0))


### Bug Fixes

* access item mounts causing perpetual stale mount detection ([#26](https://github.com/thesimonho/warden/issues/26)) ([f9a506b](https://github.com/thesimonho/warden/commit/f9a506b5671d63be034235bdc1430cf1c4dda409))
* add missing project names to audit events and improve data display ([#31](https://github.com/thesimonho/warden/issues/31)) ([c76a94b](https://github.com/thesimonho/warden/commit/c76a94b7b0334593596a5f84dfeb8ed592b507d6))
* canvas zoom not intercepting browser Ctrl+scroll ([#20](https://github.com/thesimonho/warden/issues/20)) ([fa80fb7](https://github.com/thesimonho/warden/commit/fa80fb786e1d35b2462577c713e99a768704c964))
* project cards never showing attention state ([#23](https://github.com/thesimonho/warden/issues/23)) ([ac19e8b](https://github.com/thesimonho/warden/commit/ac19e8b010647e8ef67178514cfb214590f207f0))
* resolve endpoint accepts items directly, enabling test during creation ([#27](https://github.com/thesimonho/warden/issues/27)) ([fc220ad](https://github.com/thesimonho/warden/commit/fc220ad61d143c1dc51dfcf5dcf7ae50cb682d88))
* SSH agent passthrough blocked by IdentitiesOnly in mounted config ([#22](https://github.com/thesimonho/warden/issues/22)) ([41410d2](https://github.com/thesimonho/warden/commit/41410d224a1364b5600c89c4cb91d4d94d91ae67))
* sync OpenAPI spec with actual API implementation ([#15](https://github.com/thesimonho/warden/issues/15)) ([811a926](https://github.com/thesimonho/warden/commit/811a9264abbb870d78dd6cb93add383877d72e0d))
* use semantic success color for access item availability dot ([#30](https://github.com/thesimonho/warden/issues/30)) ([68acbf5](https://github.com/thesimonho/warden/commit/68acbf54a203f6f6f559d45d8bc3daf1e73787da))

## [0.3.0](https://github.com/thesimonho/warden/compare/v0.2.0...v0.3.0) (2026-03-28)


### Features

* add file browse mode to directory browser ([#11](https://github.com/thesimonho/warden/issues/11)) ([fb1bcbd](https://github.com/thesimonho/warden/commit/fb1bcbd22645cacc14c46bef7c8b028a6c6b0dc0))


### Bug Fixes

* audit log event data and UI improvements ([#8](https://github.com/thesimonho/warden/issues/8)) ([f4993ae](https://github.com/thesimonho/warden/commit/f4993ae1813acdf32b7c80128a9e346e73b483e5))
* dynamic DNS filtering and SSH agent forwarding for containers ([#10](https://github.com/thesimonho/warden/issues/10)) ([a35d1f7](https://github.com/thesimonho/warden/commit/a35d1f76cd6fa428803307fefc5dabe6cb44b058))
* exclude event directory from stale mount validation ([#9](https://github.com/thesimonho/warden/issues/9)) ([4c55413](https://github.com/thesimonho/warden/commit/4c554131bad7810b995edea59d9f3bb3a057a9dd))
* improve audit table usability and layout ([#13](https://github.com/thesimonho/warden/issues/13)) ([6892395](https://github.com/thesimonho/warden/commit/68923956d790b2d1c44431485323ecdc23d6f0f4))
* improve manage project dialog UX ([#4](https://github.com/thesimonho/warden/issues/4)) ([e5cc1cd](https://github.com/thesimonho/warden/commit/e5cc1cd9900cd94ae1bbdc8cb00ec3ae84464a05))

## [0.2.0](https://github.com/thesimonho/warden/compare/v0.1.0...v0.2.0) (2026-03-28)


### Features

* initial commit ([26e9efe](https://github.com/thesimonho/warden/commit/26e9efea38da8dd94dc3092ab279945e3fc14269))
