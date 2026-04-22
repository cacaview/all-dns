<template>
  <el-table :data="domains" v-loading="loading" border>
    <el-table-column label="域名" min-width="220">
      <template #default="{ row }">
        <div class="domain-name">{{ row.name }}</div>
        <div class="domain-meta">{{ row.provider }} · {{ row.accountName }}</div>
      </template>
    </el-table-column>
    <el-table-column label="星标" width="90">
      <template #default="{ row }">
        <el-button v-if="editable" link @click="$emit('toggle-star', row)">{{ row.isStarred ? '★' : '☆' }}</el-button>
        <span v-else class="star-indicator">{{ row.isStarred ? '★' : '☆' }}</span>
      </template>
    </el-table-column>
    <el-table-column label="标签" min-width="220">
      <template #default="{ row }">
        <div class="tag-list">
          <el-tag v-for="tag in row.tags" :key="tag" size="small">{{ tag }}</el-tag>
          <span v-if="!row.tags?.length" class="muted">未设置</span>
        </div>
      </template>
    </el-table-column>
    <el-table-column label="传播状态" width="180">
      <template #default="{ row }">
        <PropagationStatusTag :status="row.lastPropagationStatus" />
        <div class="propagation-meta">{{ propagationSummary(row) }}</div>
      </template>
    </el-table-column>
    <el-table-column label="状态" width="140">
      <template #default="{ row }">
        <el-space>
          <el-tag v-if="row.isArchived" type="info">已归档</el-tag>
          <el-tag v-else type="success">活跃</el-tag>
        </el-space>
      </template>
    </el-table-column>
    <el-table-column label="最近同步" width="180">
      <template #default="{ row }">
        {{ row.lastSyncedAt ? new Date(row.lastSyncedAt).toLocaleString() : '未同步' }}
      </template>
    </el-table-column>
    <el-table-column label="操作" width="400" fixed="right">
      <template #default="{ row }">
        <el-space wrap>
          <el-button size="small" @click="$emit('edit-records', row)">{{ editable ? '解析编辑' : '查看解析' }}</el-button>
          <el-button size="small" @click="$emit('view-propagation', row)">传播详情</el-button>
          <el-button size="small" @click="$emit('edit-profile', row)">{{ editable ? '业务档案' : '查看档案' }}</el-button>
          <el-button v-if="editable" size="small" @click="$emit('edit-tags', row)">标签</el-button>
          <el-button v-if="editable" size="small" :type="row.isArchived ? 'success' : 'warning'" @click="$emit('toggle-archive', row)">
            {{ row.isArchived ? '取消归档' : '归档' }}
          </el-button>
        </el-space>
      </template>
    </el-table-column>
  </el-table>
</template>

<script setup lang="ts">
import type { Domain } from '../types/domain'
import PropagationStatusTag from './PropagationStatusTag.vue'

defineProps<{
  domains: Domain[]
  loading?: boolean
  editable?: boolean
}>()

defineEmits<{
  (e: 'toggle-star', domain: Domain): void
  (e: 'edit-records', domain: Domain): void
  (e: 'view-propagation', domain: Domain): void
  (e: 'edit-profile', domain: Domain): void
  (e: 'edit-tags', domain: Domain): void
  (e: 'toggle-archive', domain: Domain): void
}>()

function propagationSummary(domain: Domain) {
  const status = domain.lastPropagationStatus
  if (!status?.checkedAt) return '暂无最近检查结果'
  return `已命中 ${status.matchedCount || 0} / 待生效 ${status.pendingCount || 0} / 异常 ${status.failedCount || 0}`
}
</script>

<style scoped>
.domain-name {
  font-weight: 600;
}
.domain-meta,
.muted,
.propagation-meta {
  color: #6b7280;
  font-size: 12px;
}
.tag-list {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
}
.star-indicator {
  color: #f59e0b;
  font-size: 16px;
}
</style>
