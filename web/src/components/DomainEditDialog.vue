<template>
  <el-dialog :model-value="modelValue" :title="domain ? `编辑解析 - ${domain.name}` : '编辑解析'" width="960px" @close="emit('update:modelValue', false)">
    <el-row justify="space-between" class="toolbar">
      <el-col>
        <el-space direction="vertical" alignment="flex-start" :size="4">
          <el-text type="info">所有变更都会先自动生成快照备份。</el-text>
          <el-text v-if="lastBackup" type="success">最近快照：#{{ lastBackup.id }} · {{ formatTime(lastBackup.createdAt) }}</el-text>
        </el-space>
      </el-col>
      <el-col>
        <el-space>
          <el-button @click="loadBackups" :disabled="!domain">刷新快照</el-button>
          <el-button @click="loadRecords" :disabled="!domain">刷新记录</el-button>
        </el-space>
      </el-col>
    </el-row>

    <el-alert v-if="!props.editable" title="当前为只读模式，可查看记录、传播结果和快照，但不能保存或删除解析。" type="info" :closable="false" class="status-alert" />

    <el-alert v-if="lastPropagationLabel" :title="lastPropagationLabel" :type="lastPropagationAlertType" :closable="false" class="status-alert" />
    <el-descriptions v-if="lastPropagationResults.length" :column="1" border class="status-details">
      <el-descriptions-item v-for="item in lastPropagationResults" :key="item.resolver" :label="item.resolver">
        <el-space wrap>
          <el-tag :type="item.matched ? 'success' : item.status === 'ok' ? 'warning' : 'danger'" size="small">
            {{ item.matched ? '已命中' : item.status === 'ok' ? '待生效' : '异常' }}
          </el-tag>
          <span>{{ item.reason }}</span>
          <span v-if="item.answers?.length" class="answer-text">{{ item.answers.join('，') }}</span>
        </el-space>
      </el-descriptions-item>
    </el-descriptions>

    <el-table :data="records" v-loading="loading" border>
      <el-table-column prop="type" label="类型" width="100" />
      <el-table-column prop="name" label="主机记录" width="180" />
      <el-table-column prop="content" label="值" min-width="220" />
      <el-table-column prop="ttl" label="TTL" width="100" />
      <el-table-column label="操作" width="160">
        <template #default="{ row }">
          <el-button size="small" @click="fillEditor(row)">{{ props.editable ? '更新' : '查看' }}</el-button>
          <el-button v-if="props.editable" size="small" type="danger" plain @click="removeRecord(row)">删除</el-button>
        </template>
      </el-table-column>
    </el-table>

    <el-divider>新增 / 更新记录</el-divider>
    <el-form label-width="100px">
      <el-form-item label="记录 ID">
        <el-input v-model="editor.id" placeholder="留空表示新增" :disabled="!props.editable" />
      </el-form-item>
      <el-form-item label="类型">
        <el-select v-model="editor.type" style="width: 100%" :disabled="!props.editable">
          <el-option label="A" value="A" />
          <el-option label="AAAA" value="AAAA" />
          <el-option label="CNAME" value="CNAME" />
          <el-option label="TXT" value="TXT" />
          <el-option label="MX" value="MX" />
        </el-select>
      </el-form-item>
      <el-form-item label="主机记录">
        <el-input v-model="editor.name" placeholder="@ / www" :disabled="!props.editable" />
      </el-form-item>
      <el-form-item label="值">
        <el-input v-model="editor.content" placeholder="203.0.113.10" :disabled="!props.editable" />
      </el-form-item>
      <el-form-item label="TTL">
        <el-input-number v-model="editor.ttl" :min="1" :max="86400" :disabled="!props.editable" />
      </el-form-item>
      <el-form-item label="备注">
        <el-input v-model="editor.comment" :disabled="!props.editable" />
      </el-form-item>
    </el-form>

    <el-divider>最近快照</el-divider>
    <el-empty v-if="!backups.length" description="暂无快照" />
    <el-timeline v-else>
      <el-timeline-item v-for="item in backups" :key="item.id" :timestamp="formatTime(item.createdAt)">
        <div class="backup-title">#{{ item.id }} · {{ item.reason }}</div>
        <div class="backup-text">包含 {{ item.content?.records?.length ?? 0 }} 条记录</div>
      </el-timeline-item>
    </el-timeline>

    <template #footer>
      <el-button @click="emit('update:modelValue', false)">关闭</el-button>
      <el-button type="primary" :disabled="!domain || !props.editable" :loading="saving" @click="applyRecord(editor)">保存记录</el-button>
    </template>
  </el-dialog>
</template>

<script setup lang="ts">
import { computed, reactive, ref, watch } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { deleteRecord, listBackups, listRecords, upsertRecord } from '../api/domains'
import type { Backup, DNSRecord, Domain, PropagationStatus } from '../types/domain'

