# web/ — React SPA frontend

The browser-based frontend for Warden. Built with React 19, TypeScript, Vite, and shadcn/ui.

## For developers building their own web frontend

**This is a reference implementation.** If you're building a web application that integrates with Warden, the key files to study are:

### [`src/lib/api.ts`](src/lib/api.ts)

The HTTP client that talks to the Warden API. Every API call pattern is demonstrated here — REST endpoints, request/response shapes, and error handling. Your application would make the same HTTP calls to a running `warden` server.

### [`src/lib/types.ts`](src/lib/types.ts)

TypeScript type definitions for all API request and response shapes, including SSE event payloads. Use these as a reference for the data contract between frontend and backend.

### [`src/hooks/use-event-source.ts`](src/hooks/use-event-source.ts)

SSE client for real-time events (worktree state changes, cost updates). Shows the reconnection strategy and event parsing.

### [`src/hooks/use-terminal.ts`](src/hooks/use-terminal.ts)

WebSocket client for terminal I/O. Shows how to connect to the terminal proxy, handle binary frames, and send resize events.

## Development

```bash
npm install          # install dependencies
npm run dev          # start Vite dev server (port 5173)
npm run build        # production build → dist/
npm run test         # run vitest
npm run typecheck    # TypeScript check
```

In development, Vite proxies `/api/*` to the Go backend on `:8090` (configured in `vite.config.ts`).

## Stack

| Layer | Technology |
|-------|-----------|
| Framework | React 19 |
| Build | Vite 7 |
| Language | TypeScript |
| UI | shadcn/ui + Tailwind CSS v4 |
| Terminal | xterm.js |
| State | React hooks + SSE |
