// 临时预览入口：不启动后端容器，用内存假后端渲染糖果探针区块与智商监控页面。
// 访问 /preview.html 使用；属开发辅助文件，不进主构建、不提交。
import { createApp } from 'vue'
import { createPinia } from 'pinia'
import i18n, { initI18n } from '@/i18n'
import PreviewRoot from './PreviewRoot.vue'
import { installPreviewMockBackend } from './mockBackend'
import '@/style.css'

installPreviewMockBackend()

async function bootstrap() {
  document.documentElement.classList.toggle(
    'dark',
    localStorage.getItem('theme') === 'dark' ||
      (!localStorage.getItem('theme') && window.matchMedia('(prefers-color-scheme: dark)').matches),
  )
  const app = createApp(PreviewRoot)
  app.use(createPinia())
  await initI18n()
  app.use(i18n)
  app.mount('#app')
}

void bootstrap()
