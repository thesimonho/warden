#!/usr/bin/env bash
# Regenerates the OpenAPI 3.1 spec from swag annotations.
# Called by both the justfile and the openapi.yml CI workflow.
set -euo pipefail

swag init --v3.1 --parseInternal --parseDependency \
  --generalInfo internal/server/doc.go \
  --output docs/openapi --outputTypes yaml
