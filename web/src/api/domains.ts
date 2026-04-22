import api from './client'
import type { Backup, BackupListItem, DashboardSummary, DNSRecord, Domain, DomainProfile, PropagationHistoryItem, UploadedAttachment, UpsertRecordResponse } from '../types/domain'

export async function fetchDashboardSummary() {
  const { data } = await api.get<{ item: DashboardSummary }>('/dashboard/summary')
  return data.item
}

export async function listDomains(search = '', includeArchived = false) {
  const { data } = await api.get<{ items: Domain[] }>('/domains', { params: { search, includeArchived } })
  return data.items
}

export async function updateArchive(domainId: number, archived: boolean) {
  const { data } = await api.put<{ item: Domain }>(`/domains/${domainId}/archive`, { archived })
  return data.item
}

export async function listAllBackups(search = '') {
  const { data } = await api.get<{ items: BackupListItem[] }>('/domains/backups', { params: { search } })
  return data.items
}

export async function restoreBackup(backupId: number) {
  const { data } = await api.post<{ item: Backup }>(`/backups/${backupId}/restore`)
  return data.item
}

export async function exportBackup(backupId: number) {
  const { data, headers } = await api.get<ArrayBuffer>(`/backups/${backupId}/export`, {
    responseType: 'arraybuffer',
  })
  const disposition = String(headers['content-disposition'] || '')
  const match = disposition.match(/filename="?([^";]+)"?/i)
  return {
    data,
    filename: match?.[1] || `backup-${backupId}.json`,
  }
}

export async function toggleStar(domainId: number) {
  const { data } = await api.post<{ item: Domain }>(`/domains/${domainId}/star`)
  return data.item
}

export async function updateTags(domainId: number, tags: string[]) {
  const { data } = await api.put<{ item: Domain }>(`/domains/${domainId}/tags`, { tags })
  return data.item
}

export async function listRecords(domainId: number) {
  const { data } = await api.get<{ items: DNSRecord[] }>(`/domains/${domainId}/records`)
  return data.items
}

export async function upsertRecord(domainId: number, payload: DNSRecord) {
  const { data } = await api.post<UpsertRecordResponse>(`/domains/${domainId}/records/upsert`, payload)
  return data
}

export async function deleteRecord(domainId: number, recordId: string) {
  const { data } = await api.post<{ backup: Backup }>(`/domains/${domainId}/records/delete`, { recordId })
  return data.backup
}

export async function listBackups(domainId: number) {
  const { data } = await api.get<{ items: Backup[] }>(`/domains/${domainId}/backups`)
  return data.items
}

export async function listPropagationHistory(domainId?: number) {
  const { data } = await api.get<{ items: PropagationHistoryItem[] }>('/domains/propagation-history', {
    params: domainId ? { domainId } : undefined,
  })
  return data.items
}

export async function fetchProfile(domainId: number) {
  const { data } = await api.get<{ item: DomainProfile }>(`/domains/${domainId}/profile`)
  return data.item
}

export async function updateProfile(domainId: number, payload: Pick<DomainProfile, 'description' | 'attachmentUrls'>) {
  const { data } = await api.put<{ item: DomainProfile }>(`/domains/${domainId}/profile`, payload)
  return data.item
}

export async function uploadProfileAttachment(domainId: number, file: File) {
  const form = new FormData()
  form.append('file', file)
  const { data } = await api.post<{ item: UploadedAttachment }>(`/domains/${domainId}/profile/attachments`, form, {
    headers: { 'Content-Type': 'multipart/form-data' },
  })
  return data.item
}

export async function triggerPropagation(domainId: number, payload: DNSRecord) {
  const { data } = await api.post<{ item: Record<string, any> }>(`/domains/${domainId}/propagation-check`, payload)
  return data.item
}

export interface PropagationWatchOptions {
  resolvers?: string[]
  watch?: boolean
  watchInterval?: number
  watchMaxAttempts?: number
}

export async function triggerPropagationWatch(domainId: number, payload: DNSRecord, opts: PropagationWatchOptions = {}) {
  const { data } = await api.post<{ item: Record<string, any> }>(`/domains/${domainId}/propagation-check`, {
    ...payload,
    ...opts,
  })
  return data.item
}
