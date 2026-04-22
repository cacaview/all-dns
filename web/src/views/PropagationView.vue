<template>
  <AppLayout>
    <el-card>
      <template #header>
        <div class="toolbar">
          <div>
            <div class="domain-title">{{ pageTitle }}</div>
            <div class="domain-subtitle">{{ pageSubtitle }}</div>
          </div>
          <el-space wrap>
            <el-button @click="backToDomains">域名列表</el-button>
            <el-button type="primary" @click="reload">刷新</el-button>
            <el-button v-if="!overviewMode" type="warning" :loading="watching" @click="toggleWatch">
              {{ watching ? '停止监控' : '持续监控' }}
            </el-button>
          </el-space>
        </div>
      </template>

      <template v-if="overviewMode">
        <el-empty v-if="!overviewDomains.length && !historyRows.length" description="暂无传播数据" />
        <template v-else>
          <el-row :gutter="16" class="summary-row">
            <el-col :md="6" :sm="12" :xs="24">
              <el-statistic title="监控域名" :value="overviewDomainCount" />
            </el-col>
            <el-col :md="6" :sm="12" :xs="24">
              <el-statistic title="已完成" :value="verifiedDomainCount" />
            </el-col>
            <el-col :md="6" :sm="12" :xs="24">
              <el-statistic title="待关注" :value="attentionDomainCount" />
            </el-col>
            <el-col :md="6" :sm="12" :xs="24">
              <el-statistic title="最近检查" :value="historyRows.length" />
            </el-col>
          </el-row>

          <el-card shadow="never" class="summary-card">
            <template #header>
              <div class="history-header">
                <span>域名状态总览</span>
                <span class="history-subtitle">汇总各域名最近一次传播检查结果</span>
              </div>
            </template>
            <el-empty v-if="!overviewDomainRows.length" description="暂无域名数据" />
            <el-table v-else :data="overviewDomainRows" border>
              <el-table-column prop="name" label="域名" min-width="220" />
              <el-table-column prop="provider" label="Provider" width="130" />
              <el-table-column prop="accountName" label="账户" min-width="160" />
              <el-table-column label="最近状态" width="140">
                <template #default="{ row }">
                  <el-tag :type="historyTagType(row.lastPropagationStatus?.overallStatus)">
                    {{ historyStatusLabel(row.lastPropagationStatus?.overallStatus) }}
                  </el-tag>
                </template>
              </el-table-column>
              <el-table-column label="结果摘要" min-width="260">
                <template #default="{ row }">
                  {{ row.lastPropagationStatus?.summary || '暂无传播检查结果' }}
                </template>
              </el-table-column>
              <el-table-column label="最近检查" min-width="180">
                <template #default="{ row }">
                  {{ formatDateTime(row.lastPropagationStatus?.checkedAt) }}
                </template>
              </el-table-column>
              <el-table-column label="解析结果" min-width="200">
                <template #default="{ row }">
                  <span>
                    已命中 {{ row.lastPropagationStatus?.matchedCount || 0 }} / 待生效 {{ row.lastPropagationStatus?.pendingCount || 0 }} /
                    异常 {{ row.lastPropagationStatus?.failedCount || 0 }}
                  </span>
                </template>
              </el-table-column>
              <el-table-column label="操作" width="120" fixed="right">
                <template #default="{ row }">
                  <el-button size="small" @click="openDomainPropagation(row.id)">查看</el-button>
                </template>
              </el-table-column>
            </el-table>
          </el-card>

          <el-card shadow="never" class="history-card">
            <template #header>
              <div class="history-header">
                <span>最近传播历史</span>
                <span class="history-subtitle">跨域名保留最近 50 次检查记录</span>
              </div>
            </template>
            <el-empty v-if="!historyRows.length" description="暂无历史记录" />
            <el-table v-else :data="historyRows" border max-height="420">
              <el-table-column label="检查时间" min-width="180">
                <template #default="{ row }">{{ formatDateTime(row.checkedAt) }}</template>
              </el-table-column>
              <el-table-column label="域名" min-width="220">
                <template #default="{ row }">{{ domainNameById(row.domainId) }}</template>
              </el-table-column>
              <el-table-column prop="fqdn" label="FQDN" min-width="220" />
              <el-table-column label="状态" width="120">
                <template #default="{ row }">
                  <el-tag :type="historyTagType(row.overallStatus)">{{ historyStatusLabel(row.overallStatus) }}</el-tag>
                </template>
              </el-table-column>
              <el-table-column prop="summary" label="摘要" min-width="260" />
              <el-table-column label="操作" width="180" fixed="right">
                <template #default="{ row }">
                  <el-space>
                    <el-button size="small" @click="inspectHistory(row)">明细</el-button>
                    <el-button size="small" type="primary" plain @click="openDomainPropagation(row.domainId)">域名</el-button>
                  </el-space>
                </template>
              </el-table-column>
            </el-table>
          </el-card>
        </template>
      </template>

      <el-empty v-else-if="!domain" description="未找到域名" />
      <template v-else>
        <el-row :gutter="16" class="summary-row">
          <el-col :md="6" :sm="12" :xs="24">
            <el-statistic title="整体状态" :value="summaryLabel" />
          </el-col>
          <el-col :md="6" :sm="12" :xs="24">
            <el-statistic title="已命中" :value="matchedCount" />
          </el-col>
          <el-col :md="6" :sm="12" :xs="24">
            <el-statistic title="待生效" :value="pendingCount" />
          </el-col>
          <el-col :md="6" :sm="12" :xs="24">
            <el-statistic title="异常" :value="failedCount" />
          </el-col>
        </el-row>

        <el-descriptions :column="2" border class="summary-card">
          <el-descriptions-item label="记录 FQDN">{{ status?.fqdn || '—' }}</el-descriptions-item>
          <el-descriptions-item label="最近检查时间">{{ checkedAtLabel }}</el-descriptions-item>
          <el-descriptions-item label="域名">{{ domain.name }}</el-descriptions-item>
          <el-descriptions-item label="Provider">{{ domain.provider }}</el-descriptions-item>
          <el-descriptions-item label="账户">{{ domain.accountName }}</el-descriptions-item>
          <el-descriptions-item label="状态摘要">
            <el-tag :type="summaryType">{{ summaryLabel }}</el-tag>
          </el-descriptions-item>
        </el-descriptions>

        <el-card shadow="never" class="history-card">
          <template #header>
            <div class="history-header">
              <span>传播历史</span>
              <span class="history-subtitle">保留最近 50 次检查记录</span>
            </div>
          </template>
          <el-empty v-if="!historyRows.length" description="暂无历史记录" />
          <el-table v-else :data="historyRows" border max-height="320">
            <el-table-column label="检查时间" min-width="180">
              <template #default="{ row }">{{ formatDateTime(row.checkedAt) }}</template>
            </el-table-column>
            <el-table-column prop="fqdn" label="FQDN" min-width="220" />
            <el-table-column label="状态" width="120">
              <template #default="{ row }">
                <el-tag :type="historyTagType(row.overallStatus)">{{ historyStatusLabel(row.overallStatus) }}</el-tag>
              </template>
            </el-table-column>
            <el-table-column prop="summary" label="摘要" min-width="260" />
            <el-table-column label="明细" width="120" fixed="right">
              <template #default="{ row }">
                <el-button size="small" @click="inspectHistory(row)">查看</el-button>
              </template>
            </el-table-column>
          </el-table>
        </el-card>

        <el-table :data="statusResults" border>
          <el-table-column prop="resolver" label="解析器" min-width="180" />
          <el-table-column label="状态" width="120">
            <template #default="{ row }">
              <el-tag :type="resolverTagType(row)">{{ resolverLabel(row) }}</el-tag>
            </template>
          </el-table-column>
          <el-table-column prop="reasonLabel" label="说明" min-width="220" />
          <el-table-column label="返回值" min-width="280">
            <template #default="{ row }">
              <span>{{ row.answers?.length ? row.answers.join('，') : '无返回值' }}</span>
            </template>
          </el-table-column>
        </el-table>
      </template>
    </el-card>

    <el-drawer v-model="showHistoryDrawer" title="传播历史详情" size="50%">
      <el-empty v-if="!selectedHistory" description="未选择记录" />
      <template v-else>
        <el-descriptions :column="1" border>
          <el-descriptions-item v-if="overviewMode" label="域名">{{ domainNameById(selectedHistory.domainId) }}</el-descriptions-item>
          <el-descriptions-item label="检查时间">{{ formatDateTime(selectedHistory.checkedAt) }}</el-descriptions-item>
          <el-descriptions-item label="FQDN">{{ selectedHistory.fqdn }}</el-descriptions-item>
          <el-descriptions-item label="记录值">{{ historyRecordLabel(selectedHistory) }}</el-descriptions-item>
          <el-descriptions-item label="状态">
            <el-tag :type="historyTagType(selectedHistory.overallStatus)">{{ historyStatusLabel(selectedHistory.overallStatus) }}</el-tag>
          </el-descriptions-item>
          <el-descriptions-item label="摘要">{{ selectedHistory.summary }}</el-descriptions-item>
        </el-descriptions>
        <el-divider>解析器结果</el-divider>
        <el-table :data="selectedHistory.results" border>
          <el-table-column prop="resolver" label="解析器" min-width="180" />
          <el-table-column label="状态" width="120">
            <template #default="{ row }">
              <el-tag :type="resolverTagType(row)">{{ resolverLabel(row) }}</el-tag>
            </template>
          </el-table-column>
          <el-table-column label="说明" min-width="220">
            <template #default="{ row }">{{ propagationReasonLabel(row.reason) }}</template>
          </el-table-column>
          <el-table-column label="返回值" min-width="280">
            <template #default="{ row }">
              <span>{{ row.answers?.length ? row.answers.join('，') : '无返回值' }}</span>
            </template>
          </el-table-column>
        </el-table>
      </template>
    </el-drawer>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import AppLayout from '../layouts/AppLayout.vue'
