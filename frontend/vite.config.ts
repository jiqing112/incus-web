import { svelte } from '@sveltejs/vite-plugin-svelte'
import tailwindcss from '@tailwindcss/vite'
import { defineConfig } from 'vite'
import path from 'node:path'

// https://vite.dev/config/
export default defineConfig({
  plugins: [tailwindcss(), svelte()],
  resolve: {
    alias: {
      $lib: path.resolve('./src/lib'),
    },
  },
  // 仅当以 VITE_MOCK=1 启动 vite 时注入 mock 开关（构建时求值，生产构建恒为 false）
  define: {
    __MOCK__: JSON.stringify(process.env.VITE_MOCK === '1'),
  },
  server: {
    proxy: {
      '/api': {
        target: 'http://127.0.0.1:8080',
        changeOrigin: true,
        ws: true,
      },
    },
  },
  build: {
    // 单面板应用整包交付（go:embed 单二进制），不做代码分割；
    // 前端全量包超过默认 500kB 提示阈值属预期形态
    chunkSizeWarningLimit: 1500,
  },
})
