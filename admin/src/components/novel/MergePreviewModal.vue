<template>
  <a-modal
    :visible="visible"
    :width="860"
    title-align="start"
    :mask-closable="false"
    unmount-on-close
    @update:visible="$emit('update:visible', $event)"
  >
    <template #title>
      <span class="inline-flex items-center gap-2 min-w-0">
        小说章节合并预览
        <a-tag v-if="result?.keyword" color="arcoblue" size="small" class="max-w-[300px] min-w-0" :title="result.keyword">
          <span class="truncate">{{ result.keyword }}</span>
        </a-tag>
        <a-tag v-if="mergeResult" color="green" size="small">合并完成</a-tag>
        <a-tag v-else color="green" size="small">只读 · 未改动数据</a-tag>
      </span>
    </template>

    <!-- 预览加载中 -->
    <div v-if="loading" class="py-8 text-center">
      <a-spin :size="28" tip="正在按标题关键词匹配章节文章…" />
    </div>

    <template v-else-if="mergeResult">
      <!-- 执行结果态 -->
      <a-result status="success" :title="`已合并为《${mergeResult.title}》（${mergeResult.chapter_count} 章）`">
        <template #subtitle>
          <div class="text-xs leading-6">
            新文章 ID：{{ mergeResult.new_id }} · slug：{{ mergeResult.slug }} · 已删除原章节 {{ mergeResult.deleted_count }} 篇 · 耗时 {{ mergeResult.cost_ms }}ms
          </div>
        </template>
        <template #extra>
          <a-button type="primary" :href="mergeResult.url" target="_blank" rel="noopener">打开合并文章</a-button>
        </template>
      </a-result>
      <a-alert v-if="mergeResult.failed_deletes?.length" type="warning" class="mt-2">
        以下 {{ mergeResult.failed_deletes.length }} 篇原章节删除失败（内容已包含在合并文章中，请手动删除避免重复）：
        <div v-for="d in mergeResult.failed_deletes" :key="d.id" class="text-xs mt-1">
          [ID:{{ d.id }}] {{ d.title }} —— {{ d.error }}
        </div>
      </a-alert>
    </template>

    <template v-else-if="result">
      <!-- 流程错误（禁止确认） -->
      <a-alert v-if="result.error" type="error" class="mb-3">{{ result.error }}</a-alert>

      <template v-else>
        <!-- 非致命提示 -->
        <a-alert v-if="result.warning" type="warning" class="mb-3">{{ result.warning }}</a-alert>
        <a-alert v-if="result.title_exists" type="error" class="mb-3">
          已存在标题为「{{ result.keyword }}」的文章，执行合并将被拒绝——可能是此前已合并过，请先手动处理同名文章。
        </a-alert>
        <a-alert v-else-if="result.duplicate_nos?.length" type="warning" class="mb-3">
          以下章节号出现多次：{{ result.duplicate_nos.join('、') }}（已按时间稳定排序，请核对顺序）
        </a-alert>

        <!-- 概要统计 -->
        <div class="grid grid-cols-5 gap-2 mb-3 text-center">
          <div class="rounded-lg border border-gray-100 py-2">
            <div class="text-lg font-semibold">{{ result.total }}</div>
            <div class="text-xs text-gray-400">标题命中</div>
          </div>
          <div class="rounded-lg border border-gray-100 py-2">
            <div class="text-lg font-semibold">{{ result.included }}</div>
            <div class="text-xs text-gray-400">可合并章节</div>
          </div>
          <div class="rounded-lg border border-gray-100 py-2">
            <div class="text-lg font-semibold" :class="result.unrecognized > 0 ? 'text-orange-500' : ''">{{ result.unrecognized }}</div>
            <div class="text-xs text-gray-400">未识别章节号</div>
          </div>
          <div class="rounded-lg border border-gray-100 py-2">
            <div class="text-lg font-semibold">{{ result.excluded }}</div>
            <div class="text-xs text-gray-400">已排除</div>
          </div>
          <div class="rounded-lg border border-gray-100 py-2">
            <div class="text-lg font-semibold">{{ result.cost_ms }}ms</div>
            <div class="text-xs text-gray-400">耗时</div>
          </div>
        </div>

        <!-- 合并目标 -->
        <section class="v-card">
          <header class="v-title">合并目标</header>
          <div class="text-xs flex flex-wrap gap-x-6 gap-y-1">
            <span>标题：<b>{{ result.target?.title }}</b></span>
            <span>slug：<span class="font-mono">{{ result.target?.slug }}</span>
              <span v-if="result.slug_taken" class="text-orange-500">（已占用，执行时自动加后缀）</span>
            </span>
            <span>分类ID：{{ result.target?.category_id || '—' }}</span>
            <span v-if="result.pattern_used !== 'builtin'" class="text-gray-400">
              自定义正则：<span class="font-mono">{{ result.pattern_used }}</span>
            </span>
          </div>
        </section>

        <!-- 章节列表 -->
        <section class="v-card">
          <header class="v-title flex items-center justify-between">
            <span>章节顺序（{{ checkedIds.length }}/{{ result.chapters?.length || 0 }} 勾选，按此顺序合并）</span>
            <a-checkbox
              :model-value="checkedIds.length === (result.chapters?.length || 0) && checkedIds.length > 0"
              @change="toggleAll"
            >全选</a-checkbox>
          </header>
          <div class="max-h-[380px] overflow-y-auto">
            <div
              v-for="ch in result.chapters"
              :key="ch.id"
              class="flex items-center gap-2 py-1 border-b border-gray-50 last:border-0"
            >
              <a-checkbox :model-value="checkedIds.includes(ch.id)" @change="toggleOne(ch.id)" />
              <span class="text-gray-400 shrink-0" style="width: 34px">#{{ ch.sort_index }}</span>
              <a-tag v-if="ch.recognized" color="arcoblue" size="small" class="shrink-0">第 {{ ch.chapter_no }} 章</a-tag>
              <a-tag v-else color="orange" size="small" class="shrink-0">未识别</a-tag>
              <a-tooltip :content="`${ch.title}（命中规则：${ch.matched_by || '—'}）`">
                <span class="truncate min-w-0 flex-1">{{ ch.title }}</span>
              </a-tooltip>
              <span v-if="!ch.status" class="shrink-0"><a-tag size="small">未发布</a-tag></span>
              <span class="text-gray-400 shrink-0 text-right" style="width: 150px; font-size: 11px">{{ fmtTime(ch.create_time) }}</span>
              <span class="text-gray-300 shrink-0 text-right" style="width: 50px; font-size: 11px">ID:{{ ch.id }}</span>
            </div>
          </div>
        </section>

        <!-- 排除项 -->
        <section v-if="result.excluded_list?.length" class="v-card">
          <a-collapse :default-active-key="[]" expand-icon-position="left" :bordered="false">
            <a-collapse-item key="excluded" :header="`已排除文章（${result.excluded_list.length} 篇，不参与合并）`">
              <div v-for="e in result.excluded_list" :key="e.id" class="text-xs flex gap-2 items-baseline py-0.5">
                <span class="text-gray-400 shrink-0">[ID:{{ e.id }}]</span>
                <span class="truncate min-w-0 flex-1" :title="e.title">{{ e.title }}</span>
                <span class="text-gray-400 shrink-0">{{ e.reason }}</span>
              </div>
            </a-collapse-item>
          </a-collapse>
        </section>
      </template>
    </template>

    <template #footer>
      <template v-if="mergeResult">
        <a-button type="primary" @click="$emit('update:visible', false)">关闭</a-button>
      </template>
      <template v-else>
        <div class="text-left text-xs text-red-500 flex-1 mr-3 leading-5">
          确认后将创建合并文章，并<b>硬删除 {{ checkedIds.length }} 篇原章节文章（不可恢复）</b>
        </div>
        <a-button @click="$emit('update:visible', false)">取消</a-button>
        <a-button
          type="primary"
          status="danger"
          :loading="merging"
          :disabled="!!result?.error || !!result?.title_exists || checkedIds.length === 0"
          @click="onConfirm"
        >确认合并（{{ checkedIds.length }} 章）</a-button>
      </template>
    </template>
  </a-modal>
