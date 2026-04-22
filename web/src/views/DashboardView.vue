<template>
  <AppLayout>
    <el-row :gutter="16">
      <el-col :span="6">
        <el-card>
          <el-statistic title="账户总数" :value="store.summary?.accounts ?? 0" />
        </el-card>
      </el-col>
      <el-col :span="6">
        <el-card>
          <el-statistic title="域名总数" :value="store.summary?.domains ?? 0" />
        </el-card>
      </el-col>
      <el-col :span="6">
        <el-card>
          <el-statistic title="星标域名" :value="store.summary?.starredDomains ?? 0" />
        </el-card>
      </el-col>
      <el-col :span="6">
        <el-card>
          <el-statistic title="即将过期凭证" :value="store.summary?.expiringAccounts ?? 0" />
        </el-card>
      </el-col>
    </el-row>

    <el-row :gutter="16" class="section">
      <el-col :span="14">
        <el-card>
          <template #header>
            <div class="card-header">
              <span>账户状态</span>
              <span class="muted-text">共 {{ filteredAccounts.length }} 个可见账户</span>
            </div>
          </template>

          <el-row :gutter="12" class="account-summary-row">
            <el-col :md="6" :sm="12" :xs="24">
              <div class="summary-block">
                <div class="summary-label">展示账户</div>
                <div class="summary-value">{{ filteredAccounts.length }}</div>
              </div>
            </el-col>
            <el-col :md="6" :sm="12" :xs="24">
              <div class="summary-block">
                <div class="summary-label">已校验</div>
                <div class="summary-value success-text">{{ validAccountCount }}</div>
              </div>
            </el-col>
            <el-col :md="6" :sm="12" :xs="24">
              <div class="summary-block">
                <div class="summary-label">待处理异常</div>
                <div class="summary-value danger-text">{{ attentionAccountCount }}</div>
              </div>
            </el-col>
            <el-col :md="6" :sm="12" :xs="24">
              <div class="summary-block">
                <div class="summary-label">过期提醒</div>
                <div class="summary-value warning-text">{{ expiringAccountCount }}</div>
              </div>
            </el-col>
          </el-row>

          <el-row :gutter="12" class="filter-row">
            <el-col :md="8" :sm="12" :xs="24">
              <el-input v-model="searchKeyword" clearable placeholder="搜索账户名称或 Provider" />
            </el-col>
            <el-col :md="5" :sm="12" :xs="24">
              <el-select v-model="providerFilter" clearable placeholder="全部 Provider" style="width: 100%">
                <el-option v-for="item in accountProviders" :key="item" :label="item" :value="item" />
              </el-select>
            </el-col>
            <el-col :md="5" :sm="12" :xs="24">
              <el-select v-model="credentialFilter" clearable placeholder="全部凭证状态" style="width: 100%">
                <el-option label="已校验" value="valid" />
                <el-option label="待校验" value="pending" />
                <el-option label="校验失败" value="invalid" />
                <el-option label="未知" value="unknown" />
              </el-select>
            </el-col>
            <el-col :md="4" :sm="12" :xs="24">
              <el-select v-model="reminderFilter" clearable placeholder="全部提醒" style="width: 100%">
                <el-option label="有提醒" value="attention" />
                <el-option label="正常" value="normal" />
              </el-select>
            </el-col>
            <el-col :md="2" :sm="12" :xs="24">
              <el-button style="width: 100%" @click="resetFilters">重置</el-button>
            </el-col>
          </el-row>

          <el-table :data="filteredAccounts" border>
            <el-table-column prop="name" label="账户" min-width="180" />
            <el-table-column prop="provider" label="Provider" width="120" />
            <el-table-column prop="domainCount" label="域名数" width="100" />
            <el-table-column label="凭证状态" min-width="240">
              <template #default="{ row }">
                <el-space wrap>
                  <el-tag :type="credentialTagType(row.credentialStatus)">{{ credentialLabel(row.credentialStatus) }}</el-tag>
                  <span v-if="row.lastCheckedAt" class="muted-text">校验 {{ formatDateTime(row.lastCheckedAt) }}</span>
                </el-space>
                <div v-if="row.lastValidationError" class="validation-error">{{ row.lastValidationError }}</div>
              </template>
            </el-table-column>
            <el-table-column label="提醒" width="140">
              <template #default="{ row }">
                <el-tag :type="reminderTagType(row.reminder)">{{ reminderLabel(row.reminder) }}</el-tag>
              </template>
            </el-table-column>
            <el-table-column label="轮换时间" width="180">
              <template #default="{ row }">
                {{ row.lastRotatedAt ? formatDateTime(row.lastRotatedAt) : '未轮换' }}
              </template>
            </el-table-column>
            <el-table-column label="过期时间" width="180">
              <template #default="{ row }">
                {{ row.expiresAt ? formatDateTime(row.expiresAt) : '未设置' }}
              </template>
            </el-table-column>
            <el-table-column label="操作" min-width="230" fixed="right">
              <template #default="{ row }">
                <el-space wrap>
                  <el-button text type="primary" :loading="validatingAccountId === row.id" @click="runValidation(row)">
                    立即校验
                  </el-button>
                  <template v-if="editable">
                    <el-button text @click="openEditDialog(row)">编辑信息</el-button>
                    <el-button text type="warning" @click="openRotateDialog(row)">轮换凭证</el-button>
                  </template>
                </el-space>
              </template>
            </el-table-column>
          </el-table>
        </el-card>
      </el-col>
      <el-col :span="10">
        <el-card>
          <template #header>
            <div class="card-header">
              <span>过期提醒</span>
              <el-button text @click="openNotifications">进入通知中心</el-button>
            </div>
          </template>
          <el-empty v-if="!store.reminders.length" description="暂无需要提醒的凭证" />
          <el-timeline v-else>
            <el-timeline-item v-for="item in store.reminders.slice(0, 6)" :key="`${item.accountId}-${item.severity}-${item.expiresAt || ''}`" :type="timelineType(item.severity)">
              <div class="timeline-title">{{ item.name }} · {{ item.provider }}</div>
              <div class="timeline-text">{{ reminderStatusText(item) }}，剩余 {{ item.daysLeft }} 天</div>
            </el-timeline-item>
          </el-timeline>
        </el-card>
      </el-col>
    </el-row>

    <el-row :gutter="16" class="section">
      <el-col :span="24">
        <el-card header="最近域名状态">
          <el-table :data="store.domains.slice(0, 8)" border>
            <el-table-column prop="name" label="域名" min-width="220" />
            <el-table-column prop="provider" label="Provider" width="120" />
            <el-table-column prop="accountName" label="账户" min-width="180" />
            <el-table-column label="传播状态" width="180">
              <template #default="{ row }">
                <PropagationStatusTag :status="row.lastPropagationStatus" />
              </template>
            </el-table-column>
            <el-table-column label="最近同步" width="180">
              <template #default="{ row }">
                {{ row.lastSyncedAt ? formatDateTime(row.lastSyncedAt) : '未同步' }}
              </template>
            </el-table-column>
          </el-table>
        </el-card>
      </el-col>
    </el-row>

    <el-dialog :model-value="showEditDialog" title="编辑账户信息" width="520px" @close="closeEditDialog">
      <el-form label-width="110px">
        <el-form-item label="账户名称">
          <el-input v-model="editForm.name" placeholder="例如：Cloudflare 主账号" />
        </el-form-item>
        <el-form-item label="Provider">
          <el-input :model-value="editTarget?.provider || ''" disabled />
        </el-form-item>
        <el-form-item label="过期时间">
          <el-input v-model="editForm.expiresAt" placeholder="RFC3339，可留空" />
        </el-form-item>
        <el-alert
          title="此处仅更新账户名称和过期时间，不会回显或修改已有凭证内容。"
          type="info"
          :closable="false"
          show-icon
        />
      </el-form>
      <template #footer>
        <el-button @click="closeEditDialog">取消</el-button>
        <el-button type="primary" :loading="savingEdit" @click="saveAccountMetadata">保存</el-button>
      </template>
    </el-dialog>

    <el-dialog :model-value="showRotateDialog" title="轮换凭证" width="640px" @close="closeRotateDialog">
      <el-form label-width="120px">
        <el-form-item label="账户名称">
          <el-input v-model="rotationForm.name" />
        </el-form-item>
        <el-form-item label="Provider">
          <el-input :model-value="rotationTarget?.provider || ''" disabled />
        </el-form-item>
        <template v-if="selectedProvider">
          <el-form-item v-for="field in selectedProvider.fields" :key="field.key" :label="field.label" :required="field.required">
            <el-switch v-if="field.type === 'boolean'" v-model="rotationConfig[field.key]" />
            <el-input-number v-else-if="field.type === 'number'" v-model="rotationConfig[field.key]" style="width: 100%" />
            <el-input
              v-else
              v-model="rotationConfig[field.key]"
              :type="field.type === 'password' ? 'password' : 'text'"
              :show-password="field.type === 'password'"
              :placeholder="field.placeholder"
            />
            <div v-if="field.helpText" class="field-help">{{ field.helpText }}</div>
          </el-form-item>
        </template>
        <el-form-item label="过期时间">
          <el-input v-model="rotationForm.expiresAt" placeholder="RFC3339，可留空" />
        </el-form-item>
        <el-alert
          title="提交后会立即重新校验凭证并同步域名。旧凭证不会回显，请输入新的完整配置。"
          type="warning"
          :closable="false"
          show-icon
        />
      </el-form>
      <template #footer>
        <el-button @click="closeRotateDialog">取消</el-button>
        <el-button type="primary" :loading="rotating" @click="rotateCredentials">提交并校验</el-button>
      </template>
    </el-dialog>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import AppLayout from '../layouts/AppLayout.vue'
