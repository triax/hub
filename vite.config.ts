import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

const apiBase = `http://localhost:${process.env.VITE_API_PORT ?? '8080'}`

export default defineConfig({
  plugins: [react()],
  root: 'client',
  build: {
    outDir: 'dest',
    emptyOutDir: true,
  },
  test: {
    passWithNoTests: true,
  },
  server: {
    port: parseInt(process.env.VITE_PORT ?? '3000'),
    proxy: {
      '/api':    apiBase,
      '/auth':   apiBase,
      '/login':  apiBase,
      '/logout': apiBase,
    },
  },
})
