// Plugins
import elementPlus from './element-plus'

// Types
import type { App } from 'vue'

export function registerPlugins(app: App) {
  app.use(elementPlus)
}
