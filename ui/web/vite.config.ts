import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

// Em desenvolvimento (npm run dev) as chamadas /api vão para o ui/server em :8080.
// Em produção o ui/server serve o build de dist/ e a API na mesma origem.
export default defineConfig({
  plugins: [react()],
  server: {
    proxy: { '/api': 'http://localhost:8080' },
  },
})