import PropagationStatusTag from '../components/PropagationStatusTag.vue'
import { listProviders, rotateAccountCredentials, updateAccount, validateAccount } from '../api/accounts'
import { useAuthStore } from '../stores/auth'
import { reminderStatusText, reminderTimelineType, useDomainStore } from '../stores/domains'
import type { Account, ProviderDescriptor } from '../types/domain'
import { canEdit } from '../utils/permissions'

const router = useRouter()
const store = useDomainStore()
const authStore = useAuthStore()
const providers = ref<ProviderDescriptor[]>([])
const providersLoading = ref(false)
const showRotateDialog = ref(false)
const showEditDialog = ref(false)
const rotating = ref(false)
const savingEdit = ref(false)
const validatingAccountId = ref<number | null>(null)
const rotationTarget = ref<Account | null>(null)
const editTarget = ref<Account | null>(null)
const searchKeyword = ref('')
const providerFilter = ref('')
const credentialFilter = ref('')
const reminderFilter = ref('')
const rotationForm = reactive({
  name: '',
  expiresAt: '',
})
const editForm = reactive({
  name: '',
  expiresAt: '',
})
const rotationConfig = reactive<Record<string, any>>({})

const editable = computed(() => canEdit(authStore.user))
const selectedProvider = computed(() => providers.value.find((item) => item.key === rotationTarget.value?.provider) ?? null)
const accountProviders = computed(() => [...new Set(store.accounts.map((item) => item.provider))].sort((a, b) => a.localeCompare(b)))
const filteredAccounts = computed(() =>
  store.accounts.filter((item) => {
    const keyword = searchKeyword.value.trim().toLowerCase()
    if (keyword) {
      const haystack = `${item.name} ${item.provider}`.toLowerCase()
      if (!haystack.includes(keyword)) return false
    }
    if (providerFilter.value && item.provider !== providerFilter.value) return false
    if (credentialFilter.value && item.credentialStatus !== credentialFilter.value) return false
    if (reminderFilter.value === 'attention' && !item.reminder) return false
    if (reminderFilter.value === 'normal' && item.reminder) return false
    return true
  }),
)
const validAccountCount = computed(() => filteredAccounts.value.filter((item) => item.credentialStatus === 'valid').length)
const attentionAccountCount = computed(() => filteredAccounts.value.filter((item) => item.credentialStatus === 'invalid' || item.credentialStatus === 'pending').length)
const expiringAccountCount = computed(() => filteredAccounts.value.filter((item) => Boolean(item.reminder)).length)

