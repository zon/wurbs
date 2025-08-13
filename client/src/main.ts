import './assets/main.css'

import { createApp } from 'vue'
import App from './App.vue'
import './handlers'
import { initRouter } from './router'
import { fatalError } from 'gonf'

async function main() {
  try {
    await load()
  } catch (err) {
    unloadedErrorHandler(err)
  }
}

async function load() {
  const router = initRouter()

  const app = createApp(App)
  app.use(router)
  app.config.errorHandler = errorHandler
  app.mount('#app')
}

function errorHandler(err: unknown) {
  console.error(err)
  if (err instanceof Error) {
    fatalError(err)
  } else {
    fatalError(new Error(String(err)))
  }
}

function unloadedErrorHandler(err: unknown) {
  console.error(err)
  const app = document.getElementById('app')
  const pre = document.createElement('pre')
  pre.textContent = String(err)
  app?.appendChild(pre)
}

main()
