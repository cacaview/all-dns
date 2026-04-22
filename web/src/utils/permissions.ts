import type { User } from '../types/auth'

export function canEdit(user?: User | null): boolean {
  if (!user) return false
  return user.role === 'admin' || user.role === 'editor'
}
