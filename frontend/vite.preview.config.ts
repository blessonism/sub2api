// 临时预览专用 Vite 配置：把整页 AppLayout 换成轻量壳，便于在无后端/无容器环境下
// 渲染糖果探针区块与智商监控页面。仅供 preview.html 使用，不影响主构建。
import { defineConfig } from 'vite'
import { resolve } from 'path'
import baseFactory from './vite.config'

const base = (baseFactory as unknown as (env: { mode: string; command: string }) => Record<string, unknown>)({
  mode: 'development',
  command: 'serve',
})

base.resolve = base.resolve ?? {}
base.resolve.alias = [
  // 精确匹配必须排在 '@' 通配之前
  { find: '@/components/layout/AppLayout.vue', replacement: resolve(__dirname, 'src/preview/PreviewLayout.vue') },
  { find: '@', replacement: resolve(__dirname, 'src') },
  { find: 'vue-i18n', replacement: 'vue-i18n/dist/vue-i18n.runtime.esm-bundler.js' },
]

export default defineConfig(base)
