import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

const apiBase = `http://localhost:${process.env.VITE_API_PORT ?? '8080'}`

export default defineConfig({
  plugins: [react()],
  root: 'client',
  // minify 時も関数名・クラス名を保持する。これによりエラー報告（observability）の
  // JS スタックと React コンポーネントスタックが本番でも実名で読める（犯人特定に有効）。
  esbuild: {
    keepNames: true,
  },
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
