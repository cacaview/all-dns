import { describe, it, expect, vi, beforeEach } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'
import { useDomainStore } from './domains'

// Mock the API module
vi.mock('../api/domains', () => ({
  fetchDashboardSummary: vi.fn(),
  listAllBackups: vi.fn(),
  listDomains: vi.fn(),
  listPropagationHistory: vi.fn(),
  listRecords: vi.fn(),
}))

vi.mock('../api/accounts', () => ({
  listAccounts: vi.fn(),
  listReminders: vi.fn(),
  setReminderHandled: vi.fn() as any,
}))

import { fetchDashboardSummary, listDomains } from '../api/domains'
import { listAccounts, listReminders } from '../api/accounts'

const { fetchDashboardSummary: mockFetchDashboardSummary, listDomains: mockListDomains } = vi.mocked({ fetchDashboardSummary, listDomains })
const { listAccounts: mockListAccounts, listReminders: mockListReminders } = vi.mocked({ listAccounts, listReminders })

// Expose pure functions for testing
import {
  normalizeReminders,
  toggleReminder,
  reminderStatusText,
  reminderDetailText,
  reminderActionLabel,
  reminderTagType,
  reminderTimelineType,
} from './domains'

describe('domains store — pure functions', () => {
  const makeReminder = (overrides: Partial<import('../types/domain').Reminder> = {}): import('../types/domain').Reminder =>
    ({
      accountId: 1,
      name: 'Test Account',
      provider: 'cloudflare',
      userId: 1,
      expiresAt: '2025-06-01T00:00:00Z',
      severity: 'warning',
      daysLeft: 7,
      handled: false,
      handledAt: undefined,
      ...overrides,
    } as any)

  describe('normalizeReminders', () => {
    it('sorts unhandled by severity rank descending then daysLeft ascending', () => {
      // severity rank: expired(4) > critical(3) > warning(2) > notice(1)
      const items = [
        makeReminder({ severity: 'notice', daysLeft: 30 }),
        makeReminder({ severity: 'critical', daysLeft: 1 }),
        makeReminder({ severity: 'warning', daysLeft: 5 }),
      ]
      const result = normalizeReminders(items)
      expect(result[0].severity).toBe('critical') // rank 3
      expect(result[1].severity).toBe('warning') // rank 2
      expect(result[2].severity).toBe('notice')   // rank 1
    })
  })

  describe('toggleReminder', () => {
    it('marks a reminder as handled', () => {
      const items = [makeReminder({ accountId: 5, severity: 'critical', daysLeft: 1 })]
      const target = items[0]
      const result = toggleReminder(items, target, true)
      expect(result[0].handled).toBe(true)
    })

    it('unmarks a reminder', () => {
      const items = [makeReminder({ accountId: 5, severity: 'critical', daysLeft: 1, handled: true, handledAt: '2025-01-01T00:00:00Z' })]
      const target = items[0]
      const result = toggleReminder(items, target, false)
      expect(result[0].handled).toBe(false)
    })

    it('returns a new array and does not mutate the original', () => {
      const r1 = makeReminder({ accountId: 5, severity: 'critical', daysLeft: 1 })
      const original = [r1]
      const result = toggleReminder(original, r1, true)
      expect(result).not.toBe(original)
      expect(original[0].handled).toBe(false) // original not mutated
      expect(result[0].handled).toBe(true)   // result is updated
    })
  })

  describe('reminderStatusText', () => {
    it('returns 已处理 for handled reminders', () => {
      const r = makeReminder({ handled: true })
      expect(reminderStatusText(r)).toBe('已处理')
    })

    it('returns severity label for unhandled reminders', () => {
      const r = makeReminder({ severity: 'warning', handled: false })
      expect(reminderStatusText(r)).toBe('警告')
    })
  })

  describe('reminderDetailText', () => {
    it('returns handled timestamp when handled', () => {
      const r = makeReminder({ handled: true, handledAt: '2025-03-01T10:00:00Z' })
      expect(reminderDetailText(r)).toContain('2025')
    })

    it('returns severity text when not handled', () => {
      const r = makeReminder({ severity: 'critical', daysLeft: 1, handled: false })
      expect(reminderDetailText(r)).toContain('距离过期')
    })
  })

  describe('reminderActionLabel', () => {
    it('returns 恢复未处理 when handled', () => {
      const r = makeReminder({ handled: true })
      expect(reminderActionLabel(r)).toBe('恢复未处理')
    })

    it('returns 标记已处理 when not handled', () => {
      const r = makeReminder({ handled: false })
      expect(reminderActionLabel(r)).toBe('标记已处理')
    })
  })

  describe('reminderTagType', () => {
    it('returns info for handled', () => {
      expect(reminderTagType(makeReminder({ handled: true }))).toBe('info')
    })

    it('returns danger for expired/critical unhandled', () => {
      expect(reminderTagType(makeReminder({ severity: 'expired', handled: false }))).toBe('danger')
      expect(reminderTagType(makeReminder({ severity: 'critical', handled: false }))).toBe('danger')
    })

    it('returns warning for warning severity', () => {
      expect(reminderTagType(makeReminder({ severity: 'warning', handled: false }))).toBe('warning')
    })

    it('returns primary for notice', () => {
      expect(reminderTagType(makeReminder({ severity: 'notice', handled: false }))).toBe('primary')
    })
  })

  describe('reminderTimelineType', () => {
    it('returns danger for expired/critical', () => {
      expect(reminderTimelineType('expired')).toBe('danger')
      expect(reminderTimelineType('critical')).toBe('danger')
    })

    it('returns warning for warning', () => {
      expect(reminderTimelineType('warning')).toBe('warning')
    })

    it('returns primary for notice', () => {
      expect(reminderTimelineType('notice')).toBe('primary')
    })
  })
})

