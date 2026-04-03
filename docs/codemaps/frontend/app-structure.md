# App Structure

## Entry Files

| File            | Purpose                                                                                                                  |
| --------------- | ------------------------------------------------------------------------------------------------------------------------ |
| `src/main.tsx`  | React root mount                                                                                                         |
| `src/App.tsx`   | Router setup: `/` (home), `/projects/:id/:agentType` (project), `/access`, `/audit`                                      |
| `src/index.css` | Tailwind v4 base + theme imports; custom font-size scale (shifted down one level: `text-base` = 14px), h1-h3 base styles |

## Pages

| File                         | Route                      | Purpose                                                                                                                                                                                                                                                              |
| ---------------------------- | -------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `src/pages/home-page.tsx`    | `/`                        | Project grid with cost dashboard and cards                                                                                                                                                                                                                           |
| `src/pages/project-page.tsx` | `/projects/:id/:agentType` | Route wrapper — reads project ID and agent type from URL, renders `ProjectView` in fixed viewport layout. Subscribes to `budget_container_stopped` SSE events and auto-redirects to home page when the current project's container is stopped by budget enforcement. |
| `src/pages/access-page.tsx`  | `/access`                  | Access item management with list of user and built-in items, detection status, test/resolve preview, create/edit dialogs, delete capability (user items only)                                                                                                        |
| `src/pages/audit-page.tsx`   | `/audit`                   | Audit log viewer with summary dashboard (sessions/tools/prompts/cost), category filters, level filters, project filter, activity timeline brush, CSV/JSON export, scoped delete dialog (by project/category/age) with type-to-confirm                                |

## Text Sizing

The font-size scale in `src/index.css` is shifted down one level from Tailwind defaults, making body text default to 14px.

| Class       | Size | Use for                       |
| ----------- | ---- | ----------------------------- |
| `text-xs`   | 10px | Fine print, zoom indicators   |
| `text-sm`   | 12px | Secondary labels, metadata    |
| `text-base` | 14px | Body text (inherited default) |
| `text-lg`   | 16px | Emphasis, subheadings         |
| `text-xl`   | 18px | h2 headings                   |
| `text-2xl`  | 20px | h1 headings                   |

Heading elements (`h1`--`h3`) have base styles defined in `src/index.css` -- don't repeat sizing/weight on heading tags.

## Themes

| File                    | Purpose                                                                     |
| ----------------------- | --------------------------------------------------------------------------- |
| `themes/permafrost.css` | Light theme — defines semantic colors, `--category-*` audit category colors |
| `themes/frostpunk.css`  | Dark theme — defines semantic colors, `--category-*` audit category colors  |
