import { describe, it, expect, vi, beforeEach } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'
import { useAuthStore } from './auth'

// Mock the API module
vi.mock('../api/auth', () => ({
  devLogin: vi.fn(),
  fetchCurrentUser: vi.fn(),
  logout: vi.fn(),
  refreshSession: vi.fn(),
}))

import { devLogin, fetchCurrentUser, logout, refreshSession } from '../api/auth'

const { devLogin: mockDevLogin, fetchCurrentUser: mockFetchCurrentUser, logout: mockLogout, refreshSession: mockRefreshSession } = vi.mocked({ devLogin, fetchCurrentUser, logout, refreshSession })

describe('useAuthStore', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    localStorage.clear()
    vi.clearAllMocks()
  })

  it('initializes with empty token and user', () => {
    const store = useAuthStore()
    expect(store.accessToken).toBe('')
    expect(store.user).toBeNull()
    expect(store.isAuthenticated).toBe(false)
  })

  it('setTokens stores tokens in state and localStorage', () => {
    const store = useAuthStore()
    const tokens = {
      accessToken: 'access123',
      refreshToken: 'refresh456',
      accessExpiresAt: '2025-01-01T00:00:00Z',
      refreshExpiresAt: '2025-01-08T00:00:00Z',
    }
    store.setTokens(tokens)
    expect(store.accessToken).toBe('access123')
    expect(store.refreshToken).toBe('refresh456')
    expect(localStorage.getItem('dns-hub-access-token')).toBe('access123')
    expect(localStorage.getItem('dns-hub-refresh-token')).toBe('refresh456')
  })

  it('clearSession wipes state and localStorage', () => {
    const store = useAuthStore()
    store.accessToken = 'token'
    store.user = { id: 1, email: 'test@test.com', role: 'admin' } as any
    localStorage.setItem('dns-hub-access-token', 'token')
    localStorage.setItem('dns-hub-refresh-token', 'refresh')

    store.clearSession()

    expect(store.accessToken).toBe('')
    expect(store.user).toBeNull()
    expect(localStorage.getItem('dns-hub-access-token')).toBeNull()
    expect(localStorage.getItem('dns-hub-refresh-token')).toBeNull()
  })

  it('consumeOAuthRedirect parses tokens from URL hash', () => {
    const store = useAuthStore()
    // Simulate URL with hash tokens
    Object.defineProperty(window, 'location', {
      value: { hash: '#accessToken=abc&refreshToken=def' },
      writable: true,
    })
    window.history.replaceState = vi.fn()

    const result = store.consumeOAuthRedirect()

    expect(result).toBe(true)
    expect(store.accessToken).toBe('abc')
    expect(store.refreshToken).toBe('def')
  })

  it('consumeOAuthRedirect returns false when no tokens in hash', () => {
    const store = useAuthStore()
    Object.defineProperty(window, 'location', {
      value: { hash: '' },
      writable: true,
    })

    const result = store.consumeOAuthRedirect()

    expect(result).toBe(false)
  })

  it('signInDev calls devLogin and sets tokens', async () => {
    const store = useAuthStore()
    const mockUser = { id: 1, email: 'demo@dns-hub.local', role: 'admin' } as any
    mockDevLogin.mockResolvedValueOnce({
      user: mockUser,
      tokens: {
        accessToken: 'dev-access',
        refreshToken: 'dev-refresh',
        accessExpiresAt: '',
        refreshExpiresAt: '',
      },
    })

    await store.signInDev()

    expect(mockDevLogin).toHaveBeenCalledWith('demo@dns-hub.local')
    expect(store.user).toEqual(mockUser)
    expect(store.accessToken).toBe('dev-access')
    expect(store.initialized).toBe(true)
  })

  it('signOut calls logout and clears session', async () => {
    const store = useAuthStore()
    store.user = { id: 1 } as any
    store.accessToken = 'token'
    mockLogout.mockResolvedValueOnce(undefined)

    await store.signOut()

    expect(mockLogout).toHaveBeenCalled()
    expect(store.user).toBeNull()
    expect(store.accessToken).toBe('')
  })

  it('refresh calls refreshSession and updates tokens', async () => {
    const store = useAuthStore()
    store.refreshToken = 'old-refresh'
    const mockUser = { id: 1, email: 'test@test.com' } as any
    mockRefreshSession.mockResolvedValueOnce({
      user: mockUser,
      tokens: {
        accessToken: 'new-access',
        refreshToken: 'new-refresh',
        accessExpiresAt: '',
        refreshExpiresAt: '',
      },
    })

    await store.refresh()

    expect(mockRefreshSession).toHaveBeenCalledWith('old-refresh')
    expect(store.accessToken).toBe('new-access')
    expect(store.user).toEqual(mockUser)
  })

  it('refresh clears session when refreshToken is missing', async () => {
    const store = useAuthStore()
    store.refreshToken = ''
    store.accessToken = 'some-token'

    await store.refresh()

    expect(store.accessToken).toBe('')
    expect(mockRefreshSession).not.toHaveBeenCalled()
  })
})
