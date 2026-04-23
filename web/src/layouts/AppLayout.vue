<template>
  <el-container class="layout-shell">
    <el-aside width="220px" class="layout-sidebar">
      <div class="brand">DNS Hub</div>
      <el-menu router :default-active="activeMenu" class="menu">
        <el-menu-item index="/dashboard">仪表盘</el-menu-item>
        <el-menu-item index="/domains">域名管理</el-menu-item>
        <el-menu-item index="/propagation">传播监控</el-menu-item>
        <el-menu-item index="/notifications">通知中心</el-menu-item>
        <el-menu-item index="/backups">备份中心</el-menu-item>
        <el-menu-item v-if="auth.user?.role === 'admin'" index="/webhooks">Webhook 管理</el-menu-item>
        <el-menu-item v-if="auth.user?.role === 'admin'" index="/users">用户管理</el-menu-item>
      </el-menu>
    </el-aside>
    <el-container>
      <el-header class="layout-header">
        <div>
          <div class="title">{{ title }}</div>
          <div class="subtitle">统一管理多云 DNS 资产、备份与传播状态</div>
        </div>
        <div class="header-actions">
          <el-tag v-if="auth.user?.role" :type="auth.user.role === 'viewer' ? 'info' : 'success'">{{ roleLabel }}</el-tag>
          <el-tag>{{ auth.user?.email }}</el-tag>
          <el-button text @click="signOut">退出</el-button>
        </div>
      </el-header>
      <el-main class="layout-main">
        <slot />
      </el-main>
    </el-container>
  </el-container>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useAuthStore } from '../stores/auth'

const route = useRoute()
const router = useRouter()
const auth = useAuthStore()

const activeMenu = computed(() => {
  if (route.path.startsWith('/domains/') && route.path.includes('/propagation')) return '/propagation'
  if (route.path.startsWith('/domains')) return '/domains'
  if (route.path.startsWith('/notifications')) return '/notifications'
  if (route.path.startsWith('/backups')) return '/backups'
  if (route.path.startsWith('/propagation')) return '/propagation'
  if (route.path.startsWith('/webhooks')) return '/webhooks'
  if (route.path.startsWith('/users')) return '/users'
  return '/dashboard'
})

const title = computed(() => {
  if (route.path.startsWith('/domains/') && route.path.includes('/propagation')) return '传播监控'
  if (route.path.startsWith('/domains')) return '域名管理'
  if (route.path.startsWith('/notifications')) return '通知中心'
  if (route.path.startsWith('/backups')) return '备份中心'
  if (route.path.startsWith('/propagation')) return '传播监控'
  if (route.path.startsWith('/webhooks')) return 'Webhook 管理'
  if (route.path.startsWith('/users')) return '用户管理'
  return '运营总览'
})

const roleLabel = computed(() => {
  if (auth.user?.role === 'admin') return '管理员'
  if (auth.user?.role === 'editor') return '编辑者'
  if (auth.user?.role === 'viewer') return '只读访客'
  return '未登录'
})

async function signOut() {
  await auth.signOut()
  router.push({ name: 'login' })
}
</script>

<style scoped>
.layout-shell {
  min-height: 100vh;
}
.layout-sidebar {
  background: #0f172a;
  color: #fff;
}
.brand {
  padding: 20px;
  font-size: 20px;
  font-weight: 700;
}
.menu {
  border-right: none;
}
.layout-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  background: #fff;
  border-bottom: 1px solid #e5e7eb;
}
.title {
  font-size: 20px;
  font-weight: 600;
}
.subtitle {
  color: #6b7280;
  font-size: 13px;
}
.header-actions {
  display: flex;
  gap: 12px;
  align-items: center;
}
.layout-main {
  background: #f8fafc;
}
</style>
