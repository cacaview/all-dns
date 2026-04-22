<template>
  <div class="login-page">
    <el-card class="login-card">
      <template #header>
        <div class="title">DNS Hub</div>
        <div class="subtitle">全球多云 DNS 聚合管理平台</div>
      </template>

      <el-alert
        title="当前支持 GitHub / GitLab OAuth，也支持先使用 Mock Provider 完成联调。"
        type="info"
        :closable="false"
        class="login-alert"
      />

      <el-space direction="vertical" fill>
        <el-button type="primary" size="large" @click="goOAuth('github')">GitHub 登录</el-button>
        <el-button size="large" @click="goOAuth('gitlab')">GitLab 登录</el-button>
        <el-button type="success" size="large" @click="goDevLogin">演示登录</el-button>
      </el-space>

      <div class="hint">
        登录成功后会自动回跳，并在本地保存访问令牌。
      </div>
    </el-card>
  </div>
</template>

<script setup lang="ts">
import { ElMessage } from 'element-plus'
import { useRouter } from 'vue-router'
import { useAuthStore } from '../stores/auth'

const router = useRouter()
const auth = useAuthStore()

function goOAuth(provider: 'github' | 'gitlab') {
  window.location.href = `/api/v1/auth/oauth/${provider}/login`
}

async function goDevLogin() {
  try {
    await auth.signInDev()
    await router.push({ name: 'dashboard' })
  } catch (error: any) {
    ElMessage.error(error?.message || '演示登录失败')
  }
}
</script>

<style scoped>
.login-page {
  min-height: 100vh;
  display: flex;
  align-items: center;
  justify-content: center;
  background: linear-gradient(135deg, #0f172a, #1d4ed8);
}
.login-card {
  width: 460px;
}
.title {
  font-size: 28px;
  font-weight: 700;
}
.subtitle,
.hint {
  color: #6b7280;
}
.login-alert {
  margin-bottom: 20px;
}
</style>
