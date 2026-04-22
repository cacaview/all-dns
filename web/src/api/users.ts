import api from './client'
import type { User } from '../types/auth'

export function listUsers(): Promise<User[]> {
  return api.get('/users').then((res) => res.data.items)
}

export function updateUserRole(userId: number, role: User['role']): Promise<void> {
  return api.put(`/users/${userId}/role`, { role }).then((res) => res.data)
}