onMounted(async () => {
  await Promise.all([store.loadDashboard(), ensureProviders()])
})

async function ensureProviders() {
  if (providers.value.length || providersLoading.value) return
  providersLoading.value = true
  try {
    providers.value = await listProviders()
  } finally {
    providersLoading.value = false
  }
}

function openNotifications() {
  router.push({ name: 'notifications' })
}

function timelineType(severity: string) {
  return reminderTimelineType(severity)
}

function formatDateTime(value?: string) {
  return value ? new Date(value).toLocaleString() : '未设置'
}

function credentialLabel(status: string) {
  if (status === 'valid') return '已校验'
  if (status === 'invalid') return '校验失败'
  if (status === 'pending') return '待校验'
  return '未知'
}

function credentialTagType(status: string) {
  if (status === 'valid') return 'success'
  if (status === 'invalid') return 'danger'
  if (status === 'pending') return 'warning'
  return 'info'
}

function reminderLabel(reminder: string) {
  if (reminder === 'expired') return '已过期'
  if (reminder === 'critical') return '紧急'
  if (reminder === 'warning') return '警告'
  if (reminder === 'notice') return '提醒'
  return '正常'
}

function reminderTagType(reminder: string) {
  if (reminder === 'expired' || reminder === 'critical') return 'danger'
  if (reminder === 'warning') return 'warning'
  if (reminder === 'notice') return 'primary'
  return 'info'
}

