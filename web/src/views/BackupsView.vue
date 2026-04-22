<template>
  <AppLayout>
    <el-card>
      <template #header>
        <div class="toolbar">
          <div class="toolbar-main">
            <el-space wrap>
              <el-input v-model="search" placeholder="搜索域名 / 账户 / 原因" clearable @change="reload" />
              <el-select v-model="providerFilter" clearable placeholder="筛选 Provider" style="width: 160px">
                <el-option v-for="provider in providerOptions" :key="provider" :label="provider" :value="provider" />
              </el-select>
              <el-select v-model="domainFilter" clearable filterable placeholder="筛选域名" style="width: 220px">
                <el-option v-for="domain in domainOptions" :key="domain" :label="domain" :value="domain" />
              </el-select>
              <el-date-picker
                v-model="createdRange"
                type="daterange"
                range-separator="至"
                start-placeholder="开始时间"
                end-placeholder="结束时间"
                unlink-panels
              />
            </el-space>
            <el-space wrap class="toolbar-actions">
              <el-button @click="resetFilters">清空筛选</el-button>
              <el-button @click="reload">刷新</el-button>
            </el-space>
          </div>
        </div>
      </template>

      <el-row :gutter="16" class="summary-row">
        <el-col :md="6" :sm="12" :xs="24">
          <el-statistic title="快照数量" :value="filteredBackups.length" />
        </el-col>
        <el-col :md="6" :sm="12" :xs="24">
          <el-statistic title="覆盖域名" :value="filteredDomainCount" />
        </el-col>
        <el-col :md="6" :sm="12" :xs="24">
          <el-statistic title="涉及 Provider" :value="filteredProviderCount" />
        </el-col>
        <el-col :md="6" :sm="12" :xs="24">
          <el-statistic title="恢复快照" :value="restoredBackupCount" />
        </el-col>
      </el-row>

      <el-table :data="filteredBackups" v-loading="loading" border>
        <el-table-column label="快照" width="120">
          <template #default="{ row }">#{{ row.id }}</template>
        </el-table-column>
        <el-table-column prop="domainName" label="域名" min-width="220" />
        <el-table-column prop="accountName" label="账户" min-width="160" />
        <el-table-column prop="provider" label="Provider" width="120" />
        <el-table-column prop="reason" label="原因" min-width="220" />
        <el-table-column label="记录数" width="100">
          <template #default="{ row }">{{ row.recordCount }}</template>
        </el-table-column>
        <el-table-column label="创建时间" width="180">
          <template #default="{ row }">{{ formatDateTime(row.createdAt) }}</template>
        </el-table-column>
        <el-table-column label="备注" min-width="180">
          <template #default="{ row }">{{ row.restoreLabel || '普通快照' }}</template>
        </el-table-column>
        <el-table-column label="操作" width="300" fixed="right">
          <template #default="{ row }">
            <el-space>
              <el-button size="small" @click="inspect(row)">查看内容</el-button>
              <el-button size="small" @click="download(row)">导出</el-button>
              <el-button size="small" type="warning" @click="restore(row)">恢复</el-button>
            </el-space>
          </template>
        </el-table-column>
      </el-table>
    </el-card>

    <el-drawer v-model="showDetail" title="快照详情" size="50%">
      <el-empty v-if="!selectedBackup" description="未选择快照" />
      <template v-else>
        <el-descriptions :column="1" border>
          <el-descriptions-item label="快照 ID">#{{ selectedBackup.id }}</el-descriptions-item>
          <el-descriptions-item label="域名">{{ selectedBackup.domainName }}</el-descriptions-item>
          <el-descriptions-item label="账户">{{ selectedBackup.accountName }}</el-descriptions-item>
          <el-descriptions-item label="Provider">{{ selectedBackup.provider }}</el-descriptions-item>
          <el-descriptions-item label="原因">{{ selectedBackup.reason }}</el-descriptions-item>
          <el-descriptions-item label="记录数">{{ selectedBackup.recordCount }}</el-descriptions-item>
          <el-descriptions-item label="备注">{{ selectedBackup.restoreLabel || '普通快照' }}</el-descriptions-item>
          <el-descriptions-item label="创建时间">{{ formatDateTime(selectedBackup.createdAt) }}</el-descriptions-item>
        </el-descriptions>
        <el-divider>JSON 内容</el-divider>
        <pre class="json-panel">{{ JSON.stringify(selectedBackup.content, null, 2) }}</pre>
      </template>
    </el-drawer>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import AppLayout from '../layouts/AppLayout.vue'