import { listDomains as listDomainsRequest, triggerPropagationWatch } from '../api/domains'
import { useDomainStore } from '../stores/domains'
import type { Domain, DNSRecord, PropagationHistoryItem, PropagationResult, PropagationStatus } from '../types/domain'

const route = useRoute()
const router = useRouter()
const store = useDomainStore()

const showHistoryDrawer = ref(false)
const selectedHistory = ref<PropagationHistoryItem | null>(null)
const overviewDomains = ref<Domain[]>([])
const watching = ref(false)

const overviewMode = computed(() => !route.params.id)
const visibleDomains = computed(() => (overviewMode.value ? overviewDomains.value : store.domains))
const domainMap = computed(() => new Map(visibleDomains.value.map((item) => [item.id, item])))

const domain = computed<Domain | null>(() => {
  if (overviewMode.value) return null
  const rawId = route.params.id
  if (!rawId) return store.selectedDomain || store.domains[0] || null
  const routeId = Number(rawId)
  if (store.selectedDomain?.id === routeId) return store.selectedDomain
  return store.domains.find((item) => item.id === routeId) || null
})

const pageTitle = computed(() => (overviewMode.value ? '传播监控中心' : domain.value?.name || '传播监控'))
const pageSubtitle = computed(() =>
  overviewMode.value ? '汇总各域名最近一次 DNS 生效检查结果与传播历史' : '按公共解析器查看最近一次 DNS 生效检查结果',
)

