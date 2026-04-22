import api from './client'
import type {
  Account,
  AccountPayload,
  AccountRotationPayload,
  AccountRotationResponse,
  AccountUpdatePayload,
  ProviderDescriptor,
  Reminder,
  ValidationResult,
} from '../types/domain'

export async function listProviders() {
  const { data } = await api.get<{ items: ProviderDescriptor[] }>('/accounts/providers')
  return data.items
}


export async function listAccounts() {
  const { data } = await api.get<{ items: Account[] }>('/accounts')
  return data.items
}

export async function createAccount(payload: AccountPayload) {
  const { data } = await api.post<{ item: Account }>('/accounts', payload)
  return data.item
}

export async function updateAccount(id: number, payload: AccountUpdatePayload) {
  const { data } = await api.put<{ item: Account }>(`/accounts/${id}`, payload)
  return data.item
}

export async function validateAccount(id: number) {
  const { data } = await api.post<{ item: ValidationResult }>(`/accounts/${id}/validate`)
  return data.item
}

export async function rotateAccountCredentials(id: number, payload: AccountRotationPayload) {
  const { data } = await api.post<AccountRotationResponse>(`/accounts/${id}/rotate`, payload)
  return data
}

export async function listReminders() {
  const { data } = await api.get<{ items: Reminder[] }>('/accounts/reminders')
  return data.items
}

export async function setReminderHandled(accountId: number, handled: boolean) {
  await api.put(`/accounts/${accountId}/reminder-handled`, { handled })
}
