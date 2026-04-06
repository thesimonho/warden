<!-- GENERATED from docs/openapi/swagger.yaml — do not edit manually -->

# Health API

All error responses return `{"error": "message", "code": "ERROR_CODE"}`.
## Health check

- **Method:** `GET`
- **Path:** `/api/v1/health`
- **Tags:** health

Returns a simple health check response indicating the server is running.

#### Responses

##### Status: 200 OK

###### Content-Type: application/json

- **`status`**

  `string` — Status is always "ok" when the server is running.

- **`version`**

  `string` — Version is the server build version.

**Example:**

```json
{
  "status": "ok",
  "version": "v0.5.2"
}
```


---

## Shutdown server

- **Method:** `POST`
- **Path:** `/api/v1/shutdown`
- **Tags:** health

Requests a graceful shutdown of the Warden server process.

#### Responses

##### Status: 200 OK

###### Content-Type: application/json

- **`status`**

  `string` — Status is always "shutting down".

**Example:**

```json
{
  "status": "shutting down"
}
```


