---
name: Documentation Locations
description: Where project documentation lives in the Warden codebase
type: reference
---

## Documentation Locations

- **codemaps**: `/home/simon/Projects/warden/docs/developer/codemaps/` — Barrel README.md links to backend/ (api-server, service, database, events, supporting), frontend/ (app structure, components, hooks, testing), tui/ (architecture, views, components), container/ (image, security, scripts, environment)
- **references**: `/home/simon/Projects/warden/docs/developer/` + `/home/simon/Projects/warden/docs/openapi/` — Contributor docs: architecture.md, terminology.md, events.md, parser.md, worktrees.md, events_claude.md, events_codex.md; root-level: CLAUDE.md, CONTRIBUTING.md, README.md
- **site**: `/home/simon/Projects/warden/docs/site/` (generator: Astro Starlight) — Public documentation at https://thesimonho.github.io/warden/, config at astro.config.mjs, content in src/content/docs/
- **plugin**: `/home/simon/Projects/warden/docs/plugin/` — Claude Code plugin with `.claude-plugin/plugin.json`, agents/ (surveyor.md), skills/ (guide/)
