import { test, expect } from './fixtures'
import type { APIRequestContext } from '@playwright/test'

async function pauseTaskRunner(request: APIRequestContext) {
  const res = await request.post('/api/tasks/governance/global', {
    data: { max_running_workspaces: 0 },
  })
  expect(res.ok(), await res.text()).toBeTruthy()
}

async function createTask(request: APIRequestContext, workspace: string, prompt: string) {
  const res = await request.post('/api/tasks', {
    data: { workspace, prompt, title: 'e2e task' },
  })
  expect(res.ok(), await res.text()).toBeTruthy()
  return (await res.json()) as { id: string }
}

test('smoke: secretary bulk-cancel via "都删掉"', async ({ page, request, gotoSecretary }) => {
  const workspace = process.env.ONEAGENT_E2E_WORKSPACE || process.cwd()

  await pauseTaskRunner(request)

  const t1 = await createTask(request, workspace, 'e2e: task 1')
  const t2 = await createTask(request, workspace, 'e2e: task 2')
  const ids = [t1.id, t2.id]

  await gotoSecretary()

  await page.locator('textarea').fill('都删掉')
  await page.getByTestId('chat-send').click()

  await expect(page.getByText(/关掉了|清理\/取消/)).toBeVisible()

  await expect.poll(async () => {
    const res = await request.get(`/api/tasks?workspace=${encodeURIComponent(workspace)}`)
    if (!res.ok()) return false
    const tasks = (await res.json()) as Array<any>
    const byId = new Map(tasks.map((t) => [String(t?.id || ''), t]))
    return ids.every((id) => {
      const task = byId.get(id)
      const attempt = Array.isArray(task?.attempts) && task.attempts.length > 0 ? task.attempts[task.attempts.length - 1] : null
      return String(attempt?.status || '') === 'canceled'
    })
  }).toBe(true)
})