describe('useDomainStore', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.clearAllMocks()
    localStorage.clear()
  })

  it('loadDashboard populates summary, accounts, reminders, domains', async () => {
    const store = useDomainStore()
    const mockSummary = { accounts: 3, domains: 10, starredDomains: 2, expiringAccounts: 1 } as any
    const mockAccounts = [{ id: 1, name: 'CF' }] as any[]
    const mockReminders = [{ accountId: 1, severity: 'warning', daysLeft: 5, handled: false }] as any[]
    const mockDomains = [{ id: 1, name: 'example.com' }] as any[]

    mockFetchDashboardSummary.mockResolvedValueOnce(mockSummary)
    mockListAccounts.mockResolvedValueOnce(mockAccounts)
    mockListReminders.mockResolvedValueOnce(mockReminders)
    mockListDomains.mockResolvedValueOnce(mockDomains)

    await store.loadDashboard()

    expect(store.summary).toEqual(mockSummary)
    expect(store.accounts).toEqual(mockAccounts)
    expect(store.reminders[0].severity).toBe('warning')
    expect(store.domains).toEqual(mockDomains)
  })

  it('loadDomains updates search and loading state', async () => {
    const store = useDomainStore()
    mockListDomains.mockResolvedValueOnce([])

    await store.loadDomains('example', false)

    expect(store.search).toBe('example')
    expect(store.includeArchived).toBe(false)
    expect(store.loading).toBe(false)
  })

  it('setSelectedDomain updates selectedDomain', () => {
    const store = useDomainStore()
    const domain = { id: 1, name: 'test.com' } as any
    store.setSelectedDomain(domain)
    expect(store.selectedDomain).toEqual(domain)
  })

  it('domainLastPropagationStatus updates domain and selectedDomain', () => {
    const store = useDomainStore()
    store.domains = [{ id: 5, name: 'test.com', lastPropagationStatus: {} }] as any
    store.selectedDomain = { id: 5, name: 'test.com' } as any
    const status = { resolved: true } as any

    store.domainLastPropagationStatus(5, status)

    expect(store.domains[0].lastPropagationStatus).toEqual(status)
    expect(store.selectedDomain.lastPropagationStatus).toEqual(status)
  })
})
