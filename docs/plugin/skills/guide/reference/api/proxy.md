<!-- GENERATED from docs/openapi/swagger.yaml — do not edit manually -->

# Proxy API

All error responses return `{"error": "message", "code": "ERROR_CODE"}`.
## Proxy to container port

- **Method:** `GET`
- **Path:** `/api/v1/projects/{projectId}/{agentType}/proxy/{port}/{path}`
- **Tags:** proxy

Reverse-proxies HTTP and WebSocket requests to the specified port inside the project's container. The port must be declared in the project's forwardedPorts list. Supports all HTTP methods and WebSocket upgrade for HMR.

#### Responses

##### Status: 200 Proxied response from container

##### Status: 400 Invalid port number
##### Status: 404 Port not declared or project not found
##### Status: 502 Container unreachable