const status = computed(() => domain.value?.lastPropagationStatus)
const matchedCount = computed(() => status.value?.matchedCount || 0)
const failedCount = computed(() => status.value?.failedCount || 0)
const pendingCount = computed(() => status.value?.pendingCount || 0)
const historyRows = computed(() => store.propagationHistory)
const checkedAtLabel = computed(() => formatDateTime(status.value?.checkedAt))
const summaryLabel = computed(() => status.value?.summary || '暂无传播检查结果')
const summaryType = computed(() => historyTagType(status.value?.overallStatus))
const statusResults = computed(() =>
  (status.value?.results || []).map((item) => ({
    ...item,
    reasonLabel: propagationReasonLabel(item.reason),
  })),
)

const overviewDomainRows = computed(() =>
  [...overviewDomains.value].sort((a, b) => dateValue(b.lastPropagationStatus?.checkedAt || b.updatedAt) - dateValue(a.lastPropagationStatus?.checkedAt || a.updatedAt)),
)
const overviewDomainCount = computed(() => overviewDomains.value.length)
const verifiedDomainCount = computed(
  () => overviewDomains.value.filter((item) => item.lastPropagationStatus?.overallStatus === 'verified').length,
)
const attentionDomainCount = computed(
  () => overviewDomains.value.filter((item) => ['partial', 'failed', 'pending'].includes(item.lastPropagationStatus?.overallStatus || '')).length,
)

watch(
  () => route.params.id,
  () => {
    showHistoryDrawer.value = false
    selectedHistory.value = null
    void loadPageData()
  },
  { immediate: true },
)

