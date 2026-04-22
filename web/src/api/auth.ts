import api from './client'
import type { AuthResponse } from '../types/auth'

export async function fetchCurrentUser() {
  const { data } = await api.get<{ user: AuthResponse['user'] }>('/auth/me')
  return data.user
}

export async function refreshSession(refreshToken: string) {
  const { data } = await api.post<AuthResponse>('/auth/refresh', { refreshToken })
  return data
}

export async function devLogin(email = 'demo@dns-hub.local') {
  const { data } = await api.post<AuthResponse>('/auth/dev-login', { email })
  return data
}

export async function logout() {
  await api.post('/auth/logout')
}
