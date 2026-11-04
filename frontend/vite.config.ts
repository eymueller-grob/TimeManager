import {defineConfig} from 'vite'
import react from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'
import {readFileSync} from 'node:fs'
import {fileURLToPath, URL} from 'node:url'

const wailsConfig: unknown = JSON.parse(readFileSync(new URL('../wails.json', import.meta.url), 'utf8'))
const version = typeof wailsConfig === 'object' && wailsConfig !== null && 'info' in wailsConfig
  && typeof wailsConfig.info === 'object' && wailsConfig.info !== null && 'productVersion' in wailsConfig.info
  && typeof wailsConfig.info.productVersion === 'string' ? wailsConfig.info.productVersion : ''
if (!version) throw new Error('wails.json must define info.productVersion')

// https://vitejs.dev/config/
export default defineConfig({
  plugins: [react(), tailwindcss()],
  define: {__APP_VERSION__: JSON.stringify(version)},
  resolve: {alias: {'@': fileURLToPath(new URL('./src', import.meta.url))}}
})