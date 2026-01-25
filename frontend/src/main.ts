import { createApp } from 'vue'
import { createPinia } from 'pinia'
import piniaPluginPersistedstate from 'pinia-plugin-persistedstate'
import { VueQueryPlugin } from '@tanstack/vue-query'

import App from './App.vue'
import router from './router'
import { initAuth } from './composables/useAuth'

import './styles/index.css'

const app = createApp(App)

// Setup Pinia with persistence
const pinia = createPinia()
pinia.use(piniaPluginPersistedstate)
app.use(pinia)

// Setup Vue Query
app.use(VueQueryPlugin, {
    queryClientConfig: {
        defaultOptions: {
            queries: {
                staleTime: 1000 * 60 * 5, // 5 minutes
                retry: 1,
            },
        },
    },
})

// Initialize auth and mount app
async function initializeApp() {
    try {
        await initAuth()
    } catch (error) {
        console.error('Failed to initialize auth:', error)
    }

    app.use(router)

    await router.isReady()
    app.mount('#app')
}

initializeApp()
