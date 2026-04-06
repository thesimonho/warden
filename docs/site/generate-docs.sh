#!/usr/bin/env bash
# Unified documentation generation script.
# Generates all content that is gitignored in the docs site:
#   - Plugin reference files → site features/ and integration/ pages
#   - Go package reference (gomarkdoc)
#   - Changelog and contributing pages
#   - Agent-format API docs (committed, not gitignored — for plugin distribution)
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"
SITE_CONTENT="${SCRIPT_DIR}/src/content/docs"
PLUGIN_REF="${PROJECT_ROOT}/docs/plugin/skills/guide/reference"
MODULE_PATH="github.com/thesimonho/warden"

# --- Helper: generate a site page from a source markdown file ---
# Strips H1 heading (Starlight uses frontmatter title), converts absolute
# site URLs to relative paths, and prepends Starlight frontmatter.
generate_page() {
  local src="$1" out="$2" title="$3" desc="$4"
  if [[ ! -f "$src" ]]; then
    echo "ERROR: source file not found: $src" >&2
    return 1
  fi
  local relative_src="${src#"${PROJECT_ROOT}"/}"
  local content
  content=$(tail -n +2 "$src" | sed 's|https://thesimonho.github.io/warden/|/warden/|g')
  cat > "$out" <<EOF
---
title: "${title}"
description: "${desc}"
editUrl: false
---
<!-- Generated from ${relative_src} — do not edit directly -->
${content}
EOF
  echo "  ${relative_src} -> ${out#"${SCRIPT_DIR}"/}"
}

# =============================================================================
# 1. Plugin reference → site integration pages
# =============================================================================
echo "=== Generating integration pages ==="
mkdir -p "${SITE_CONTENT}/integration"

generate_page "${PLUGIN_REF}/concepts.md" \
  "${SITE_CONTENT}/integration/architecture.md" \
  "Architecture" "How Warden is structured — layered system, infrastructure layout."

generate_page "${PLUGIN_REF}/paths.md" \
  "${SITE_CONTENT}/integration/paths.md" \
  "Integration Paths" "Integrate Warden using HTTP API, Go client, or Go library."

generate_page "${PLUGIN_REF}/examples/api.md" \
  "${SITE_CONTENT}/integration/http-api.md" \
  "HTTP API" "Integrate Warden using the REST API from any language."

generate_page "${PLUGIN_REF}/examples/client.md" \
  "${SITE_CONTENT}/integration/go-client.md" \
  "Go Client" "Use the typed Go client to talk to a running Warden server."

generate_page "${PLUGIN_REF}/examples/library.md" \
  "${SITE_CONTENT}/integration/go-library.md" \
  "Go Library" "Embed Warden directly in your Go application."

generate_page "${PLUGIN_REF}/environment-variables.md" \
  "${SITE_CONTENT}/integration/environment-variables.md" \
  "Environment Variables" "Configuration environment variables for Warden."

# =============================================================================
# 3. Go package reference (gomarkdoc)
# =============================================================================
echo "=== Generating Go reference docs ==="

GO_OUTPUT_DIR="${SITE_CONTENT}/reference/go"
mkdir -p "${GO_OUTPUT_DIR}"

PACKAGES=(
  "." "access" "agent" "agent/claudecode" "agent/codex" "api" "constants"
  "client" "db" "engine" "eventbus" "runtime" "runtimes" "service" "watcher"
)

cd "${PROJECT_ROOT}"

for pkg in "${PACKAGES[@]}"; do
  if [ "${pkg}" = "." ]; then
    pkg_name="warden"
    import_path="${MODULE_PATH}"
    pkg_dir="."
  else
    pkg_name="${pkg}"
    import_path="${MODULE_PATH}/${pkg}"
    pkg_dir="./${pkg}"
  fi

  output_basename="${pkg_name##*/}"
  output_file="${GO_OUTPUT_DIR}/${output_basename}.md"

  echo "  Generating ${import_path}..."
  doc_content=$(gomarkdoc "${pkg_dir}" 2>&1) || {
    echo "    WARNING: gomarkdoc failed for ${pkg_name}" >&2
    continue
  }

  if [ -z "${doc_content}" ]; then
    echo "    Skipping ${pkg_name} (no exportable content)"
    continue
  fi

  cat > "${output_file}" <<EOF
---
title: "${pkg_name}"
description: "Go package documentation for ${import_path}"
editUrl: false
---

> [View on pkg.go.dev](https://pkg.go.dev/${import_path})

${doc_content}
EOF
done

# =============================================================================
# 4. Changelog
# =============================================================================
echo "=== Generating changelog ==="

cat > "${SITE_CONTENT}/changelog.md" <<'FRONTMATTER'
---
title: Changelog
description: Release history for Warden.
editUrl: false
---

FRONTMATTER
tail -n +2 "${PROJECT_ROOT}/CHANGELOG.md" >> "${SITE_CONTENT}/changelog.md"

# =============================================================================
# 5. Contributing
# =============================================================================
echo "=== Generating contributing page ==="

generate_page "${PROJECT_ROOT}/CONTRIBUTING.md" \
  "${SITE_CONTENT}/contributing.md" \
  "Contributing" "How to contribute to Warden — development setup, coding guidelines, and submission process."

echo "=== Done ==="
