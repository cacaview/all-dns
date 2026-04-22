export interface User {
  id: number
  email: string
  role: 'admin' | 'editor' | 'viewer'
  oauthProvider: string
  oauthSubject: string
  oauthInfo: Record<string, unknown>
  tokenVersion: number
  createdAt: string
  updatedAt: string
}

export interface TokenPair {
  accessToken: string
  accessExpiresAt: string
  refreshToken: string
  refreshExpiresAt: string
}

export interface AuthResponse {
  user: User
  tokens: TokenPair
  provider: string
}
