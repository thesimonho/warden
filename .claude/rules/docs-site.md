---
paths:
  - "docs_site/**/*"
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

The docs site is a Starlight site deployed at `https://thesimonho.github.io/warden/`. All pages live in `docs_site/src/content/docs/`. The site base path is `/warden/`. Go package reference pages are auto-generated at build time by `gomarkdoc` and are gitignored. OpenAPI reference pages are auto-generated from `docs/openapi/swagger.yaml` by the `starlight-openapi` plugin.

```bash
just docs-dev              # Starlight dev server (generates Go docs first)
just docs-build            # Full production build (gomarkdoc + Starlight)
just docs-preview          # Build and preview locally
```

## Pages that MUST stay in sync with code changes

When you change features, APIs, packages, or behavior, update the relevant docs site pages:

| Page              | Path                          | Update when...                                                             |
| ----------------- | ----------------------------- | -------------------------------------------------------------------------- |
| Architecture      | `integration/architecture.md` | Layered system, infrastructure layout changes                              |
| Integration Paths | `integration/paths.md`        | New binary, package added/removed/renamed, new integration approach        |
| HTTP API          | `integration/http-api.mdx`    | Endpoints added/removed, error codes change, SSE events change             |
| Go Client         | `integration/go-client.md`    | Client API changes                                                         |
| Go Library        | `integration/go-library.md`   | `warden.New()` options, `App` methods, service methods, event types change |
| FAQ               | `faq.md`                      | New common questions arise, behavior changes affect existing answers       |
| Comparison        | `comparison.md`               | New features that affect competitive positioning                           |
| Contributing      | `contributing.md`             | Dev setup, testing commands, architecture rules, PR process changes        |
| Go Packages index | `reference/go/index.md`       | Public Go packages added or removed                                        |
| Go docs generator | `generate-go-docs.sh`         | Public Go packages added or removed (update `PACKAGES` array)              |

## Link rules

1. **Non-index pages use `../` for siblings.** Files like `paths.md`, `architecture.md`, `go-client.md` render at `/warden/integration/<name>/`, so a link to a sibling page needs `../`. Only `index.md` files can use bare relative links like `http-api/`.

2. **Use absolute paths (`/warden/...`) for cross-section links** (e.g., FAQ linking to integration pages). Use relative paths for within-section links.

3. **After renaming or moving a page**, grep the entire `docs_site/` directory, `README.md`, `CONTRIBUTING.md`, `docs/`, and `.claude/rules/` for links to the old path.

4. **OpenAPI external docs URL** in `internal/server/doc.go` must match the docs site. Regenerate the spec after changing it.

## Other documentation

| File                  | Update when...                                                  |
| --------------------- | --------------------------------------------------------------- |
| `README.md`           | Features, installation, comparison, or integration paths change |
| `CONTRIBUTING.md`     | Dev setup, testing, or PR process changes                       |
| `docs/codemaps/**/*.md` | Package structure, key functions, or constants change          |
| `docs/terminology.md` | New terms, states, or actions are introduced                    |
| `CLAUDE.md`           | Commands, stack, or architectural rules change                  |
