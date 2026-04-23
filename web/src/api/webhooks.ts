import api from './client'

export interface Webhook {
  id: number
  orgId: number
  name: string
  url: string
  events: string[]
  active: boolean
  createdAt: string
  updatedAt: string
}

export interface CreateWebhookPayload {
  name: string
  url: string
  events?: string[]
}

export interface UpdateWebhookPayload {
  name?: string
  url?: string
  events?: string[]
  active?: boolean
}

export async function listWebhooks() {
  const { data } = await api.get<{ items: Webhook[] }>('/webhooks')
  return data.items
}

export async function createWebhook(payload: CreateWebhookPayload) {
  const { data } = await api.post<{ item: Webhook }>('/webhooks', payload)
  return data.item
}

export async function updateWebhook(id: number, payload: UpdateWebhookPayload) {
  const { data } = await api.put<{ item: Webhook }>(`/webhooks/${id}`, payload)
  return data.item
}

export async function deleteWebhook(id: number) {
  await api.delete(`/webhooks/${id}`)
}
