<template>
  <el-drawer :model-value="modelValue" :title="domain ? `${domain.name} 业务档案` : '业务档案'" size="40%" @close="emit('update:modelValue', false)">
    <el-form label-position="top">
      <el-alert v-if="!props.editable" title="当前为只读模式，可查看档案和附件，但不能修改内容。" type="info" :closable="false" style="margin-bottom: 16px" />
      <el-form-item label="Markdown 描述">
        <el-input v-model="form.description" type="textarea" :rows="12" :disabled="!props.editable" />
      </el-form-item>
      <el-form-item label="附件">
        <el-space direction="vertical" alignment="stretch" style="width: 100%">
          <el-upload :auto-upload="false" :show-file-list="false" :disabled="uploading || !domain || !props.editable" accept="*/*" @change="handleFileChange">
            <el-button :loading="uploading" :disabled="!domain || !props.editable">上传附件</el-button>
          </el-upload>
          <el-input v-model="attachmentsText" type="textarea" :rows="6" :disabled="!props.editable" placeholder="支持上传生成链接，也可手动粘贴 URL，每行一条" />
          <el-empty v-if="!attachmentList.length" description="暂无附件" />
          <el-space v-else wrap>
            <el-tag v-for="item in attachmentList" :key="item" :closable="Boolean(props.editable)" @close="removeAttachment(item)">
              <a :href="item" target="_blank" rel="noreferrer">{{ attachmentName(item) }}</a>
            </el-tag>
          </el-space>
        </el-space>
      </el-form-item>
    </el-form>

    <el-divider>预览</el-divider>
    <div class="preview" v-html="previewHtml"></div>

    <template #footer>
      <div class="drawer-footer">
        <el-button @click="emit('update:modelValue', false)">关闭</el-button>
        <el-button type="primary" :loading="saving" :disabled="!props.editable" @click="save">保存</el-button>
      </div>
    </template>
  </el-drawer>
</template>

<script setup lang="ts">
import { computed, reactive, ref, watch } from 'vue'
import type { UploadFile } from 'element-plus'
import MarkdownIt from 'markdown-it'
import { ElMessage } from 'element-plus'
import { fetchProfile, updateProfile, uploadProfileAttachment } from '../api/domains'
import type { Domain } from '../types/domain'

const md = new MarkdownIt({ linkify: true, breaks: true })

const props = defineProps<{
  modelValue: boolean
  domain: Domain | null
  editable?: boolean
}>()
const emit = defineEmits<{
  (e: 'update:modelValue', value: boolean): void
  (e: 'saved'): void
}>()

const form = reactive({
  description: '',
})
const attachmentsText = ref('')
const saving = ref(false)
const uploading = ref(false)

const attachmentList = computed(() =>
  attachmentsText.value
    .split('\n')
    .map((item) => item.trim())
    .filter(Boolean),
)
const previewHtml = computed(() => md.render(form.description || '暂无描述'))

watch(
  () => props.modelValue,
  async (open) => {
    if (!open || !props.domain) return
    try {
      const profile = await fetchProfile(props.domain.id)
      form.description = profile.description
      attachmentsText.value = profile.attachmentUrls.join('\n')
    } catch (error: any) {
      ElMessage.error(error?.message || '加载档案失败')
    }
  },
)

function setAttachments(items: string[]) {
  attachmentsText.value = items.join('\n')
}

function removeAttachment(item: string) {
  if (!props.editable) return
  setAttachments(attachmentList.value.filter((current) => current !== item))
}

function attachmentName(url: string) {
  const parts = url.split('/')
  return parts[parts.length - 1] || url
}

async function handleFileChange(uploadFile: UploadFile) {
  if (!props.domain || !uploadFile.raw || !props.editable) return
  uploading.value = true
  try {
    const uploaded = await uploadProfileAttachment(props.domain.id, uploadFile.raw)
    setAttachments([...attachmentList.value, uploaded.url])
    ElMessage.success('附件上传成功')
  } catch (error: any) {
    ElMessage.error(error?.message || '上传附件失败')
  } finally {
    uploading.value = false
  }
}

async function save() {
  if (!props.domain || !props.editable) return
  saving.value = true
  try {
    await updateProfile(props.domain.id, {
      description: form.description,
      attachmentUrls: attachmentList.value,
    })
    ElMessage.success('档案已保存')
    emit('saved')
    emit('update:modelValue', false)
  } catch (error: any) {
    ElMessage.error(error?.message || '保存档案失败')
  } finally {
    saving.value = false
  }
}
</script>

<style scoped>
.preview {
  padding: 12px;
  background: #f8fafc;
  border: 1px solid #e5e7eb;
  border-radius: 8px;
}
.drawer-footer {
  display: flex;
  justify-content: flex-end;
  gap: 12px;
}
:deep(.el-tag a) {
  color: inherit;
  text-decoration: none;
}
</style>
