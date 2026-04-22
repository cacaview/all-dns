<template>
  <AppLayout>
    <el-row :gutter="16" class="summary-row">
      <el-col :md="6" :sm="12" :xs="24">
        <el-statistic title="展示域名" :value="filteredDomains.length" />
      </el-col>
      <el-col :md="6" :sm="12" :xs="24">
        <el-statistic title="星标域名" :value="starredDomainCount" />
      </el-col>
      <el-col :md="6" :sm="12" :xs="24">
        <el-statistic title="待关注传播" :value="attentionDomainCount" />
      </el-col>
      <el-col :md="6" :sm="12" :xs="24">
        <el-statistic title="已归档" :value="archivedDomainCount" />
      </el-col>
    </el-row>

    <el-card class="summary-card" shadow="never">
      <template #header>
        <div class="section-header">
          <span>域名概览</span>
          <span class="section-subtitle">按 Provider、标签、星标与传播状态快速定位业务域名</span>
        </div>
      </template>
      <el-row :gutter="16">
        <el-col :md="16" :sm="24" :xs="24">
          <el-descriptions :column="2" border>
            <el-descriptions-item label="涉及 Provider">{{ filteredProviderCount }}</el-descriptions-item>
            <el-descriptions-item label="标签总数">{{ filteredTagCount }}</el-descriptions-item>
            <el-descriptions-item label="最近同步">{{ latestSyncedLabel }}</el-descriptions-item>
            <el-descriptions-item label="当前筛选">{{ activeFilterSummary }}</el-descriptions-item>
          </el-descriptions>
        </el-col>
        <el-col :md="8" :sm="24" :xs="24">
          <el-alert
            :title="attentionHeadline"
            type="info"
            :description="attentionDescription"
            :closable="false"
            show-icon
          />
        </el-col>
      </el-row>
    </el-card>

    <el-card>
      <template #header>
        <div class="toolbar">
          <div class="toolbar-main">
            <el-space wrap>
              <el-input v-model="search" placeholder="搜索域名 / 账户" clearable @change="reloadDomains" />
              <el-select v-model="providerFilter" clearable placeholder="筛选 Provider" style="width: 180px">
                <el-option v-for="provider in providerOptions" :key="provider" :label="provider" :value="provider" />
              </el-select>
              <el-select v-model="propagationFilter" clearable placeholder="传播状态" style="width: 180px">
                <el-option label="已完成" value="verified" />
                <el-option label="部分生效" value="partial" />
                <el-option label="待生效" value="pending" />
                <el-option label="检查异常" value="failed" />
                <el-option label="未检查" value="unchecked" />
              </el-select>
              <el-select v-model="tagFilter" clearable filterable placeholder="筛选标签" style="width: 180px">
                <el-option v-for="tag in tagOptions" :key="tag" :label="tag" :value="tag" />
              </el-select>
              <el-switch v-model="starredOnly" inline-prompt active-text="仅星标" inactive-text="全部星标" />
              <el-switch v-model="includeArchived" inline-prompt active-text="含归档" inactive-text="仅活跃" @change="reloadDomains" />
              <el-button type="primary" :disabled="!editable" @click="openAccountDialog">新增账户</el-button>
            </el-space>
            <el-space class="toolbar-meta" wrap>
              <el-tag :type="editable ? 'success' : 'info'">{{ roleLabel }}</el-tag>
              <span v-if="!editable" class="readonly-text">{{ readonlyHint }}</span>
            </el-space>
          </div>
          <el-space wrap>
            <el-button @click="resetFilters">清空筛选</el-button>
            <el-button @click="reloadDomains">刷新</el-button>
          </el-space>
        </div>
      </template>

      <DomainTable
        :domains="filteredDomains"
        :loading="store.loading"
        :editable="editable"
        @toggle-star="handleToggleStar"
        @edit-records="openRecords"
        @view-propagation="openPropagation"
        @edit-profile="openProfile"
        @edit-tags="editTags"
        @toggle-archive="handleToggleArchive"
      />
    </el-card>

    <DomainEditDialog v-model="showRecordDialog" :domain="selectedDomain" :editable="editable" @saved="reloadAll" />
    <DomainProfileDrawer v-model="showProfileDrawer" :domain="selectedDomain" :editable="editable" @saved="reloadDomains" />
    <AccountFormDialog v-model="showAccountDialog" :editable="editable" @saved="reloadAll" />
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import AppLayout from '../layouts/AppLayout.vue'
import DomainTable from '../components/DomainTable.vue'
import DomainEditDialog from '../components/DomainEditDialog.vue'
import DomainProfileDrawer from '../components/DomainProfileDrawer.vue'
import AccountFormDialog from '../components/AccountFormDialog.vue'
import { toggleStar, updateArchive, updateTags } from '../api/domains'
import { useDomainStore } from '../stores/domains'
import { useAuthStore } from '../stores/auth'
import type { Domain } from '../types/domain'
import { canEdit } from '../utils/permissions'

