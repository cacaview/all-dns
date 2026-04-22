<template>
  <AppLayout>
    <el-row :gutter="16" class="summary-row">
      <el-col :md="6" :sm="12" :xs="24">
        <el-statistic title="通知总数" :value="store.reminders.length" />
      </el-col>
      <el-col :md="6" :sm="12" :xs="24">
        <el-statistic title="待处理" :value="openReminderCount" />
      </el-col>
      <el-col :md="6" :sm="12" :xs="24">
        <el-statistic title="已处理" :value="handledReminderCount" />
      </el-col>
      <el-col :md="6" :sm="12" :xs="24">
        <el-statistic title="紧急 / 已过期" :value="criticalReminderCount" />
      </el-col>
    </el-row>

    <el-card class="summary-card" shadow="never">
      <template #header>
        <div class="section-header">
          <span>处理概览</span>
          <span class="section-subtitle">按严重级别和处理状态集中查看账户凭证提醒</span>
        </div>
      </template>
      <el-empty v-if="!store.reminders.length" description="暂无通知" />
      <el-row v-else :gutter="16">
        <el-col :md="14" :sm="24" :xs="24">
          <el-descriptions :column="2" border>
            <el-descriptions-item label="已过期">{{ expiredReminderCount }}</el-descriptions-item>
            <el-descriptions-item label="紧急">{{ criticalOnlyReminderCount }}</el-descriptions-item>
            <el-descriptions-item label="警告">{{ warningReminderCount }}</el-descriptions-item>
            <el-descriptions-item label="提醒">{{ noticeReminderCount }}</el-descriptions-item>
            <el-descriptions-item label="最早到期">{{ nearestExpiryLabel }}</el-descriptions-item>
            <el-descriptions-item label="待处理账户">{{ uniqueOpenAccountCount }}</el-descriptions-item>
          </el-descriptions>
        </el-col>
        <el-col :md="10" :sm="24" :xs="24">
          <el-alert
            :title="priorityHeadline"
            :type="priorityAlertType"
            :description="priorityDescription"
            :closable="false"
            show-icon
          />
        </el-col>
      </el-row>
    </el-card>

    <el-card shadow="never">
      <template #header>
        <div class="toolbar">
          <el-space wrap>
            <el-radio-group v-model="filter">
              <el-radio-button label="all">全部</el-radio-button>
              <el-radio-button label="open">未处理</el-radio-button>
              <el-radio-button label="handled">已处理</el-radio-button>
            </el-radio-group>
            <el-select v-model="severityFilter" clearable placeholder="筛选级别" style="width: 160px">
              <el-option label="已过期" value="expired" />
              <el-option label="紧急" value="critical" />
              <el-option label="警告" value="warning" />
              <el-option label="提醒" value="notice" />
            </el-select>
            <el-input v-model="search" placeholder="搜索账户 / Provider" clearable style="width: 240px" />
          </el-space>
          <el-space wrap>
            <el-button @click="resetFilters">清空筛选</el-button>
            <el-button @click="markAllOpenHandled" :disabled="!openReminderCount">全部标记已处理</el-button>
            <el-button @click="reload">刷新</el-button>
          </el-space>
        </div>
      </template>

      <el-empty v-if="!filteredReminders.length" description="暂无通知" />
      <el-table v-else :data="filteredReminders" border>
        <el-table-column prop="name" label="账户" min-width="200" />
        <el-table-column prop="provider" label="Provider" width="140" />
        <el-table-column label="级别" width="120">
          <template #default="{ row }">
            <el-tag :type="row.handled ? 'info' : reminderTagType(row)">{{ reminderStatusText(row) }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="说明" min-width="260">
          <template #default="{ row }">{{ reminderDetailText(row) }}</template>
        </el-table-column>
        <el-table-column label="剩余天数" width="120">
          <template #default="{ row }">{{ row.daysLeft }}</template>
        </el-table-column>
        <el-table-column label="过期时间" width="180">
          <template #default="{ row }">{{ formatDateTime(row.expiresAt) }}</template>
        </el-table-column>
        <el-table-column label="处理时间" width="180">
          <template #default="{ row }">{{ row.handledAt ? formatDateTime(row.handledAt) : '未处理' }}</template>
        </el-table-column>
        <el-table-column label="操作" width="140" fixed="right">
          <template #default="{ row }">
            <el-button size="small" @click="toggle(row)">{{ reminderActionLabel(row) }}</el-button>
          </template>
        </el-table-column>
      </el-table>
    </el-card>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import AppLayout from '../layouts/AppLayout.vue'
import {
  reminderActionLabel,
  reminderDetailText,
  reminderStatusText,
  reminderTagType,
  useDomainStore,
} from '../stores/domains'
import type { Reminder } from '../types/domain'

const store = useDomainStore()
const filter = ref<'all' | 'open' | 'handled'>('open')
const severityFilter = ref('')
const search = ref('')

const filteredReminders = computed(() =>
  store.reminders.filter((item) => {
    if (filter.value === 'open' && item.handled) return false
    if (filter.value === 'handled' && !item.handled) return false
    if (severityFilter.value && item.severity !== severityFilter.value) return false
    const keyword = search.value.trim().toLowerCase()
    if (!keyword) return true
    return item.name.toLowerCase().includes(keyword) || item.provider.toLowerCase().includes(keyword)
  }),
)

const openReminders = computed(() => store.reminders.filter((item) => !item.handled))
const handledReminderCount = computed(() => store.reminders.filter((item) => item.handled).length)
const openReminderCount = computed(() => openReminders.value.length)
const expiredReminderCount = computed(() => store.reminders.filter((item) => item.severity === 'expired').length)
const criticalOnlyReminderCount = computed(() => store.reminders.filter((item) => item.severity === 'critical').length)
const warningReminderCount = computed(() => store.reminders.filter((item) => item.severity === 'warning').length)
const noticeReminderCount = computed(() => store.reminders.filter((item) => item.severity === 'notice').length)
const criticalReminderCount = computed(() => expiredReminderCount.value + criticalOnlyReminderCount.value)
const uniqueOpenAccountCount = computed(() => new Set(openReminders.value.map((item) => item.accountId)).size)
const nearestExpiry = computed(() => {
  const datedItems = openReminders.value.filter((item) => item.expiresAt)
  if (!datedItems.length) return null
  return [...datedItems].sort((a, b) => new Date(a.expiresAt || '').getTime() - new Date(b.expiresAt || '').getTime())[0]
})
const nearestExpiryLabel = computed(() => {
  if (!nearestExpiry.value?.expiresAt) return '未设置'
  return `${nearestExpiry.value.name} · ${formatDateTime(nearestExpiry.value.expiresAt)}`
})
const priorityHeadline = computed(() => {
  if (expiredReminderCount.value) return `有 ${expiredReminderCount.value} 个账户凭证已过期`
  if (criticalOnlyReminderCount.value) return `有 ${criticalOnlyReminderCount.value} 个账户需要优先轮换`
  if (warningReminderCount.value) return `有 ${warningReminderCount.value} 个账户即将过期`
  if (openReminderCount.value) return '当前存在待处理凭证提醒'
  return '当前没有待处理提醒'
})
const priorityDescription = computed(() => {
  if (nearestExpiry.value?.expiresAt) {
    return `最近到期账户：${nearestExpiry.value.name}（${nearestExpiry.value.provider}），到期时间 ${formatDateTime(nearestExpiry.value.expiresAt)}。`
  }
  if (openReminderCount.value) {
    return '建议进入账户管理或轮换流程，尽快完成凭证检查与更新。'
  }
  return '所有当前提醒均已处理。'
})
const priorityAlertType = computed(() => {
  if (expiredReminderCount.value || criticalOnlyReminderCount.value) return 'error'
  if (warningReminderCount.value) return 'warning'
  return 'success'
})

onMounted(async () => {
  await reload()
})

async function reload() {
  await store.loadDashboard()
}

function resetFilters() {
  filter.value = 'open'
  severityFilter.value = ''
  search.value = ''
}

function toggle(item: Reminder) {
  store.setReminderHandled(item, !item.handled)
}

function markAllOpenHandled() {
  for (const item of openReminders.value) {
    store.setReminderHandled(item, true)
  }
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
  align-items: center;
  gap: 16px;
}
</style>
