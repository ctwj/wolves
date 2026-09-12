<template>
  <a-modal
    :visible="visible"
    :width="760"
    title-align="start"
    :footer="false"
    unmount-on-close
    @update:visible="$emit('update:visible', $event)"
  >
    <template #title>
      <span class="inline-flex items-center gap-2 min-w-0">
        任务校验
        <a-tag v-if="taskLabel" color="arcoblue" size="small" class="max-w-[300px] min-w-0" :title="taskLabel">
          <span class="truncate">{{ taskLabel }}</span>
        </a-tag>
        <a-tag color="green" size="small">不入库 · 仅校验</a-tag>
      </span>
    </template>

    <!-- 加载中 -->
    <div v-if="loading" class="py-8 text-center">
      <a-spin :size="28" tip="抓取列表页第一页并提取首篇文章…" />
    </div>

    <template v-else-if="result">
      <!-- 流程错误 -->
      <a-alert v-if="result.error" type="error" class="mb-3">
        {{ result.error }}
        <template v-if="result.links_found === 0 && result.list_url">
          <div class="text-xs mt-1 truncate" :title="result.list_url">列表页地址：{{ result.list_url }}</div>
        </template>
      </a-alert>

      <template v-else>
        <!-- 概要 -->
        <div class="grid grid-cols-3 gap-2 mb-3 text-center">
          <div class="rounded-lg border border-gray-100 py-2">
            <div class="text-lg font-semibold">{{ result.links_found }}</div>
            <div class="text-xs text-gray-400">列表页命中链接</div>
          </div>
          <div class="rounded-lg border border-gray-100 py-2">
            <div class="text-lg font-semibold">{{ result.article?.content_len || 0 }}</div>
            <div class="text-xs text-gray-400">正文字符数</div>
          </div>
          <div class="rounded-lg border border-gray-100 py-2">
            <div class="text-lg font-semibold">{{ result.cost_ms }}ms</div>
            <div class="text-xs text-gray-400">耗时</div>
          </div>
        </div>

        <!-- 列表页链接样本（单行不换行，超出省略，悬停看全文） -->
        <section class="v-card">
          <header class="v-title">列表页链接（前 5 条）</header>
          <div v-for="(l, i) in result.sample_links" :key="i" class="text-xs flex gap-2 items-baseline py-0.5">
            <span class="text-gray-400 shrink-0">#{{ i + 1 }}</span>
            <span class="truncate min-w-0 flex-1" :title="l.title">{{ l.title || '(无标题)' }}</span>
            <span class="truncate min-w-0 flex-1 text-right text-gray-400" style="font-size: 11px" :title="l.url">{{ l.url }}</span>
          </div>
          <div class="text-xs mt-1 flex items-baseline gap-1 overflow-hidden">
            <span class="text-gray-500 shrink-0">校验详情页：</span>
            <span class="truncate min-w-0" :title="result.first_link?.url">{{ result.first_link?.url }}</span>
          </div>
        </section>

        <!-- 文章字段 -->
        <section class="v-card">
          <header class="v-title">文章字段提取结果</header>
          <table class="w-full text-xs">
            <tbody>
              <tr v-for="row in fieldRows" :key="row.label" class="border-b border-gray-50 last:border-0">
                <td class="py-1.5 pr-2 text-gray-500 whitespace-nowrap align-top" style="width: 72px">{{ row.label }}</td>
                <td class="py-1.5 break-all">
                  <template v-if="row.type === 'text'">{{ row.value || '—' }}</template>
                  <template v-else-if="row.type === 'link'">
                    <a v-if="row.value" :href="row.value" target="_blank" rel="noopener" class="break-all">{{ row.value }}</a>
                    <span v-else>—</span>
                  </template>
                  <template v-else-if="row.type === 'list'">
                    <div v-for="(v, i) in row.value" :key="i" class="break-all">{{ v }}</div>
                    <span v-if="!row.value?.length">—</span>
                  </template>
                  <template v-else-if="row.type === 'videos'">
                    <div v-for="(v, i) in row.value" :key="i" class="flex gap-2">
                      <a-tag size="small" :color="v.embed ? 'orangered' : 'arcoblue'">{{ v.embed ? 'iframe' : '直链' }}</a-tag>
                      <span class="break-all">{{ v.label }}：{{ v.url }}</span>
                    </div>
                    <span v-if="!row.value?.length">—</span>
                  </template>
                </td>
                <td class="py-1.5 pl-2 text-right whitespace-nowrap align-top" style="width: 46px">
                  <icon-check-circle-fill v-if="row.ok" class="text-green-500" />
                  <icon-close-circle-fill v-else-if="row.warn" class="text-red-500" />
                  <icon-minus v-else class="text-gray-300" />
                </td>
              </tr>
            </tbody>
          </table>
        </section>

        <!-- 正文预览 -->
        <section v-if="result.article?.content_preview" class="v-card">
          <a-collapse :default-active-key="[]" expand-icon-position="left" :bordered="false">
            <a-collapse-item key="preview" header="正文预览（前 600 字，已去标签）">
              <div class="text-xs leading-6 whitespace-pre-wrap break-all font-mono">{{ result.article.content_preview }}</div>
            </a-collapse-item>
          </a-collapse>
        </section>
      </template>

      <!-- 未命中警告 -->
      <a-alert v-if="result.article?.missed?.length" type="warning" class="mt-3">
        以下已配置的选择器未命中（其余字段可能走了自动回退）：
        <a-tag v-for="m in result.article.missed" :key="m" color="orange" size="small" class="ml-1">{{ m }}</a-tag>
      </a-alert>
    </template>
  </a-modal>