const router = useRouter()
const store = useDomainStore()
const authStore = useAuthStore()
const search = ref('')
const includeArchived = ref(false)
const providerFilter = ref('')
const propagationFilter = ref('')
const tagFilter = ref('')
const starredOnly = ref(false)
const selectedDomain = ref<Domain | null>(null)
const showRecordDialog = ref(false)
const showProfileDrawer = ref(false)
const showAccountDialog = ref(false)
const editable = computed(() => canEdit(authStore.user))
const roleLabel = computed(() => {
  if (authStore.user?.role === 'admin') return '管理员'
  if (authStore.user?.role === 'editor') return '编辑者'
  if (authStore.user?.role === 'viewer') return '只读访客'
  return '未登录'
})
const readonlyHint = '当前账号为只读角色，可查看域名、传播和备份，但不能修改资产。'
const providerOptions = computed(() => [...new Set(store.domains.map((item) => item.provider).filter(Boolean))].sort())
const tagOptions = computed(() => [...new Set(store.domains.flatMap((item) => item.tags || []).filter(Boolean))].sort())
const filteredDomains = computed(() =>
  store.domains.filter((item) => {
    if (providerFilter.value && item.provider !== providerFilter.value) return false
    if (tagFilter.value && !item.tags.includes(tagFilter.value)) return false
    if (starredOnly.value && !item.isStarred) return false
    const overallStatus = item.lastPropagationStatus?.overallStatus || 'unchecked'
    if (propagationFilter.value && overallStatus !== propagationFilter.value) return false
    return true
  }),
)
const starredDomainCount = computed(() => filteredDomains.value.filter((item) => item.isStarred).length)
const archivedDomainCount = computed(() => filteredDomains.value.filter((item) => item.isArchived).length)
const attentionDomainCount = computed(
  () => filteredDomains.value.filter((item) => ['partial', 'failed', 'pending'].includes(item.lastPropagationStatus?.overallStatus || '')).length,
)
const filteredProviderCount = computed(() => new Set(filteredDomains.value.map((item) => item.provider)).size)
const filteredTagCount = computed(() => new Set(filteredDomains.value.flatMap((item) => item.tags || [])).size)
const latestSyncedDomain = computed(() => {
  const datedDomains = filteredDomains.value.filter((item) => item.lastSyncedAt)
  if (!datedDomains.length) return null
  return [...datedDomains].sort((a, b) => new Date(b.lastSyncedAt || '').getTime() - new Date(a.lastSyncedAt || '').getTime())[0]
})
const latestSyncedLabel = computed(() => {
  if (!latestSyncedDomain.value?.lastSyncedAt) return '暂无同步记录'
  return `${latestSyncedDomain.value.name} · ${formatDateTime(latestSyncedDomain.value.lastSyncedAt)}`
})
const activeFilterSummary = computed(() => {
  const items = [] as string[]
  if (providerFilter.value) items.push(`Provider: ${providerFilter.value}`)
  if (propagationFilter.value) items.push(`传播: ${propagationLabel(propagationFilter.value)}`)
  if (tagFilter.value) items.push(`标签: ${tagFilter.value}`)
  if (starredOnly.value) items.push('仅星标')
  return items.length ? items.join(' / ') : '无'
})
const attentionHeadline = computed(() => {
  if (attentionDomainCount.value) return `有 ${attentionDomainCount.value} 个域名需要继续关注传播情况`
  return '当前筛选结果中的域名传播状态稳定'
})
const attentionDescription = computed(() => {
  if (attentionDomainCount.value) {
    return '可通过传播详情查看各公共解析器命中情况，并继续跟进待生效或异常记录。'
  }
  return '可结合 Provider、标签、星标等筛选条件继续定位目标域名。'
})

