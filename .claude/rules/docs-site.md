---
paths:
  - "docs/site/**/*"
  - "docs/**/*"
  - "README.md"
  - "CONTRIBUTING.md"
  - "CLAUDE.md"
  - "service/**/*"
  - "engine/**/*"
  - "client/**/*"
  - "api/**/*"
  - "cmd/**/*"
  - "internal/tui/**/*"
---

# Docs Site

The docs site is a Starlight site at `https://thesimonho.github.io/warden/` (base path `/warden/`).

```bash
just docs-dev              # Generate all docs + start Starlight dev server
just docs-build            # Generate all docs + full production build
```

## Key rule: never edit gitignored site content

Some pages in `docs/site/src/content/docs/` are **gitignored and generated at build time**. Never edit gitignored files — find and edit the source instead.

## Site Sections

- **`Guide`** → End user getting started guides. Keep files directly up to date.
- **`Features`** → End user feature explanations. Keep files directly up to date.
- **`Integration`** → Embedded at build time. Keep their _sources_ up to date at `docs/plugin/skills/warden/reference/`.
- **`Reference`** → Go package docs are regenerated at build time (gitignored). `reference/go/index.md` is the only hand-authored file.

## Agent-format API docs

Files in `docs/plugin/skills/warden/reference/api/` are auto-generated from `docs/openapi/swagger.yaml` but **committed** (not gitignored) because the plugin distribution needs them. They regenerate automatically as part of `just docs-build`. CI checks freshness. DO NOT edit them directly.

## Link rules

1. Non-index pages render at `/warden/<section>/<name>/` — sibling links need `../`.
2. Use absolute paths (`/warden/...`) for cross-section links, relative for within-section.
3. After renaming/moving a page, grep `docs/`, `README.md`, `CONTRIBUTING.md`, and `.claude/rules/` for old paths.