const props = defineProps<{
  modelValue: boolean
  domain: Domain | null
  editable?: boolean
}>()
const emit = defineEmits<{
  (e: 'update:modelValue', value: boolean): void
  (e: 'saved'): void
}>()

const loading = ref(false)
const saving = ref(false)
const records = ref<DNSRecord[]>([])
const backups = ref<Backup[]>([])
const lastPropagationLabel = ref('')
const lastPropagationAlertType = ref<'info' | 'success' | 'warning' | 'error'>('info')
const lastPropagationResults = ref<Array<{ resolver: string; status: string; matched: boolean; reason: string; answers: string[] }>>([])
const lastBackup = computed(() => backups.value[0] ?? null)
const editor = reactive<DNSRecord>({
  id: '',
  type: 'A',
  name: '@',
  content: '',
  ttl: 300,
  comment: '',
})

watch(
  () => props.modelValue,
  (open) => {
    if (open && props.domain) {
      loadRecords()
      loadBackups()
      setPropagationStatus(props.domain.lastPropagationStatus)
      resetEditor()
    }
  },
)

async function loadRecords() {
  if (!props.domain) return
  loading.value = true
  try {
    records.value = await listRecords(props.domain.id)
  } catch (error: any) {
    ElMessage.error(error?.message || '加载记录失败')
  } finally {
    loading.value = false
  }
}

async function loadBackups() {
  if (!props.domain) return
  try {
    backups.value = await listBackups(props.domain.id)
  } catch (error: any) {
    ElMessage.error(error?.message || '加载快照失败')
  }
}

function fillEditor(record: DNSRecord) {
  editor.id = record.id
  editor.type = record.type
  editor.name = record.name
  editor.content = record.content
  editor.ttl = record.ttl
  editor.comment = record.comment || ''
}

function formatTime(value?: string) {
  return value ? new Date(value).toLocaleString() : '未知时间'
}

function propagationSummary(payload?: PropagationStatus) {
  if (!payload) return ''
  return payload.summary || ''
}

function propagationAlertType(payload?: PropagationStatus): 'info' | 'success' | 'warning' | 'error' {
  if (!payload) return 'info'
  if (payload.overallStatus === 'verified') return 'success'
  if (payload.overallStatus === 'failed') return 'error'
  if (payload.overallStatus === 'partial') return 'warning'
  return 'info'
}

function propagationReasonLabel(reason: string) {
  if (reason === 'matched') return '目标值已返回'
  if (reason === 'resolver_error') return '解析器请求失败'
  if (reason === 'no_answer') return '暂无解析结果'
  if (reason === 'value_mismatch') return '返回值与目标不一致'
  return reason || '未知状态'
}

function syncLatestBackup(backup: Backup) {
  backups.value = [backup, ...backups.value.filter((item) => item.id !== backup.id)]
}

function setPropagationStatus(payload?: PropagationStatus) {
  lastPropagationLabel.value = propagationSummary(payload)
  lastPropagationAlertType.value = propagationAlertType(payload)
  lastPropagationResults.value = (payload?.results || []).map((item) => ({
    resolver: item.resolver,
    status: item.status,
    matched: item.matched,
    reason: propagationReasonLabel(item.reason),
    answers: item.answers || [],
  }))
}

async function applyRecord(record: DNSRecord) {
  if (!props.domain || !props.editable) return
  saving.value = true
  try {
    const result = await upsertRecord(props.domain.id, record)
    setPropagationStatus(result.propagation)
    syncLatestBackup(result.backup)
    ElMessage.success('记录已保存并触发传播检查')
    resetEditor()
    await loadRecords()
    emit('saved')
  } catch (error: any) {
    ElMessage.error(error?.message || '保存记录失败')
  } finally {
    saving.value = false
  }
}

async function removeRecord(record: DNSRecord) {
  if (!props.domain || !record.id || !props.editable) return
  await ElMessageBox.confirm(`确认删除记录 ${record.name} ${record.type} 吗？`, '删除记录', { type: 'warning' })
  try {
    const backup = await deleteRecord(props.domain.id, record.id)
    setPropagationStatus(undefined)
    syncLatestBackup(backup)
    ElMessage.success('记录已删除，已保留删除前快照')
    await loadRecords()
    emit('saved')
  } catch (error: any) {
    ElMessage.error(error?.message || '删除失败')
  }
}

function resetEditor() {
  editor.id = ''
  editor.type = 'A'
  editor.name = '@'
  editor.content = ''
  editor.ttl = 300
  editor.comment = ''
}
</script>

<style scoped>
.toolbar {
  margin-bottom: 12px;
}
.status-alert {
  margin-bottom: 12px;
}
.status-details {
  margin-bottom: 12px;
}
.answer-text {
  color: #6b7280;
}
.backup-title {
  font-weight: 600;
}
.backup-text {
  color: #6b7280;
}
</style>
