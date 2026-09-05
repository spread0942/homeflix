import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'

const apiProxy = process.env.VITE_API_PROXY || 'http://localhost:8080'
const hmrClientPort = Number(process.env.VITE_HMR_CLIENT_PORT || 0) || undefined

export default defineConfig({
  plugins: [vue()],
  server: {
    host: true,
    port: 5173,
    strictPort: true,
    watch: {
      usePolling: true,
      interval: 300,
    },
    hmr: hmrClientPort
      ? {
          protocol: 'ws',
          host: 'localhost',
          clientPort: hmrClientPort,
        }
      : true,
    proxy: {
      '/api': {
        target: apiProxy,
        changeOrigin: true,
      },
    },
  },
})
