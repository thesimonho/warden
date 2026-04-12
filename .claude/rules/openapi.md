---
paths:
  - "internal/server/**/*"
  - "docs/openapi/**/*"
  - "docs/site/**/*"
  - "api/**/*"
  - "access/**/*"
  - "engine/**/*"
  - "event/**/*"
  - "service/**/*"
  - "client/**/*"
---

# OpenAPI Spec

The OpenAPI 3.1 spec is generated from swaggo/swag v2 annotations on handler functions in `internal/server/routes.go` and `internal/server/server.go`. General API info lives in `internal/server/doc.go`. Anonymous request/response types for swag are in `internal/server/openapi_types.go`.

## Keeping annotations in sync

Every `@Router` annotation MUST match the corresponding `mux.HandleFunc` registration in `registerAPIRoutes()`. When you:

- **Add a new endpoint**: add the `mux.HandleFunc` registration AND the swag annotations on the handler in the same commit.
- **Change a route path**: update both `mux.HandleFunc` and the `@Router` annotation.
- **Change request/response types**: update the `@Param`/`@Success`/`@Failure` annotations to reference the correct types.
- **Remove an endpoint**: remove both the registration and all swag annotations.

## Regenerating the spec

After changing any annotations, regenerate and commit the result:

```bash
just openapi
```
