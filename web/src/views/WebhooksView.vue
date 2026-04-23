<template>
  <AppLayout>
    <el-card shadow="never">
      <template #header>
        <div class="toolbar">
          <div>
            <div class="page-title">Webhook 管理</div>
            <div class="page-subtitle">配置过期提醒通知的 Webhook 接收端点</div>
          </div>
          <el-button type="primary" @click="openCreateDialog">添加 Webhook</el-button>
        </div>
      </template>

      <el-empty v-if="!webhooks.length" description="暂无 Webhook 配置" />
      <el-table v-else :data="webhooks" border>
        <el-table-column prop="name" label="名称" min-width="160" />
        <el-table-column prop="url" label="URL" min-width="300" show-overflow-tooltip />
        <el-table-column label="事件" min-width="200">
          <template #default="{ row }">
            <el-space wrap>
              <el-tag v-for="event in row.events" :key="event" size="small" type="info">{{ eventLabel(event) }}</el-tag>
            </el-space>
          </template>
        </el-table-column>
        <el-table-column label="状态" width="100">
          <template #default="{ row }">
            <el-tag :type="row.active ? 'success' : 'info'">{{ row.active ? '启用' : '禁用' }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="创建时间" width="180">
          <template #default="{ row }">{{ formatDateTime(row.createdAt) }}</template>
        </el-table-column>
        <el-table-column label="操作" width="140" fixed="right">
          <template #default="{ row }">
            <el-space>
              <el-button size="small" @click="openEditDialog(row)">编辑</el-button>
              <el-button size="small" type="danger" plain @click="handleDelete(row)">删除</el-button>
            </el-space>
          </template>
        </el-table-column>
      </el-table>
    </el-card>

    <el-dialog v-model="dialogVisible" :title="dialogTitle" width="500px" @close="resetForm">
      <el-form ref="formRef" :model="form" :rules="rules" label-width="80px">
        <el-form-item label="名称" prop="name">
          <el-input v-model="form.name" placeholder="例如：生产环境提醒" />
        </el-form-item>
        <el-form-item label="URL" prop="url">
          <el-input v-model="form.url" placeholder="https://example.com/webhook" />
        </el-form-item>
        <el-form-item label="事件">
          <el-checkbox-group v-model="form.events">
            <el-checkbox label="credential_expiry">凭证过期提醒</el-checkbox>
          </el-checkbox-group>
        </el-form-item>
        <el-form-item label="启用">
          <el-switch v-model="form.active" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="saving" @click="handleSave">保存</el-button>
      </template>
    </el-dialog>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import AppLayout from '../layouts/AppLayout.vue'
import { listWebhooks, createWebhook, updateWebhook, deleteWebhook } from '../api/webhooks'
import type { Webhook } from '../api/webhooks'

const webhooks = ref<Webhook[]>([])
const dialogVisible = ref(false)
const saving = ref(false)
const editingId = ref<number | null>(null)
const formRef = ref()

const form = ref({
  name: '',
  url: '',
  events: ['credential_expiry'] as string[],
  active: true,
})

const rules = {
  name: [{ required: true, message: '请输入名称', trigger: 'blur' }],
  url: [
    { required: true, message: '请输入 URL', trigger: 'blur' },
    { type: 'url', message: '请输入有效的 URL', trigger: 'blur' },
  ],
}

const dialogTitle = computed(() => (editingId.value ? '编辑 Webhook' : '添加 Webhook'))

async function loadWebhooks() {
  try {
    webhooks.value = await listWebhooks()
  } catch (e: any) {
    ElMessage.error(e.response?.data?.error || '加载失败')
  }
}

function openCreateDialog() {
  editingId.value = null
  form.value = { name: '', url: '', events: ['credential_expiry'], active: true }
  dialogVisible.value = true
}

function openEditDialog(row: Webhook) {
  editingId.value = row.id
  form.value = {
    name: row.name,
    url: row.url,
    events: row.events?.length ? row.events : ['credential_expiry'],
    active: row.active,
  }
  dialogVisible.value = true
}

async function handleSave() {
  const valid = await formRef.value?.validate().catch(() => false)
  if (!valid) return
  saving.value = true
  try {
    if (editingId.value) {
      await updateWebhook(editingId.value, form.value)
      ElMessage.success('更新成功')
    } else {
      await createWebhook(form.value)
      ElMessage.success('创建成功')
    }
    dialogVisible.value = false
    await loadWebhooks()
  } catch (e: any) {
    ElMessage.error(e.response?.data?.error || '保存失败')
  } finally {
    saving.value = false
  }
}

async function handleDelete(row: Webhook) {
  try {
    await ElMessageBox.confirm(`确定删除 Webhook「${row.name}」？`, '删除确认', { type: 'warning' })
    await deleteWebhook(row.id)
    ElMessage.success('删除成功')
    await loadWebhooks()
  } catch (e: any) {
    if (e !== 'cancel') {
      ElMessage.error(e.response?.data?.error || '删除失败')
    }
  }
}

function resetForm() {
  formRef.value?.resetFields()
}

function eventLabel(event: string) {
  if (event === 'credential_expiry') return '凭证过期'
  return event
}

function formatDateTime(value: string) {
  return value ? new Date(value).toLocaleString() : '—'
}

loadWebhooks()
</script>

<style scoped>
.toolbar {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 16px;
}

.page-title {
  font-size: 18px;
  font-weight: 600;
}

.page-subtitle {
  color: #6b7280;
  font-size: 13px;
}
</style>