onMounted(async () => {
  search.value = store.search
  includeArchived.value = store.includeArchived
  await reloadAll()
})

function ensureEditable() {
  if (editable.value) return true
  ElMessage.warning(readonlyHint)
  return false
}

function openAccountDialog() {
  if (!ensureEditable()) return
  showAccountDialog.value = true
}

function openRecords(domain: Domain) {
  selectedDomain.value = domain
  showRecordDialog.value = true
}

function openProfile(domain: Domain) {
  selectedDomain.value = domain
  showProfileDrawer.value = true
}

function openPropagation(domain: Domain) {
  store.setSelectedDomain(domain)
  void router.push({ name: 'propagation', params: { id: domain.id } })
}

async function reloadDomains() {
  await store.loadDomains(search.value, includeArchived.value)
}

async function reloadAll() {
  await store.loadDashboard()
  await reloadDomains()
}

function resetFilters() {
  providerFilter.value = ''
  propagationFilter.value = ''
  tagFilter.value = ''
  starredOnly.value = false
  includeArchived.value = false
  search.value = ''
  void reloadDomains()
}

async function editTags(domain: Domain) {
  if (!ensureEditable()) return
  try {
    const { value } = await ElMessageBox.prompt('请输入标签，使用英文逗号分隔', '编辑标签', {
      inputValue: domain.tags.join(', '),
    })
    await updateTags(domain.id, value.split(',').map((item) => item.trim()).filter(Boolean))
    ElMessage.success('标签已更新')
    await reloadDomains()
  } catch {
    // ignore cancel
  }
}

async function handleToggleStar(domain: Domain) {
  if (!ensureEditable()) return
  try {
    await toggleStar(domain.id)
    ElMessage.success('星标状态已更新')
    await reloadDomains()
  } catch (error: any) {
    ElMessage.error(error?.message || '更新星标失败')
  }
}

async function handleToggleArchive(domain: Domain) {
  if (!ensureEditable()) return
  const archived = !domain.isArchived
  try {
    await ElMessageBox.confirm(
      archived ? `确认归档 ${domain.name} 吗？` : `确认取消归档 ${domain.name} 吗？`,
      archived ? '归档域名' : '取消归档',
      { type: 'warning' },
    )
    await updateArchive(domain.id, archived)
    ElMessage.success(archived ? '域名已归档' : '域名已恢复为活跃')
    await reloadDomains()
  } catch (error: any) {
    if (error === 'cancel') return
    ElMessage.error(error?.message || (archived ? '归档域名失败' : '取消归档失败'))
  }
}

function propagationLabel(status: string) {
  if (status === 'verified') return '已完成'
  if (status === 'partial') return '部分生效'
  if (status === 'pending') return '待生效'
  if (status === 'failed') return '检查异常'
  return '未检查'
}

function formatDateTime(value?: string) {
  return value ? new Date(value).toLocaleString() : '未设置'
}
</script>

<style scoped>
.summary-row,
.summary-card {
  margin-bottom: 16px;
}

.section-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 12px;
}

.section-subtitle {
  color: #6b7280;
  font-size: 13px;
}

.toolbar {
  display: flex;
  justify-content: space-between;
  gap: 16px;
  align-items: flex-start;
}
.toolbar-main {
  display: flex;
  flex-direction: column;
  gap: 10px;
}
.readonly-text {
  color: #6b7280;
  font-size: 12px;
}
</style>