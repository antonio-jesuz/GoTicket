import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'

// O proxy evita CORS em dev e mantém o SSE em mesma origem.
export default defineConfig({
  plugins: [vue()],
  server: { port: 5173, proxy: { '/api': 'http://localhost:8080' } },
})
