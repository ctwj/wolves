<template>
  <div>
    <a-alert class="mb-2" type="warning">
      在此编辑任务 JSON 后，请点击「解析并应用」同步到「采集任务」表单；切换页签时也会自动尝试应用。任务 JSON 与两个爬虫插件配置模型对齐，可平移复用。
    </a-alert>
    <a-textarea v-model="data.tasks" class="font-mono text-xs" placeholder="[]" :auto-size="{ minRows: 12, maxRows: 24 }" />
    <a-space class="mt-2">
      <a-button size="small" type="primary" @click="$emit('apply')">
        <template #icon><icon-check :size="14" /></template>
        解析并应用
      </a-button>
      <a-button size="small" @click="$emit('format')">格式化 / 校验</a-button>
      <a-typography-text type="secondary" class="text-xs">{{ summary }}</a-typography-text>
    </a-space>
  </div>
</template>

<script setup>
// HttpSpider / HeadlessSpider 共用的「JSON 源码」页签；
// data 为插件 options 对象（直接读写其 tasks 字段），apply/format 由页面组合式函数提供
defineProps({
  data: { type: Object, required: true },
  summary: { type: String, default: "" },
});

defineEmits(["apply", "format"]);
</script>
