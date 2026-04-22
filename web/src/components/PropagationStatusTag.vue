<template>
  <el-tooltip v-if="hasStatus" placement="top" effect="dark">
    <template #content>
      <div class="tooltip-title">{{ summary }}</div>
      <div v-if="status?.checkedAt">检查时间：{{ formatTime(status.checkedAt) }}</div>
      <div v-if="status?.fqdn">记录：{{ status.fqdn }}</div>
      <div>命中：{{ matched }}/{{ total }}</div>
      <div>异常：{{ failed }}</div>
      <div>待生效：{{ pending }}</div>
    </template>
    <el-tag :type="tagType" effect="light">{{ label }}</el-tag>
  </el-tooltip>
  <el-tag v-else :type="tagType" effect="light">{{ label }}</el-tag>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import type { PropagationStatus } from '../types/domain'

const props = defineProps<{ status?: PropagationStatus }>()

const hasStatus = computed(() => Boolean(props.status && Object.keys(props.status).length))
const matched = computed(() => props.status?.matchedCount ?? props.status?.matchedResolvers?.length ?? 0)
const total = computed(() => props.status?.totalResolvers ?? 0)
const failed = computed(() => props.status?.failedCount ?? props.status?.failedResolvers?.length ?? 0)
const pending = computed(() => props.status?.pendingCount ?? props.status?.pendingResolvers?.length ?? 0)
const summary = computed(() => props.status?.summary || '未检查传播状态')

const label = computed(() => {
  if (!hasStatus.value) return '未检查'
  if (props.status?.overallStatus === 'verified') return `已生效 ${matched.value}/${total.value}`
  if (props.status?.overallStatus === 'failed') return `检查失败 ${failed.value}/${total.value}`
  return `传播中 ${matched.value}/${total.value}`
})

const tagType = computed(() => {
  if (!hasStatus.value) return 'info'
  if (props.status?.overallStatus === 'verified') return 'success'
  if (props.status?.overallStatus === 'failed') return 'danger'
  if (props.status?.overallStatus === 'partial') return 'warning'
  return 'info'
})

function formatTime(value?: string) {
  return value ? new Date(value).toLocaleString() : '未知时间'
}
</script>

<style scoped>
.tooltip-title {
  font-weight: 600;
  margin-bottom: 4px;
}
</style>
