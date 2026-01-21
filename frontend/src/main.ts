import { createApp } from 'vue'
import { createPinia } from 'pinia'
import piniaPluginPersistedstate from 'pinia-plugin-persistedstate'
import { VueQueryPlugin } from '@tanstack/vue-query'

import App from './App.vue'
import router from './router'
import { initKeycloak } from './composables/useAuth'

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

// Initialize Keycloak and mount app
async function initializeApp() {
    try {
        console.log('Initializing Keycloak...')
        await initKeycloak()
        console.log('Keycloak initialized')
    } catch (error) {
        console.error('Failed to initialize Keycloak:', error)
    }

    // Setup Router (after Keycloak init to avoid clobbering OIDC callback URL)
    app.use(router)

    await router.isReady()
    app.mount('#app')
}

initializeApp()
