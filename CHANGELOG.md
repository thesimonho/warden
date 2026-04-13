# Changelog

## [1.13.3](https://github.com/thesimonho/warden/compare/v1.13.2...v1.13.3) (2026-04-13)


### Bug Fixes

* prevent network mode from reverting to full on create ([#228](https://github.com/thesimonho/warden/issues/228)) ([2cb591d](https://github.com/thesimonho/warden/commit/2cb591d740fabf6ca35125ca8c33a6c82667d3ac))
* reduce noise from unresolvable symlink warnings ([#230](https://github.com/thesimonho/warden/issues/230)) ([c7c74f5](https://github.com/thesimonho/warden/commit/c7c74f5a2d4589d2a90232e051124167cbca4c55))
* toast close button position and theme colors ([#231](https://github.com/thesimonho/warden/issues/231)) ([723defd](https://github.com/thesimonho/warden/commit/723defd76311672380ec523b6bade18022f49501))

## [1.13.2](https://github.com/thesimonho/warden/compare/v1.13.1...v1.13.2) (2026-04-13)


### Bug Fixes

* evict proxy cache on stop/delete, increase idle conns ([#223](https://github.com/thesimonho/warden/issues/223)) ([a3f7c42](https://github.com/thesimonho/warden/commit/a3f7c423f104c6a7efa855f85ec89736646c18c4))
* isolate dev, production, and E2E environments ([#225](https://github.com/thesimonho/warden/issues/225)) ([44002c1](https://github.com/thesimonho/warden/commit/44002c185c32217a0cb64461eaf9bd73a6c82f5e))
* label ephemeral containers and clean up orphans on startup ([#219](https://github.com/thesimonho/warden/issues/219)) ([a452346](https://github.com/thesimonho/warden/commit/a4523463b9ae13b2b67982355605b0ca053dfb80))
* move toast close button to top-right ([#215](https://github.com/thesimonho/warden/issues/215)) ([96786b7](https://github.com/thesimonho/warden/commit/96786b7c998fdd1181c5b9c7788d1a7870815d31))
* port proxy unreachable on Docker Desktop ([#222](https://github.com/thesimonho/warden/issues/222)) ([39bd620](https://github.com/thesimonho/warden/commit/39bd6201ccc3733a536999cacf691598ff0f68a3))
* pre-flight container name check and dev mode suffix ([#220](https://github.com/thesimonho/warden/issues/220)) ([c22ebe6](https://github.com/thesimonho/warden/commit/c22ebe649fa12be3ee6e35c9791921b5d032e40a))
* reduce noisy container startup warnings ([#217](https://github.com/thesimonho/warden/issues/217)) ([d962376](https://github.com/thesimonho/warden/commit/d962376b70e48b9cb231b2542759c69389096ebf))
* reset isSubmitting state when edit dialog closes ([#224](https://github.com/thesimonho/warden/issues/224)) ([1f3db4d](https://github.com/thesimonho/warden/commit/1f3db4d85d685c43fa840de6d68241ddc8ba7bf0))
* sort project cards by Docker container state ([#221](https://github.com/thesimonho/warden/issues/221)) ([5a8f198](https://github.com/thesimonho/warden/commit/5a8f1985dd92f0adfc101cbde5fbd94fe80caae7))
* unblock restart API and update project cards via SSE ([#218](https://github.com/thesimonho/warden/issues/218)) ([1a73546](https://github.com/thesimonho/warden/commit/1a73546e609b8019d94363cd7a198425b114b058))

## [1.13.1](https://github.com/thesimonho/warden/compare/v1.13.0...v1.13.1) (2026-04-12)


### Bug Fixes

* auto-discover and mount gitconfig include/includeIf files ([#214](https://github.com/thesimonho/warden/issues/214)) ([4e88686](https://github.com/thesimonho/warden/commit/4e88686801115f1ccb9cd33c63ba0b9fbdd06c75))
* Docker Desktop path resolution for host actions ([#212](https://github.com/thesimonho/warden/issues/212)) ([b016b3c](https://github.com/thesimonho/warden/commit/b016b3c54f48d6c203a3208cedd2df6e2a38caf0))

## [1.13.0](https://github.com/thesimonho/warden/compare/v1.12.0...v1.13.0) (2026-04-11)


### Features

* add Terminal (shell) tab to worktree terminal card ([#210](https://github.com/thesimonho/warden/issues/210)) ([8135806](https://github.com/thesimonho/warden/commit/813580630c9f3bd0885fd495d080f761d64b4fda))
* set container timezone to match host ([#205](https://github.com/thesimonho/warden/issues/205)) ([19a2f93](https://github.com/thesimonho/warden/commit/19a2f936a6e3355779cb9ea6a14fc2b806af5d82))


### Bug Fixes

* detect manual agent restart in worktree state ([#209](https://github.com/thesimonho/warden/issues/209)) ([e08331f](https://github.com/thesimonho/warden/commit/e08331f554f92ba73982176b0239d94191e2dd59))
* disable Claude Code autoupdater in managed settings ([#208](https://github.com/thesimonho/warden/issues/208)) ([54f6dd5](https://github.com/thesimonho/warden/commit/54f6dd54c7ec45df5efe350705809f3f051ddf4d))
* hide stale tray attention submenu items instead of removing ([#207](https://github.com/thesimonho/warden/issues/207)) ([ae38b2a](https://github.com/thesimonho/warden/commit/ae38b2a49e262bce42882e54d65036317c85f57f))

## [1.12.0](https://github.com/thesimonho/warden/compare/v1.11.0...v1.12.0) (2026-04-11)


### Features

* auto-connect worktrees via URL and tray attention submenu ([#202](https://github.com/thesimonho/warden/issues/202)) ([7e986c9](https://github.com/thesimonho/warden/commit/7e986c94d323acb25a85656115aad33b2143ecc5))


### Bug Fixes

* mount GPG public keyring and recover bridges after stop ([#204](https://github.com/thesimonho/warden/issues/204)) ([3f61a18](https://github.com/thesimonho/warden/commit/3f61a184428ff576db01b386a421165c0a2f65ef))
* suppress desktop notifications for focused projects ([#200](https://github.com/thesimonho/warden/issues/200)) ([ba1e192](https://github.com/thesimonho/warden/commit/ba1e1929092e7e720d4fa4a1dd2adaf84db890eb))

## [1.11.0](https://github.com/thesimonho/warden/compare/v1.10.2...v1.11.0) (2026-04-10)


### Features

* add GPG signing as built-in access item ([#189](https://github.com/thesimonho/warden/issues/189)) ([3299f39](https://github.com/thesimonho/warden/commit/3299f3974597e49a95d0ed56140186177516468f))
* replace browser notifications with native desktop notifications ([#197](https://github.com/thesimonho/warden/issues/197)) ([a1f9462](https://github.com/thesimonho/warden/commit/a1f94629c863bf615da82ae32f6c7fcd5d092c3b))
* TCP socket bridge for SSH/GPG agent forwarding ([#191](https://github.com/thesimonho/warden/issues/191)) ([db3d127](https://github.com/thesimonho/warden/commit/db3d127a43c9952c3fd1be0be5abf3a005b50f5f))


### Bug Fixes

* container image CVEs, Docker Desktop compat, file sharing hints ([#188](https://github.com/thesimonho/warden/issues/188)) ([d4aed68](https://github.com/thesimonho/warden/commit/d4aed6868f5ab46faafd11fd012ac1008c92cb10))
* detect Docker Desktop and probe sockets for liveness ([#190](https://github.com/thesimonho/warden/issues/190)) ([0168fcb](https://github.com/thesimonho/warden/commit/0168fcb9f1d5ba88c9a6c89b8672b14c5fd37f3e))
* JSON-encode container log tail in audit events ([#192](https://github.com/thesimonho/warden/issues/192)) ([6d53723](https://github.com/thesimonho/warden/commit/6d537236265938f075b350ddb888ecb709cce0a8))
* network blocked domain resolution, Docker Desktop paths, recreation race ([#194](https://github.com/thesimonho/warden/issues/194)) ([890906b](https://github.com/thesimonho/warden/commit/890906bcf2c273b00701707922500ee104e331ed))
* populate default domains when switching to restricted network mode ([#195](https://github.com/thesimonho/warden/issues/195)) ([0f8eede](https://github.com/thesimonho/warden/commit/0f8eede17a3d2ddfd71a6d54be39c3906a57a888))
* prevent phantom WebSocket reconnect during Strict Mode double-mount ([#199](https://github.com/thesimonho/warden/issues/199)) ([8f89e25](https://github.com/thesimonho/warden/commit/8f89e25b2b39c2fcb0981c7fa11972275559274b))
* protect host symlinks from entrypoint dereference ([#198](https://github.com/thesimonho/warden/issues/198)) ([59b09a7](https://github.com/thesimonho/warden/commit/59b09a763ba8b4ed2232fdb579342471b216d6b2))
* reliable SSH/GPG agent forwarding across all Docker runtimes ([#193](https://github.com/thesimonho/warden/issues/193)) ([913a8c7](https://github.com/thesimonho/warden/commit/913a8c72a0c8fe77002bddf1c623b3dd515091a8))
* SSH agent socket inaccessible inside containers ([#184](https://github.com/thesimonho/warden/issues/184)) ([ead14f9](https://github.com/thesimonho/warden/commit/ead14f98c1bd912a7396be87b532f1df76968a09))
* stale mount detection and Docker context discovery ([#187](https://github.com/thesimonho/warden/issues/187)) ([d66f00b](https://github.com/thesimonho/warden/commit/d66f00bb0ab3bd78f84a62c1735cb68891c43322))

## [1.10.2](https://github.com/thesimonho/warden/compare/v1.10.1...v1.10.2) (2026-04-08)


### Bug Fixes

* platform-aware SSH agent socket mounting for Docker Desktop ([#181](https://github.com/thesimonho/warden/issues/181)) ([d608f50](https://github.com/thesimonho/warden/commit/d608f504de30db0441fd3c4627a3f40833d784c3))
* remote repo create button disabled and temporary flag not loading ([#183](https://github.com/thesimonho/warden/issues/183)) ([6e59a9c](https://github.com/thesimonho/warden/commit/6e59a9c7ee5c0d36f519c2a94112e68fc967dd7d)), closes [#179](https://github.com/thesimonho/warden/issues/179)

## [1.10.1](https://github.com/thesimonho/warden/compare/v1.10.0...v1.10.1) (2026-04-08)


### Bug Fixes

* tray icon theme detection and attention indicator ([#177](https://github.com/thesimonho/warden/issues/177)) ([48f2b8c](https://github.com/thesimonho/warden/commit/48f2b8c83b46948e3c71b494e832919bbb26e324))

## [1.10.0](https://github.com/thesimonho/warden/compare/v1.9.1...v1.10.0) (2026-04-08)


### Features

* add support for remote git repository projects ([#168](https://github.com/thesimonho/warden/issues/168)) ([b33ae40](https://github.com/thesimonho/warden/commit/b33ae40bf5e59ed6b86d2476b671767cf56930f8))


### Bug Fixes

* automate icon generation and improve tray icon rendering ([#172](https://github.com/thesimonho/warden/issues/172)) ([8bc8574](https://github.com/thesimonho/warden/commit/8bc8574f097f6f9abc63673b9e6d3adcdb9f04d4))
* enable terminal copy via OSC 52 clipboard forwarding ([#167](https://github.com/thesimonho/warden/issues/167)) ([5e99ade](https://github.com/thesimonho/warden/commit/5e99ade5aac1bd5dba7c28acea1a2080d2c73302)), closes [#165](https://github.com/thesimonho/warden/issues/165)
* fall back to fresh session when auto-resume fails ([#161](https://github.com/thesimonho/warden/issues/161)) ([d92eda5](https://github.com/thesimonho/warden/commit/d92eda57ff99a509ca46d93fcb9e104b97bc4f6a))
* focus terminal after xterm attaches to DOM ([#166](https://github.com/thesimonho/warden/issues/166)) ([098f4c4](https://github.com/thesimonho/warden/commit/098f4c46db85f52e012be2cf6f7000eea4f89858)), closes [#163](https://github.com/thesimonho/warden/issues/163)
* move network isolation to privileged docker exec ([#169](https://github.com/thesimonho/warden/issues/169)) ([b0ca88d](https://github.com/thesimonho/warden/commit/b0ca88d5d3efc3b711bf67b7623e518462c4e6bd))
* project source UI and network isolation sequencing ([#174](https://github.com/thesimonho/warden/issues/174)) ([4aab3c6](https://github.com/thesimonho/warden/commit/4aab3c6f951497bfa60663f75f19c155acdc2cdc))
* update icon generation workflow to run on pull requests ([#173](https://github.com/thesimonho/warden/issues/173)) ([9f126fd](https://github.com/thesimonho/warden/commit/9f126fdc2891359dda4eb3caf0d9852b84b160ed))

## [1.9.1](https://github.com/thesimonho/warden/compare/v1.9.0...v1.9.1) (2026-04-07)


### Bug Fixes

* add firewall verification smoke test for restricted mode ([#158](https://github.com/thesimonho/warden/issues/158)) ([6b52845](https://github.com/thesimonho/warden/commit/6b528459c9484f665b2643f9ebe7dfa50c707207))
* add GitHub meta API ranges and SSH seed for restricted mode ([#157](https://github.com/thesimonho/warden/issues/157)) ([04b64dc](https://github.com/thesimonho/warden/commit/04b64dca842f53a7fd807c052f82fc711399fd94))

## [1.9.0](https://github.com/thesimonho/warden/compare/v1.8.1...v1.9.0) (2026-04-07)


### Features

* add port forwarding UI and subdomain-based proxy ([#156](https://github.com/thesimonho/warden/issues/156)) ([b181396](https://github.com/thesimonho/warden/commit/b18139684eedb5c92b266a5f8ac22dad2e01f354))
* add port forwarding via reverse proxy ([#155](https://github.com/thesimonho/warden/issues/155)) ([dda7d43](https://github.com/thesimonho/warden/commit/dda7d4344c61cdd6edc82bccddfbfa0ef43c5dd9))


### Bug Fixes

* add missing packaging metadata across all platforms ([#153](https://github.com/thesimonho/warden/issues/153)) ([35756d3](https://github.com/thesimonho/warden/commit/35756d3d5f7c2743295cefee3e3f3a7199c051f1))

## [1.8.1](https://github.com/thesimonho/warden/compare/v1.8.0...v1.8.1) (2026-04-06)


### Bug Fixes

* harden container security boundary ([#146](https://github.com/thesimonho/warden/issues/146)) ([b08ac5e](https://github.com/thesimonho/warden/commit/b08ac5e410912be107a44866aa55c6dcd0273269))


### Performance Improvements

* reduce docker exec calls, polling overhead, and goroutine count ([#149](https://github.com/thesimonho/warden/issues/149)) ([d756cca](https://github.com/thesimonho/warden/commit/d756cca9fe0b294218c17bee12a0db1bfee90b32))

## [1.8.0](https://github.com/thesimonho/warden/compare/v1.7.2...v1.8.0) (2026-04-06)


### Features

* add batch project operations endpoint ([#143](https://github.com/thesimonho/warden/issues/143)) ([3af38db](https://github.com/thesimonho/warden/commit/3af38db7c4655644bdc07012635ff10c63129f02))
* add GET endpoints for single project, costs, budget, and worktree ([#139](https://github.com/thesimonho/warden/issues/139)) ([9105e47](https://github.com/thesimonho/warden/commit/9105e472082e7c6a3302d222d909a765335ef131))
* add POST worktree input endpoint for headless/CI use ([#140](https://github.com/thesimonho/warden/issues/140)) ([700fc8a](https://github.com/thesimonho/warden/commit/700fc8a29077115a79f3cedacc89ecee55336170))
* add SSE project filtering for integrators ([#141](https://github.com/thesimonho/warden/issues/141)) ([b684afe](https://github.com/thesimonho/warden/commit/b684afed06ea484e83d3d1c5635f29491673459a))
* combined create project + container in single request ([#142](https://github.com/thesimonho/warden/issues/142)) ([5d62d68](https://github.com/thesimonho/warden/commit/5d62d680d453dca5457b02ecdfd6b629a3b10d1b))
* expand POST /api/v1/audit for integrators ([#137](https://github.com/thesimonho/warden/issues/137)) ([e5fd8be](https://github.com/thesimonho/warden/commit/e5fd8be57f5dfc8b036fdbc85ac02ef65b01e908))

## [1.7.2](https://github.com/thesimonho/warden/compare/v1.7.1...v1.7.2) (2026-04-06)


### Bug Fixes

* allow docs/plugin changes to trigger release-please ([#133](https://github.com/thesimonho/warden/issues/133)) ([caa4a21](https://github.com/thesimonho/warden/commit/caa4a211455cbac19429d97fdada23101ff90ee8))
* path to specific agent files ([#131](https://github.com/thesimonho/warden/issues/131)) ([3092cd4](https://github.com/thesimonho/warden/commit/3092cd443d32386b7f1c7ebababba757bd6e3d16))

## [1.7.1](https://github.com/thesimonho/warden/compare/v1.7.0...v1.7.1) (2026-04-06)


### Bug Fixes

* plugin manifest path and skill/agent renaming ([#129](https://github.com/thesimonho/warden/issues/129)) ([6b370a1](https://github.com/thesimonho/warden/commit/6b370a1646d07e6123944c8a328c8dcecda89f6f))

## [1.7.0](https://github.com/thesimonho/warden/compare/v1.6.6...v1.7.0) (2026-04-06)


### Features

* add POST /api/v1/shutdown endpoint ([#119](https://github.com/thesimonho/warden/issues/119)) ([8ad30be](https://github.com/thesimonho/warden/commit/8ad30bee93c2aabf201aa56abde1293d170a94a5))
* add system tray companion and exit messages ([#120](https://github.com/thesimonho/warden/issues/120)) ([3a09e84](https://github.com/thesimonho/warden/commit/3a09e84ea4a6abf63bd0d8664848b8b9da3c7f72))
* plugin system and docs restructuring ([#128](https://github.com/thesimonho/warden/issues/128)) ([362c54a](https://github.com/thesimonho/warden/commit/362c54a6c89145bcbcb37363eb7d45d4f91729fe))
* stepped project config form with tab navigation ([#125](https://github.com/thesimonho/warden/issues/125)) ([f117709](https://github.com/thesimonho/warden/commit/f117709712f50ea14966fd19579cb2e149aae831))
* use native file picker for .warden.json template import ([#124](https://github.com/thesimonho/warden/issues/124)) ([fc85e12](https://github.com/thesimonho/warden/commit/fc85e1277db721966113c3b3db0d94112d3810b1))


### Bug Fixes

* harden .warden.json template write-back system ([#123](https://github.com/thesimonho/warden/issues/123)) ([b78b3f9](https://github.com/thesimonho/warden/commit/b78b3f90e237ccfe9369ea942a56a164851606f0))
* isolate shell env spawn to prevent SIGTTOU in TUI ([#117](https://github.com/thesimonho/warden/issues/117)) ([3564f33](https://github.com/thesimonho/warden/commit/3564f337600ddd468c366e246a37a0568392c293))
* prevent process hang after TUI quit ([#122](https://github.com/thesimonho/warden/issues/122)) ([b0f1ce5](https://github.com/thesimonho/warden/commit/b0f1ce507627fc4cde631d802c5b1147e7b8e5f3))

## [1.6.6](https://github.com/thesimonho/warden/compare/v1.6.5...v1.6.6) (2026-04-05)


### Bug Fixes

* resolve access items from shell env when launched from desktop ([#115](https://github.com/thesimonho/warden/issues/115)) ([b41d05a](https://github.com/thesimonho/warden/commit/b41d05ab4c9e9752eb41b8c0028a81e2e034430d))

## [1.6.5](https://github.com/thesimonho/warden/compare/v1.6.4...v1.6.5) (2026-04-05)


### Bug Fixes

* rename Windows installer to avoid filename collision ([#113](https://github.com/thesimonho/warden/issues/113)) ([808344f](https://github.com/thesimonho/warden/commit/808344f8fe4c5afddc5ed30a7621e000ad76507e))

## [1.6.4](https://github.com/thesimonho/warden/compare/v1.6.3...v1.6.4) (2026-04-05)


### Bug Fixes

* consistent installer naming (warden-desktop-{platform}-{arch}) ([#111](https://github.com/thesimonho/warden/issues/111)) ([0714fd4](https://github.com/thesimonho/warden/commit/0714fd4c1e1ad10019106cce803139bbcce82215))

## [1.6.3](https://github.com/thesimonho/warden/compare/v1.6.2...v1.6.3) (2026-04-05)


### Bug Fixes

* use host-arch appimagetool for arm64 AppImage builds ([#109](https://github.com/thesimonho/warden/issues/109)) ([234c3c0](https://github.com/thesimonho/warden/commit/234c3c02efafbbe2cfd0530121329e3bd6359656))

## [1.6.2](https://github.com/thesimonho/warden/compare/v1.6.1...v1.6.2) (2026-04-05)


### Bug Fixes

* install nfpm for host arch, not target arch ([#107](https://github.com/thesimonho/warden/issues/107)) ([df6b2a9](https://github.com/thesimonho/warden/commit/df6b2a91061447133b1bcfa74c1c4716a966d34a))

## [1.6.1](https://github.com/thesimonho/warden/compare/v1.6.0...v1.6.1) (2026-04-05)


### Bug Fixes

* CI failures in release-build workflow ([#105](https://github.com/thesimonho/warden/issues/105)) ([c402474](https://github.com/thesimonho/warden/commit/c4024744d9df4059f2f44b28c77e832e4a28a202))

## [1.6.0](https://github.com/thesimonho/warden/compare/v1.5.1...v1.6.0) (2026-04-05)


### Features

* graceful Docker unavailability with prerequisite warnings ([#103](https://github.com/thesimonho/warden/issues/103)) ([806904f](https://github.com/thesimonho/warden/commit/806904f8b65c5b24db7c139de77df4659e635d0b))
* platform packaging ([#102](https://github.com/thesimonho/warden/issues/102)) ([f247bd2](https://github.com/thesimonho/warden/commit/f247bd23caa06f28d16707ada097ad40ebdc5db4))


### Bug Fixes

* add missing agentType to AttachTerminal ([cb99c70](https://github.com/thesimonho/warden/commit/cb99c70e76fe81cf698d87277b5470e290dc7bfd))
* use primary color for nav icons ([#95](https://github.com/thesimonho/warden/issues/95)) ([02fd069](https://github.com/thesimonho/warden/commit/02fd069801b03ecc3b356c6d394b76104710bffd))


### Performance Improvements

* lazy-load routes to reduce initial bundle by 87% ([#104](https://github.com/thesimonho/warden/issues/104)) ([3f4e2b2](https://github.com/thesimonho/warden/commit/3f4e2b208bbbccbe15ffbb2fa250d411f725687b))

## [1.5.1](https://github.com/thesimonho/warden/compare/v1.5.0...v1.5.1) (2026-04-04)


### Bug Fixes

* add missing id field in test fixtures, clean up E2E audit data ([#92](https://github.com/thesimonho/warden/issues/92)) ([1723422](https://github.com/thesimonho/warden/commit/17234229a1446fb189d08ee1cb835e7e7f23e994))

## [1.5.0](https://github.com/thesimonho/warden/compare/v0.6.0...v1.5.0) (2026-04-04)


### Features

* add reset worktree action ([#89](https://github.com/thesimonho/warden/issues/89)) ([109fab1](https://github.com/thesimonho/warden/commit/109fab175c2a0a0d9bfe3ac791df3ebd048241cf))
* format bash mode prompts in audit log ([#85](https://github.com/thesimonho/warden/issues/85)) ([1ddd5c5](https://github.com/thesimonho/warden/commit/1ddd5c5182fdf36cedd8f918a9466a26b28fcfae))
* install agent CLIs at startup, slim container image ([#78](https://github.com/thesimonho/warden/issues/78)) ([a421cd7](https://github.com/thesimonho/warden/commit/a421cd74385f41240b8d28b2eafc1851ff8946cb))
* log blocked network connections to audit log ([#82](https://github.com/thesimonho/warden/issues/82)) ([e5627fe](https://github.com/thesimonho/warden/commit/e5627fe859125366631fe62b91ce11ff0f973c87))
* persist FileTailer byte offsets to prevent audit event replay ([#91](https://github.com/thesimonho/warden/issues/91)) ([040fb9e](https://github.com/thesimonho/warden/commit/040fb9ed5d27307367b369b42608e8df1bacec6a))
* project templates via .warden.json ([#81](https://github.com/thesimonho/warden/issues/81)) ([37cb083](https://github.com/thesimonho/warden/commit/37cb083e71fef59ba3554e8863335a445417942d))


### Bug Fixes

* clear session costs when deleting audit events ([#83](https://github.com/thesimonho/warden/issues/83)) ([55be8a5](https://github.com/thesimonho/warden/commit/55be8a58f70f87e54295d2abfe3eb431f7455103))
* parse Codex user shell commands in audit log ([#86](https://github.com/thesimonho/warden/issues/86)) ([8571e08](https://github.com/thesimonho/warden/commit/8571e0834cf78be57b704ca7903d0b779cfeb30d))
* prevent audit timeline tooltip from overflowing viewport ([#84](https://github.com/thesimonho/warden/issues/84)) ([684102e](https://github.com/thesimonho/warden/commit/684102e01fb61fa114187fe87217ca3888a918ca))
* remove audit history option from project delete popup ([#90](https://github.com/thesimonho/warden/issues/90)) ([3b3beab](https://github.com/thesimonho/warden/commit/3b3beab222226806ab15a09b6dfdda32226cce57))
* use database row ID as audit log entry key ([#87](https://github.com/thesimonho/warden/issues/87)) ([298c36b](https://github.com/thesimonho/warden/commit/298c36b8118681cacaae3795b73221336ccdb202))

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
