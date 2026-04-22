export interface Account {
  id: number
  name: string
  provider: string
  status: string
  credentialStatus: string
  expiresAt?: string
  lastCheckedAt?: string
  lastRotatedAt?: string
  lastValidationError: string
  domainCount: number
  reminder: string
}

export interface AccountRotationPayload {
  name?: string
  config: Record<string, any>
  expiresAt?: string
  status?: string
}

export interface ValidationResult {
  ok: boolean
  message: string
  checkedAt: string
}

export interface AccountRotationResponse {
  item: Account
  validation?: ValidationResult
}

export interface AccountUpdatePayload {
  name?: string
  expiresAt?: string
  status?: string
}

export interface Reminder {
  accountId: number
  name: string
  provider: string
  userId: number
  expiresAt?: string
  severity: string
  daysLeft: number
  handled?: boolean
  handledAt?: string
}

export type ProviderFieldType = 'text' | 'password' | 'number' | 'boolean'

export interface ProviderFieldSpec {
  key: string
  label: string
  type: ProviderFieldType
  required: boolean
  placeholder?: string
  helpText?: string
  defaultValue?: any
}

export interface ProviderDescriptor {
  key: string
  label: string
  description?: string
  fields: ProviderFieldSpec[]
  sampleConfig?: Record<string, any>
}

export interface AccountPayload {
  name: string
  provider: string
  config: Record<string, any>
  expiresAt?: string
  status?: string
}

export interface PropagationResult {
  resolver: string
  status: string
  answers: string[]
  matched: boolean
  reason: string
}

export interface PropagationHistoryItem {
  id: number
  domainId: number
  fqdn: string
  record: Record<string, any>
  overallStatus: string
  summary: string
  matchedCount: number
  failedCount: number
  pendingCount: number
  totalResolvers: number
  results: PropagationResult[]
  checkedAt: string
  createdAt: string
}

export interface PropagationStatus {
  checkedAt?: string
  fqdn?: string
  matchedResolvers?: string[]
  failedResolvers?: string[]
  pendingResolvers?: string[]
  matchedCount?: number
  failedCount?: number
  pendingCount?: number
  totalResolvers?: number
  isFullyPropagated?: boolean
  overallStatus?: string
  summary?: string
  results?: PropagationResult[]
}

export interface Domain {
  id: number
  accountId: number
  accountName: string
  provider: string
  name: string
  providerZoneId: string
  isStarred: boolean
  isArchived: boolean
  archivedAt?: string
  tags: string[]
  lastSyncedAt?: string
  lastPropagationStatus: PropagationStatus
  createdAt: string
  updatedAt: string
}

export interface BackupListItem extends Backup {
  domainName: string
  accountName: string
  provider: string
  recordCount: number
  restoreLabel?: string
}

export interface DNSRecord {
  id: string
  type: string
  name: string
  content: string
  ttl: number
  priority?: number
  proxied?: boolean
  comment?: string
}

export interface DashboardSummary {
  accounts: number
  domains: number
  starredDomains: number
  expiringAccounts: number
}

export interface Backup {
  id: number
  domainId: number
  triggeredByUserId: number
  reason: string
  content: Record<string, any>
  createdAt: string
}

export interface UploadedAttachment {
  name: string
  url: string
}

export interface DomainProfile {
  id: number
  domainId: number
  description: string
  attachmentUrls: string[]
  createdAt: string
  updatedAt: string
}

export interface UpsertRecordResponse {
  item: DNSRecord
  backup: Backup
  propagation: PropagationStatus
}