import { exportBackup, restoreBackup } from '../api/domains'
import { useDomainStore } from '../stores/domains'
import type { BackupListItem } from '../types/domain'

const store = useDomainStore()
const loading = ref(false)
const search = ref('')
const providerFilter = ref('')
const domainFilter = ref('')
const createdRange = ref<Date[]>([])
const showDetail = ref(false)
const selectedBackup = ref<BackupListItem | null>(null)

const providerOptions = computed(() => [...new Set(store.backups.map((item) => item.provider).filter(Boolean))].sort())
const domainOptions = computed(() => [...new Set(store.backups.map((item) => item.domainName).filter(Boolean))].sort())

const filteredBackups = computed(() =>
  store.backups.filter((item) => {
    if (providerFilter.value && item.provider !== providerFilter.value) return false
    if (domainFilter.value && item.domainName !== domainFilter.value) return false
    if (createdRange.value.length === 2) {
      const [start, end] = createdRange.value
      const createdAt = new Date(item.createdAt).getTime()
      if (createdAt < startOfDay(start).getTime()) return false
      if (createdAt > endOfDay(end).getTime()) return false
    }
    return true
  }),
)

const filteredDomainCount = computed(() => new Set(filteredBackups.value.map((item) => item.domainName)).size)
const filteredProviderCount = computed(() => new Set(filteredBackups.value.map((item) => item.provider)).size)
const restoredBackupCount = computed(() => filteredBackups.value.filter((item) => Boolean(item.restoreLabel)).length)

onMounted(async () => {
  await reload()
})

async function reload() {
  loading.value = true
  try {
    await store.loadBackups(search.value)
  } finally {
    loading.value = false
  }
}

function resetFilters() {
  providerFilter.value = ''
  domainFilter.value = ''
  createdRange.value = []
}

function inspect(item: BackupListItem) {
  selectedBackup.value = item
  showDetail.value = true
}

function formatDateTime(value?: string) {
  return value ? new Date(value).toLocaleString() : '—'
}

function startOfDay(value: Date) {
  return new Date(value.getFullYear(), value.getMonth(), value.getDate(), 0, 0, 0, 0)
}

function endOfDay(value: Date) {
  return new Date(value.getFullYear(), value.getMonth(), value.getDate(), 23, 59, 59, 999)
}

async function download(item: BackupListItem) {
  try {
    const { data, filename } = await exportBackup(item.id)
    const blob = new Blob([data], { type: 'application/json' })
    const href = URL.createObjectURL(blob)
    const link = document.createElement('a')
    link.href = href
    link.download = filename
    document.body.appendChild(link)
    link.click()
    document.body.removeChild(link)
    URL.revokeObjectURL(href)
    ElMessage.success('快照导出成功')
  } catch (error: any) {
    ElMessage.error(error?.message || '导出快照失败')
  }
}

async function restore(item: BackupListItem) {
  try {
    await ElMessageBox.confirm(`确认将 ${item.domainName} 恢复到快照 #${item.id} 吗？`, '恢复快照', { type: 'warning' })
    await restoreBackup(item.id)
    ElMessage.success('快照已恢复，并生成恢复快照')
    await reload()
  } catch (error: any) {
    if (error === 'cancel') return
    ElMessage.error(error?.message || '恢复快照失败')
  }
}
</script>

<style scoped>
.toolbar {
  display: flex;
  justify-content: space-between;
  gap: 16px;
  align-items: center;
}

.toolbar-main {
  display: flex;
  width: 100%;
  justify-content: space-between;
  gap: 16px;
  align-items: flex-start;
}

.toolbar-actions {
  justify-content: flex-end;
}

.summary-row {
  margin-bottom: 16px;
}

.json-panel {
  margin: 0;
  padding: 16px;
  border-radius: 8px;
  background: #0f172a;
  color: #e2e8f0;
  overflow: auto;
}
</style>
