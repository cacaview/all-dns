import { defineStore } from 'pinia'
import { listAccounts, listReminders, setReminderHandled as apiSetReminderHandled } from '../api/accounts'
import { fetchDashboardSummary, listAllBackups, listDomains, listPropagationHistory } from '../api/domains'
import type { Account, BackupListItem, DashboardSummary, DNSRecord, Domain, PropagationHistoryItem, PropagationStatus, Reminder } from '../types/domain'

const HANDLED_REMINDER_KEY = 'dns-hub-handled-reminders'

type HandledReminderState = Record<string, string>

function readHandledReminderState(): HandledReminderState {
  try {
    const raw = localStorage.getItem(HANDLED_REMINDER_KEY)
    if (!raw) return {}
    const parsed = JSON.parse(raw) as string[] | Record<string, string>
    if (Array.isArray(parsed)) {
      return Object.fromEntries(parsed.map((key) => [key, '']))
    }
    return Object.fromEntries(
      Object.entries(parsed || {}).filter(([key]) => Boolean(key)).map(([key, value]) => [key, typeof value === 'string' ? value : '']),
    )
  } catch {
    return {}
  }
}

function writeHandledReminderState(state: HandledReminderState) {
  localStorage.setItem(HANDLED_REMINDER_KEY, JSON.stringify(state))
}

function reminderKey(item: Reminder) {
  return `${item.accountId}:${item.severity}:${item.expiresAt || ''}`
}

function withHandledState(items: Reminder[]) {
  const handledState = readHandledReminderState()
  const activeKeys = new Set(items.map(reminderKey))
  const retainedState = Object.fromEntries(Object.entries(handledState).filter(([key]) => activeKeys.has(key)))
  writeHandledReminderState(retainedState)
  return items.map((item) => {
    const key = reminderKey(item)
    const handledAt = retainedState[key] || undefined
    return {
      ...item,
      handled: Boolean(handledAt),
      handledAt,
    }
  })
}

function setReminderHandled(item: Reminder, handled: boolean) {
  const state = readHandledReminderState()
  const key = reminderKey(item)
  if (handled) state[key] = new Date().toISOString()
  else delete state[key]
  writeHandledReminderState(state)
  return state[key]
}

function updateReminder(items: Reminder[], target: Reminder, handled: boolean) {
  const handledAt = setReminderHandled(target, handled)
  return items.map((item) =>
    reminderKey(item) === reminderKey(target)
      ? {
          ...item,
          handled,
          handledAt,
        }
      : item,
  )
}

function severityRank(severity: string) {
  if (severity === 'expired') return 4
  if (severity === 'critical') return 3
  if (severity === 'warning') return 2
  if (severity === 'notice') return 1
  return 0
}

function sortReminders(items: Reminder[]) {
  return [...items].sort((a, b) => {
    if (Boolean(a.handled) !== Boolean(b.handled)) return a.handled ? 1 : -1
    if (severityRank(a.severity) !== severityRank(b.severity)) return severityRank(b.severity) - severityRank(a.severity)
    return a.daysLeft - b.daysLeft
  })
}

function normalizeReminders(items: Reminder[]) {
  return sortReminders(withHandledState(items))
}

function toggleReminder(items: Reminder[], target: Reminder, handled: boolean) {
  return sortReminders(updateReminder(items, target, handled))
}

function reminderSeverityLabel(severity: string) {
  if (severity === 'expired') return '已过期'
  if (severity === 'critical') return '紧急'
  if (severity === 'warning') return '警告'
  if (severity === 'notice') return '提醒'
  return severity
}

function reminderSeverityText(item: Reminder) {
  if (item.severity === 'expired') return '凭证已过期'
  if (item.severity === 'critical') return `距离过期 ${item.daysLeft} 天`
  if (item.severity === 'warning') return `即将过期，剩余 ${item.daysLeft} 天`
  if (item.severity === 'notice') return `建议关注，剩余 ${item.daysLeft} 天`
  return reminderSeverityLabel(item.severity)
}

function reminderStatusText(item: Reminder) {
  return item.handled ? '已处理' : reminderSeverityLabel(item.severity)
}

function reminderDetailText(item: Reminder) {
  if (item.handled) {
    return item.handledAt ? `已于 ${new Date(item.handledAt).toLocaleString()} 处理` : '该提醒已处理'
  }
  return reminderSeverityText(item)
}

function reminderActionLabel(item: Reminder) {
  return item.handled ? '恢复未处理' : '标记已处理'
}

function reminderTagType(item: Reminder) {
  if (item.handled) return 'info'
  if (item.severity === 'expired' || item.severity === 'critical') return 'danger'
  if (item.severity === 'warning') return 'warning'
  return 'primary'
}

function reminderTimelineType(severity: string) {
  if (severity === 'expired' || severity === 'critical') return 'danger'
  if (severity === 'warning') return 'warning'
  return 'primary'
}

export {
  normalizeReminders,
  toggleReminder,
  reminderStatusText,
  reminderDetailText,
  reminderActionLabel,
  reminderTagType,
  reminderTimelineType,
}

export const useDomainStore = defineStore('domains', {
  state: () => ({
    accounts: [] as Account[],
    reminders: [] as Reminder[],
    domains: [] as Domain[],
    backups: [] as BackupListItem[],
    propagationHistory: [] as PropagationHistoryItem[],
    selectedDomain: null as Domain | null,
    summary: null as DashboardSummary | null,
    loading: false,
    search: '',
    includeArchived: false,
  }),
  actions: {
    async loadDashboard() {
      this.summary = await fetchDashboardSummary()
      this.accounts = await listAccounts()
      this.reminders = normalizeReminders(await listReminders())
      this.domains = await listDomains(this.search, this.includeArchived)
    },
    async loadDomains(search?: string, includeArchived?: boolean) {
      const keyword = search ?? this.search
      this.search = keyword
      this.includeArchived = includeArchived ?? this.includeArchived
      this.loading = true
      try {
        this.domains = await listDomains(keyword, this.includeArchived)
      } finally {
        this.loading = false
      }
    },
    async loadBackups(search = '') {
      this.backups = await listAllBackups(search)
    },
    async loadPropagationHistory(domainId?: number) {
      this.propagationHistory = await listPropagationHistory(domainId)
    },
    async fetchDomainRecords(domainId: number): Promise<DNSRecord[]> {
      const { listRecords } = await import('../api/domains')
      return listRecords(domainId)
    },
    setSelectedDomain(domain: Domain | null) {
      this.selectedDomain = domain
    },
    domainLastPropagationStatus(domainId: number, status: PropagationStatus) {
      const domain = this.domains.find((d) => d.id === domainId)
      if (domain) {
        domain.lastPropagationStatus = status
      }
      if (this.selectedDomain?.id === domainId) {
        this.selectedDomain.lastPropagationStatus = status
      }
    },
    async setReminderHandled(reminder: Reminder, handled: boolean) {
      // Optimistic update
      this.reminders = toggleReminder(this.reminders, reminder, handled)
      try {
        await apiSetReminderHandled(reminder.accountId, handled)
      } catch (e) {
        // Revert on failure
        this.reminders = toggleReminder(this.reminders, reminder, !handled)
        throw e
      }
    },
    async hydrate() {
      await Promise.all([this.loadDashboard(), this.loadDomains('', this.includeArchived)])
    },
  },
})
