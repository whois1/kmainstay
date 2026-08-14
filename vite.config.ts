import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'

export default defineConfig({
  plugins: [vue()],
  build: { outDir: 'internal/webui/dist', emptyOutDir: true },
  server: { proxy: { '/api': 'http://localhost:8080', '/healthz': 'http://localhost:8080', '/api/ws': { target: 'ws://localhost:8080', ws: true } } },
  test: { environment: 'jsdom', setupFiles: ['./src/test/setup.ts'], exclude: ['examples/**', 'node_modules/**'] },
})
