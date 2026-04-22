import { defineStore } from 'pinia'
import { devLogin, fetchCurrentUser, logout, refreshSession } from '../api/auth'
import type { AuthResponse, TokenPair, User } from '../types/auth'

const ACCESS_TOKEN_KEY = 'dns-hub-access-token'
const REFRESH_TOKEN_KEY = 'dns-hub-refresh-token'

function parseAuthFromHash(): TokenPair | null {
  const hash = window.location.hash.startsWith('#') ? window.location.hash.slice(1) : window.location.hash
  const params = new URLSearchParams(hash)
  const accessToken = params.get('accessToken')
  const refreshToken = params.get('refreshToken')
  if (!accessToken || !refreshToken) {
    return null
  }
  return {
    accessToken,
    refreshToken,
    accessExpiresAt: new Date(Date.now() + 15 * 60 * 1000).toISOString(),
    refreshExpiresAt: new Date(Date.now() + 7 * 24 * 60 * 60 * 1000).toISOString(),
  }
}

export const useAuthStore = defineStore('auth', {
  state: () => ({
    user: null as User | null,
    initialized: false,
    accessToken: localStorage.getItem(ACCESS_TOKEN_KEY) || '',
    refreshToken: localStorage.getItem(REFRESH_TOKEN_KEY) || '',
  }),
  getters: {
    isAuthenticated: (state) => Boolean(state.accessToken && state.user),
  },
  actions: {
    consumeOAuthRedirect() {
      const tokens = parseAuthFromHash()
      if (!tokens) return false
      this.setTokens(tokens)
      window.history.replaceState({}, document.title, window.location.pathname)
      return true
    },
    setTokens(tokens: AuthResponse['tokens']) {
      this.accessToken = tokens.accessToken
      this.refreshToken = tokens.refreshToken
      localStorage.setItem(ACCESS_TOKEN_KEY, tokens.accessToken)
      localStorage.setItem(REFRESH_TOKEN_KEY, tokens.refreshToken)
    },
    clearSession() {
      this.user = null
      this.accessToken = ''
      this.refreshToken = ''
      localStorage.removeItem(ACCESS_TOKEN_KEY)
      localStorage.removeItem(REFRESH_TOKEN_KEY)
    },
    async initialize() {
      if (this.initialized) return
      this.consumeOAuthRedirect()
      if (!this.accessToken && this.refreshToken) {
        await this.refresh()
      }
      if (this.accessToken) {
        try {
          this.user = await fetchCurrentUser()
        } catch {
          if (this.refreshToken) {
            await this.refresh()
            this.user = await fetchCurrentUser()
          } else {
            this.clearSession()
          }
        }
      }
      this.initialized = true
    },
    async refresh() {
      if (!this.refreshToken) {
        this.clearSession()
        return
      }
      try {
        const response = await refreshSession(this.refreshToken)
        this.setTokens(response.tokens)
        this.user = response.user
      } catch {
        this.clearSession()
      }
    },
    async signInDev(email = 'demo@dns-hub.local') {
      const response = await devLogin(email)
      this.setTokens(response.tokens)
      this.user = response.user
      this.initialized = true
    },
    async signOut() {
      try {
        await logout()
      } finally {
        this.clearSession()
      }
    },
  },
})
