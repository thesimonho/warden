import path from 'node:path'
import tailwindcss from '@tailwindcss/vite'
import react from '@vitejs/plugin-react'
import { defineConfig } from 'vitest/config'

// https://vite.dev/config/
export default defineConfig({
  plugins: [react(), tailwindcss()],
  resolve: {
    alias: {
      '@': path.resolve(__dirname, './src'),
    },
  },
  server: {
    port: 5173,
    strictPort: true,
    proxy: {
      '/api': {
        target: 'http://localhost:8091',
        ws: true,
        // Return a clear error when the Go backend is unreachable,
        // instead of letting Vite's SPA fallback serve index.html.
        configure: (proxy) => {
          proxy.on('error', (_err, _req, res) => {
            if ('writeHead' in res) {
              const httpRes = res as import('http').ServerResponse
              if (!httpRes.headersSent) {
                httpRes.writeHead(502, { 'Content-Type': 'application/json' })
                httpRes.end(
                  JSON.stringify({
                    error: 'Go backend is not running on :8091. Start it with: just dev',
                  }),
                )
              }
            }
          })
        },
      },
    },
  },
  build: {
    outDir: 'dist',
    emptyOutDir: true,
  },
  test: {
    environment: 'jsdom',
    include: ['src/**/*.test.{ts,tsx}'],
  },
})
