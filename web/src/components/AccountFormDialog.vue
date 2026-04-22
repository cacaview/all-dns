<template>
  <el-dialog :model-value="modelValue" title="新增账户" width="640px" @close="emit('update:modelValue', false)">
    <el-form label-width="120px">
      <el-form-item label="账户名称">
        <el-input v-model="form.name" placeholder="例如：Cloudflare 主账号" />
      </el-form-item>
      <el-form-item label="Provider">
        <el-select v-model="form.provider" style="width: 100%" :loading="providersLoading">
          <el-option v-for="item in providers" :key="item.key" :label="item.label" :value="item.key" />
        </el-select>
      </el-form-item>
      <el-alert v-if="selectedProvider?.description" :title="selectedProvider.description" type="info" :closable="false" show-icon style="margin-bottom: 18px" />
      <template v-if="selectedProvider">
        <el-form-item v-for="field in selectedProvider.fields" :key="field.key" :label="field.label" :required="field.required">
          <el-switch v-if="field.type === 'boolean'" v-model="configForm[field.key]" />
          <el-input-number v-else-if="field.type === 'number'" v-model="configForm[field.key]" style="width: 100%" />
          <el-input
            v-else
            v-model="configForm[field.key]"
            :type="field.type === 'password' ? 'password' : 'text'"
            :show-password="field.type === 'password'"
            :placeholder="field.placeholder"
          />
          <div v-if="field.helpText" class="field-help">{{ field.helpText }}</div>
        </el-form-item>
      </template>
      <el-form-item label="过期时间">
        <el-input v-model="form.expiresAt" placeholder="RFC3339，可留空" />
      </el-form-item>
    </el-form>
    <template #footer>
      <el-button @click="emit('update:modelValue', false)">取消</el-button>
      <el-button type="primary" :loading="saving" :disabled="props.editable === false" @click="save">保存</el-button>
    </template>
  </el-dialog>
</template>

<script setup lang="ts">
import { computed, reactive, ref, watch } from 'vue'
import { ElMessage } from 'element-plus'
import { createAccount, listProviders } from '../api/accounts'
import type { ProviderDescriptor } from '../types/domain'

const props = defineProps<{ modelValue: boolean; editable?: boolean }>()
const emit = defineEmits<{ (e: 'update:modelValue', value: boolean): void; (e: 'saved'): void }>()

const providers = ref<ProviderDescriptor[]>([])
const providersLoading = ref(false)
const saving = ref(false)
const form = reactive({
  name: '',
  provider: '',
  expiresAt: '',
})
const configForm = reactive<Record<string, any>>({})

const selectedProvider = computed(() => providers.value.find((item) => item.key === form.provider) ?? null)

watch(
  () => form.provider,
  (providerKey, previousKey) => {
    if (!providerKey || providerKey === previousKey) return
    const provider = providers.value.find((item) => item.key === providerKey)
    if (!provider) return
    setConfig(buildConfig(provider))
  },
)

watch(
  () => props.modelValue,
  async (open) => {
    if (!open) return
    await ensureProviders()
    resetForm()
  },
)

async function ensureProviders() {
  if (providers.value.length || providersLoading.value) return
  providersLoading.value = true
  try {
    providers.value = await listProviders()
  } catch (error: any) {
    ElMessage.error(error?.message || '加载 Provider 列表失败')
  } finally {
    providersLoading.value = false
  }
}

function resetForm() {
  form.name = ''
  form.expiresAt = ''
  form.provider = providers.value[0]?.key || ''
  if (selectedProvider.value) {
    setConfig(buildConfig(selectedProvider.value))
  } else {
    setConfig({})
  }
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

function setConfig(value: Record<string, any>) {
  for (const key of Object.keys(configForm)) {
    delete configForm[key]
  }
  Object.assign(configForm, value)
}

async function save() {
  if (props.editable === false) return
  if (!form.provider) {
    ElMessage.error('请选择 Provider')
    return
  }
  try {
    saving.value = true
    await createAccount({
      name: form.name,
      provider: form.provider,
      config: { ...configForm },
      expiresAt: form.expiresAt || undefined,
    })
    ElMessage.success('账户已保存')
    emit('saved')
    emit('update:modelValue', false)
  } catch (error: any) {
    ElMessage.error(error?.message || '保存失败')
  } finally {
    saving.value = false
  }
}
</script>

<style scoped>
.field-help {
  margin-top: 6px;
  color: #64748b;
  font-size: 12px;
  line-height: 1.4;
}
</style>