async function loadPageData(options?: { forceRefresh?: boolean; showSuccessMessage?: boolean }) {
  const forceRefresh = options?.forceRefresh ?? false
  const showSuccessMessage = options?.showSuccessMessage ?? false

  if (overviewMode.value) {
    if (!overviewDomains.value.length || forceRefresh) {
      overviewDomains.value = await listDomainsRequest('', false)
    }
    store.setSelectedDomain(null)
    await store.loadPropagationHistory()
    if (showSuccessMessage) {
      ElMessage.success('传播总览已刷新')
    }
    return
  }

  if (!store.domains.length || forceRefresh) {
    await store.loadDomains(store.search, store.includeArchived)
  }
  if (!domain.value) {
    store.setSelectedDomain(null)
    ElMessage.warning('未找到对应域名')
    return
  }
  store.setSelectedDomain(domain.value)
  await store.loadPropagationHistory(domain.value.id)
  if (showSuccessMessage) {
    ElMessage.success('传播状态已刷新')
  }
}

async function reload() {
  await loadPageData({ forceRefresh: true, showSuccessMessage: true })
}

async function toggleWatch() {
  if (!domain.value) return
  if (watching.value) {
    watching.value = false
    ElMessage.info('已停止持续监控')
    return
  }
  watching.value = true
  ElMessage.info('开始持续监控，请等待传播完成或达到最大重试次数')
  try {
    // Get current records to watch — pick the first A/AAAA/CNAME if available
    const records = await store.fetchDomainRecords(domain.value.id)
    const target = records.find((r) => ['A', 'AAAA', 'CNAME', 'MX', 'TXT'].includes(r.type)) || records[0]
    if (!target) {
      ElMessage.warning('域名下没有可监控的记录')
      watching.value = false
      return
    }
    const result = await triggerPropagationWatch(domain.value.id, target, {
      watch: true,
      watchInterval: 30,
      watchMaxAttempts: 20,
    })
    store.domainLastPropagationStatus(domain.value.id, result)
    await store.loadPropagationHistory(domain.value.id)
    ElMessage.success('持续监控完成')
  } catch (e: any) {
    ElMessage.error(e.response?.data?.error || '监控失败')
  } finally {
    watching.value = false
  }
}

function backToDomains() {
  router.push({ name: 'domains' })
}

function openDomainPropagation(domainId: number) {
  const matchedDomain = overviewDomains.value.find((item) => item.id === domainId) || store.domains.find((item) => item.id === domainId) || null
  if (matchedDomain) {
    store.setSelectedDomain(matchedDomain)
  }
  router.push({ name: 'propagation', params: { id: domainId } })
}

function inspectHistory(item: PropagationHistoryItem) {
  selectedHistory.value = item
  showHistoryDrawer.value = true
}

function domainNameById(domainId: number) {
  return domainMap.value.get(domainId)?.name || `域名 #${domainId}`
}

function formatDateTime(value?: string) {
  return value ? new Date(value).toLocaleString() : '未检查'
}

function dateValue(value?: string) {
  return value ? new Date(value).getTime() : 0
}

function historyRecordLabel(item: PropagationHistoryItem) {
  const record = item.record || {}
  const type = String(record.type || '').trim()
  const content = String(record.content || '').trim()
  if (type && content) return `${type} ${content}`
  if (content) return content
  if (type) return type
  return '—'
}

function resolverLabel(item: PropagationResult) {
  if (item.matched) return '已命中'
  if (item.status === 'ok') return '待生效'
  return '异常'
}

function resolverTagType(item: PropagationResult) {
  if (item.matched) return 'success'
  if (item.status === 'ok') return 'warning'
  return 'danger'
}

function historyStatusLabel(status?: string) {
  if (status === 'verified') return '已完成'
  if (status === 'partial') return '部分生效'
  if (status === 'failed') return '检查异常'
  if (status === 'pending') return '待生效'
  return '未检查'
}

function historyTagType(status?: string) {
  if (status === 'verified') return 'success'
  if (status === 'partial') return 'warning'
  if (status === 'failed') return 'danger'
  return 'info'
}

function propagationReasonLabel(reason?: string) {
  if (reason === 'matched') return '解析结果已命中目标值'
  if (reason === 'resolver_error' || reason === 'lookup_failed') return '解析器查询失败'
  if (reason === 'no_answer') return '解析器未返回记录'
  if (reason === 'value_mismatch') return '解析返回值与目标值不匹配'
  return '暂无明细'
}
</script>

<style scoped>
.toolbar {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 16px;
}

.domain-title {
  font-size: 20px;
  font-weight: 600;
}

.domain-subtitle {
  color: #6b7280;
  font-size: 13px;
}

.summary-row {
  margin-bottom: 16px;
}

.summary-card,
.history-card {
  margin-bottom: 16px;
}

.history-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 12px;
}

.history-subtitle {
  color: #6b7280;
  font-size: 13px;
}
</style>