</template>

<script setup>
import { computed } from "vue";

// HttpSpider / HeadlessSpider 共用的「任务校验」结果弹窗：
// 后端只提取不入库，字段行状态 ✓=提取到 / ✗=配置了选择器但未命中 / -=未配置
const props = defineProps({
  visible: { type: Boolean, default: false },
  loading: { type: Boolean, default: false },
  result: { type: Object, default: null },
  // 点击「验证」时的任务上下文 { index, name }：多任务时加载中/未命名也能标出是哪个任务
  task: { type: Object, default: null },
});

defineEmits(["update:visible"]);

// 任务名展示：后端返回优先（已清洗），回退点击时快照；多任务下用 #序号兜底区分未命名任务
const taskLabel = computed(() => {
  const idx = props.task?.index;
  const name = props.result?.task_name || props.task?.name || "";
  if (idx == null && !name) return "";
  const label = name || "未命名任务";
  return idx != null ? `#${idx + 1} ${label}` : label;
});

const missedSet = computed(() => new Set(props.result?.article?.missed || []));

const fieldRows = computed(() => {
  const a = props.result?.article;
  if (!a) return [];
  const rows = [
    { label: "标题", type: "text", value: a.title, ok: !!a.title, warn: false },
    { label: "封面", type: "link", value: a.cover, ok: !!a.cover, warn: missedSet.value.has("cover_sel") },
    { label: "关键词", type: "text", value: a.keywords, ok: !!a.keywords, warn: false },
    { label: "发布时间", type: "text", value: a.publish_time, ok: !!a.publish_time, warn: missedSet.value.has("publish_time_sel") },
    { label: "正文", type: "text", value: `${a.content_len} 字符`, ok: a.content_len > 0, warn: missedSet.value.has("content_sel") },
  ];
  if (a.video_sources?.length) {
    rows.push({ label: "播放源", type: "videos", value: a.video_sources, ok: true, warn: false });
  }
  if (a.gallery_images?.length) {
    rows.push({ label: "图集", type: "list", value: a.gallery_images, ok: true, warn: false });
  }
  for (const [k, v] of Object.entries(a.extras || {})) {
    const val = Array.isArray(v) ? v : [String(v)];
    rows.push({ label: k, type: "list", value: val, ok: val.length > 0 && val[0] !== "", warn: missedSet.value.has(`extra:${k}`) });
  }
  rows.push({ label: "Slug", type: "text", value: a.slug, ok: !!a.slug, warn: false });
  return rows;
});
</script>

<style scoped>
.v-card {
  border: 1px solid var(--color-border-2, #e5e6eb);
  border-radius: 8px;
  padding: 8px 12px;
  margin-bottom: 12px;
  background: var(--color-fill-1, #f7f8fa);
}

.v-title {
  font-size: 12px;
  font-weight: 600;
  letter-spacing: 0.05em;
  color: var(--color-text-2, #4e5969);
  margin-bottom: 6px;
  padding-left: 8px;
  border-left: 3px solid rgb(var(--primary-6, 22, 93, 255));
  line-height: 1.4;
}
</style>
