# Changelog

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