</template>

<script setup>
import { ref, watch } from "vue";

// NovelMerger 的「预览→确认」弹窗：
// 预览态只读展示匹配章节/识别结果/排除项，勾选后 emit confirm(ids) 由父组件执行合并；
// mergeResult 非空时切换为执行结果态。
const props = defineProps({
  visible: { type: Boolean, default: false },
  loading: { type: Boolean, default: false },       // 预览请求中
  result: { type: Object, default: null },          // NovelMergePreviewResult
  merging: { type: Boolean, default: false },       // 合并请求中
  mergeResult: { type: Object, default: null },     // NovelMergeResult，非空=已完成
});

const emit = defineEmits(["update:visible", "confirm"]);

// 勾选状态：预览结果变化/弹窗打开时重置为全选
const checkedIds = ref([]);
watch(
  () => [props.visible, props.result],
  () => {
    if (props.visible && props.result) {
      checkedIds.value = (props.result.chapters || []).map((c) => c.id);
    }
  },
  { immediate: true }
);

function toggleAll() {
  const all = props.result?.chapters || [];
  checkedIds.value = checkedIds.value.length === all.length ? [] : all.map((c) => c.id);
}

function toggleOne(id) {
  const i = checkedIds.value.indexOf(id);
  if (i >= 0) checkedIds.value.splice(i, 1);
  else checkedIds.value.push(id);
}

// 按预览排序（而非勾选先后）传回 ids，保证所见即所得
function onConfirm() {
  const order = (props.result?.chapters || []).map((c) => c.id);
  emit("confirm", checkedIds.value.filter((id) => order.includes(id)).sort((a, b) => order.indexOf(a) - order.indexOf(b)));
}

function fmtTime(unix) {
  if (!unix) return "—";
  const d = new Date(unix * 1000);
  const p = (n) => String(n).padStart(2, "0");
  return `${d.getFullYear()}-${p(d.getMonth() + 1)}-${p(d.getDate())} ${p(d.getHours())}:${p(d.getMinutes())}`;
}
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