function resetFilters() {
  searchKeyword.value = ''
  providerFilter.value = ''
  credentialFilter.value = ''
  reminderFilter.value = ''
}

function buildConfig(provider: ProviderDescriptor) {
  const config = { ...(provider.sampleConfig || {}) }
  for (const field of provider.fields) {
    if (config[field.key] !== undefined) continue
    if (field.defaultValue !== undefined) {
      config[field.key] = field.defaultValue
      continue
    }
    config[field.key] = field.type === 'boolean' ? false : ''
  }
  return config
}

function setRotationConfig(value: Record<string, any>) {
  for (const key of Object.keys(rotationConfig)) {
    delete rotationConfig[key]
  }
  Object.assign(rotationConfig, value)
}

function openEditDialog(account: Account) {
  editTarget.value = account
  editForm.name = account.name
  editForm.expiresAt = account.expiresAt || ''
  showEditDialog.value = true
}

function closeEditDialog() {
  showEditDialog.value = false
  editTarget.value = null
  editForm.name = ''
  editForm.expiresAt = ''
}

async function saveAccountMetadata() {
  if (!editTarget.value) return
  try {
    savingEdit.value = true
    await updateAccount(editTarget.value.id, {
      name: editForm.name,
      expiresAt: editForm.expiresAt || undefined,
    })
    ElMessage.success('账户信息已更新')
    closeEditDialog()
    await store.loadDashboard()
  } catch (error: any) {
    const message = error?.response?.data?.error || error?.message || '更新账户信息失败'
    ElMessage.error(message)
  } finally {
    savingEdit.value = false
  }
}

async function runValidation(account: Account) {
  try {
    validatingAccountId.value = account.id
    const result = await validateAccount(account.id)
    ElMessage.success(result.message || '凭证校验成功')
  } catch (error: any) {
    const message = error?.response?.data?.error || error?.message || '凭证校验失败'
    ElMessage.error(message)
  } finally {
    validatingAccountId.value = null
    await store.loadDashboard()
  }
}

async function openRotateDialog(account: Account) {
  await ensureProviders()
  rotationTarget.value = account
  rotationForm.name = account.name
  rotationForm.expiresAt = account.expiresAt || ''
  setRotationConfig(selectedProvider.value ? buildConfig(selectedProvider.value) : {})
  showRotateDialog.value = true
}

function closeRotateDialog() {
  showRotateDialog.value = false
  rotationTarget.value = null
  rotationForm.name = ''
  rotationForm.expiresAt = ''
  setRotationConfig({})
}

async function rotateCredentials() {
  if (!rotationTarget.value) return
  try {
    rotating.value = true
    const response = await rotateAccountCredentials(rotationTarget.value.id, {
      name: rotationForm.name,
      config: { ...rotationConfig },
      expiresAt: rotationForm.expiresAt || undefined,
    })
    ElMessage.success(response.validation?.message || '凭证已轮换并校验成功')
    closeRotateDialog()
    await store.loadDashboard()
  } catch (error: any) {
    const message = error?.response?.data?.error || error?.message || '凭证轮换失败'
    ElMessage.error(message)
    await store.loadDashboard()
  } finally {
    rotating.value = false
  }
}
</script>

<style scoped>
.section {
  margin-top: 16px;
}
.card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
}
.account-summary-row {
  margin-bottom: 16px;
}
.summary-block {
  border: 1px solid #e5e7eb;
  border-radius: 10px;
  padding: 12px;
  background: #f8fafc;
}
.summary-label {
  color: #64748b;
  font-size: 12px;
}
.summary-value {
  margin-top: 6px;
  font-size: 24px;
  font-weight: 600;
  color: #0f172a;
}
.filter-row {
  margin-bottom: 16px;
}
.timeline-title {
  font-weight: 600;
}
.timeline-text {
  color: #6b7280;
}
.muted-text {
  color: #6b7280;
  font-size: 12px;
}
.validation-error {
  margin-top: 6px;
  color: #dc2626;
  font-size: 12px;
  line-height: 1.4;
}
.field-help {
  margin-top: 6px;
  color: #64748b;
  font-size: 12px;
  line-height: 1.4;
}
.success-text {
  color: #16a34a;
}
.warning-text {
  color: #d97706;
}
.danger-text {
  color: #dc2626;
}
</style>
