<template>

  <a-form-item label="小说名关键词">
    <a-input v-model="data.keyword" placeholder="例如：斗破苍穹（匹配所有标题包含该关键词的文章）" :style="{ width: '420px' }" allow-clear />
  </a-form-item>

  <a-form-item label="章节号正则">
    <a-input v-model="data.chapter_pattern" :style="{ width: '420px' }" allow-clear
             placeholder="留空使用内置识别：第N章 / 中英文数字 / Chapter N / 行尾数字" />
    <template #extra>
      自定义时第一个捕获组须为章节号，例如 <code>(?:第)?(\d+)\s*[章节]</code>；识别不出章节号的文章将按时间排在末尾
    </template>
  </a-form-item>

  <a-form-item label=" ">
    <a-space direction="vertical" :size="6">
      <a-space>
        <a-button type="primary" :loading="previewLoading" :disabled="!data.keyword" @click="doPreview">
          <template #icon><icon-search /></template>
          预览匹配
        </a-button>
        <span class="text-xs text-gray-400">预览只读不改数据；确认合并在预览弹窗中二次确认</span>
      </a-space>
    </a-space>
  </a-form-item>

  <MergePreviewModal
    v-model:visible="previewVisible"
    :loading="previewLoading"
    :result="previewResult"
    :merging="merging"
    :merge-result="mergeResult"
    @confirm="onConfirm"
  />

</template>


<script setup>
import {inject, ref} from "vue";
import {Message} from "@arco-design/web-vue";
import {pluginNovelPreview, pluginNovelMerge} from "@/api/index.js";
import MergePreviewModal from "@/components/novel/MergePreviewModal.vue";

const data = inject("options")
const currentID = inject("currentID", ref(""))

const previewVisible = ref(false)
const previewLoading = ref(false)
const previewResult = ref(null)
const merging = ref(false)
const mergeResult = ref(null)

// 预览：直接用当前表单值（不要求先保存配置，避免预览到旧配置）
async function doPreview() {
  const keyword = (data.value?.keyword || "").trim()
  if (!keyword) {
    Message.warning("请先填写小说名关键词")
    return
  }
  previewLoading.value = true
  mergeResult.value = null
  try {
    const resp = await pluginNovelPreview(currentID.value, {keyword, pattern: data.value?.chapter_pattern || ""})
    previewResult.value = resp?.data ?? resp
    previewVisible.value = true
  } catch (e) {
    // 请求级错误由 axios 拦截器统一提示
  } finally {
    previewLoading.value = false
  }
}

// 确认合并：ids 为预览弹窗勾选项（按预览顺序），所见即所得
async function onConfirm(ids) {
  merging.value = true
  try {
    const resp = await pluginNovelMerge(currentID.value, {
      keyword: (data.value?.keyword || "").trim(),
      pattern: data.value?.chapter_pattern || "",
      ids,
    })
    mergeResult.value = resp?.data ?? resp
    Message.success("合并完成")
  } catch (e) {
    // 失败保持预览态，可调整后重试
  } finally {
    merging.value = false
  }
}
</script>
