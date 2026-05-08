// Plugins
import vue from '@vitejs/plugin-vue'
import AutoImport from 'unplugin-auto-import/vite'
import Components from 'unplugin-vue-components/vite'
import { ElementPlusResolver } from 'unplugin-vue-components/resolvers'

// Utilities
import { defineConfig } from 'vite'
import { fileURLToPath, URL } from 'node:url'
import { randomBytes } from 'crypto'

function getUniqueFileName(template: string) {
  if (template.includes('.js') || template.includes('.css')) {
    const hash = randomBytes(8).toString('hex')
    return template.replace('[name]', hash)
  }
  return template
}

export default defineConfig({
  base: '',
  plugins: [
    vue(),
    AutoImport({
      imports: ['vue', 'vue-router', 'vue-i18n'],
      resolvers: [ElementPlusResolver({ importStyle: 'css' })],
      dts: 'src/auto-imports.d.ts',
    }),
    Components({
      resolvers: [ElementPlusResolver({ importStyle: 'css' })],
      dts: 'src/components.d.ts',
      dirs: [], // we manage component imports explicitly; only EP via resolver
    }),
  ],
  build: {
    manifest: false,
    outDir: 'dist',
    chunkSizeWarningLimit: 2000,
    rollupOptions: {
      output: {
        entryFileNames: getUniqueFileName('assets/[name].js'),
        chunkFileNames: getUniqueFileName('assets/[name].js'),
        assetFileNames: (assetInfo: any) => {
          const names: string[] = assetInfo.names || (assetInfo.name ? [assetInfo.name] : [])
          if (names.some((name) => name.endsWith('.css'))) {
            return getUniqueFileName('assets/[name].css')
          }
          return 'assets/' + (names[0] ?? 'asset')
        },
      },
    },
  },
  define: { 'process.env': {} },
  resolve: {
    alias: {
      '@': fileURLToPath(new URL('./src', import.meta.url)),
    },
    extensions: ['.js', '.json', '.jsx', '.mjs', '.ts', '.tsx', '.vue'],
  },
  server: {
    port: 3000,
    proxy: {
      '/app/api': {
        target: 'http://localhost:2095',
        changeOrigin: true,
      },
    },
  },
})
