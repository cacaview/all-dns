<template>
  <AppLayout>
    <el-card shadow="never">
      <template #header>
        <div class="section-header">
          <span>用户管理</span>
          <span class="section-subtitle">查看所有用户并调整角色（仅管理员可操作）</span>
        </div>
      </template>

      <el-empty v-if="!users.length" description="暂无用户" />
      <el-table v-else :data="users" border stripe>
        <el-table-column prop="id" label="ID" width="80" />
        <el-table-column prop="email" label="邮箱" min-width="200" />
        <el-table-column prop="role" label="角色" width="140">
          <template #default="{ row }">
            <el-select
              v-if="row.id !== currentUserId"
              :model-value="row.role"
              @change="handleRoleChange(row, $event)"
            >
              <el-option label="管理员" value="admin" />
              <el-option label="编辑者" value="editor" />
              <el-option label="查看者" value="viewer" />
            </el-select>
            <el-tag v-else type="info">{{ roleLabel(row.role) }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="oauthProvider" label="登录方式" width="120">
          <template #default="{ row }">{{ providerLabel(row.oauthProvider) }}</template>
        </el-table-column>
        <el-table-column prop="createdAt" label="注册时间" width="180">
          <template #default="{ row }">{{ formatDate(row.createdAt) }}</template>
        </el-table-column>
      </el-table>
    </el-card>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { ElMessage } from 'element-plus'
import AppLayout from '../layouts/AppLayout.vue'
import { useAuthStore } from '../stores/auth'
import { listUsers, updateUserRole } from '../api/users'
import type { User } from '../types/auth'

const auth = useAuthStore()
const users = ref<User[]>([])

const currentUserId = computed(() => auth.user?.id)

onMounted(async () => {
  users.value = await listUsers()
})

async function handleRoleChange(row: User, role: User['role']) {
  try {
    await updateUserRole(row.id, role)
    row.role = role
    ElMessage.success('角色已更新')
  } catch (e: any) {
    ElMessage.error(e.response?.data?.error || '更新失败')
  }
}

function roleLabel(role: User['role']) {
  return { admin: '管理员', editor: '编辑者', viewer: '查看者' }[role]
}

function providerLabel(provider: string) {
  return { github: 'GitHub', gitlab: 'GitLab', dev: 'Dev登录' }[provider] || provider
}

function formatDate(date: string) {
  return new Date(date).toLocaleString('zh-CN')
}
</script>

<style scoped>
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
</style>
