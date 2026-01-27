// @vitest-environment jsdom

import { expect, it, vi } from 'vitest'
import { shallowMount } from '@vue/test-utils'
import { ref } from 'vue'

vi.mock('vue-router', () => ({
  useRouter: () => ({
    currentRoute: ref({ query: {} }),
    push: vi.fn(async () => {}),
  }),
}))

vi.mock('@/composables/useAuth', () => ({
  useAuth: () => ({ authMode: ref('token') }),
  loginWithToken: vi.fn(async () => true),
}))

vi.mock('@/stores/auth', () => ({
  useAuthStore: () => ({ isAuthenticated: false }),
}))

it('shows LAN risk warning on login page', async () => {
  const { default: Login } = await import('@/views/Login.vue')
  const wrapper = shallowMount(Login)

  expect(wrapper.text()).toContain('仅建议在可信局域网内使用。将服务暴露到公网风险极大。')
  wrapper.unmount()
})

