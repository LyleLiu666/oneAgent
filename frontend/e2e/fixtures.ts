import { test as base, expect, type Page } from '@playwright/test'

type TestFixtures = {
  gotoSecretary: () => Promise<void>
}

export const test = base.extend<TestFixtures>({
  page: async ({ page }, use) => {
    const errors: string[] = []
    page.on('pageerror', (err) => errors.push(err.message))
    await use(page)
    expect(errors, `page errors:\\n${errors.join('\\n')}`).toEqual([])
  },
  gotoSecretary: async ({ page }, use) => {
    const gotoSecretary = async () => {
      await page.goto('/secretary')
      await expect(page.getByTestId('chat-send')).toBeVisible()
    }
    await use(gotoSecretary)
  },
})

export { expect, type Page }

