<template>
  <div ref="secRoot">
    <div class="flex items-center justify-between mb-3">
      <a-typography-text type="secondary" class="text-xs">
        <slot name="intro">每个任务独立配置选择器与入库方式，按顺序执行；点击任务标题展开编辑。</slot>
      </a-typography-text>
      <a-space>
        <a-button size="small" @click="$emit('insert')">
          <template #icon><icon-code :size="16" /></template>
          插入示例
        </a-button>
        <a-button size="small" type="primary" @click="addTask">
          <template #icon><icon-plus :size="16" /></template>
          新增任务
        </a-button>
      </a-space>
    </div>

    <a-empty v-if="!tasks.length" description="暂无任务，点击「新增任务」创建，或到「JSON 源码」页粘贴已有配置" />

    <a-collapse v-else accordion expand-icon-position="left">
      <a-collapse-item v-for="(t, i) in tasks" :key="i" hide-follow-theme>
        <template #title>
          <span class="inline-flex items-center gap-2 min-w-0">
            <span class="text-gray-400 text-xs">#{{ i + 1 }}</span>
            <span class="truncate">{{ t.name || '未命名任务' }}</span>
            <a-tag v-if="t.content_type" :color="contentTypeColor(t.content_type)">{{ contentTypeLabel(t.content_type) }}</a-tag>
            <a-typography-text type="secondary" class="text-xs truncate">{{ hostOf(t.source_url) }}</a-typography-text>
          </span>
        </template>
        <template #extra>
          <span class="inline-flex items-center gap-2" @click.stop>
            <a-switch v-model="t.enable" size="small" type="round">
              <template #checked>启用</template>
              <template #unchecked>停用</template>
            </a-switch>
            <a-button type="outline" size="mini" status="success" :loading="validating === i" @click.stop="runValidate(i)">验证</a-button>
            <a-button type="text" size="mini" :disabled="i === 0" @click.stop="moveTask(i, -1)"><template #icon><icon-up :size="14" /></template></a-button>
            <a-button type="text" size="mini" :disabled="i === tasks.length - 1" @click.stop="moveTask(i, 1)"><template #icon><icon-down :size="14" /></template></a-button>
            <a-button type="text" size="mini" @click.stop="copyTask(i)"><template #icon><icon-copy :size="14" /></template></a-button>
            <a-popconfirm content="确定删除该任务？" type="warning" @ok="removeTask(i)">
              <a-button type="text" size="mini" status="danger"><template #icon><icon-delete :size="14" /></template></a-button>
            </a-popconfirm>
          </span>
        </template>

        <!-- 分区锚点导航：长表单分区直达，滚动联动高亮 -->
        <nav class="section-nav" aria-label="配置分区导航">
          <button
            v-for="s in SECTIONS"
            :key="s.key"
            type="button"
            class="section-nav-item"
            :class="{ active: activeSec === s.key }"
            @click="jumpSection(s.key)"
          >
            {{ s.label }}
          </button>
        </nav>

        <!-- 基本设置 -->
        <section data-sec="basic" class="section-card">
          <header class="section-title">基本设置</header>
          <div class="grid grid-cols-2 gap-x-4">
            <a-form-item label="任务名">
              <a-input v-model="t.name" placeholder="用于日志区分，如 示例-视频站" allow-clear />
            </a-form-item>
            <a-form-item label="采集模式">
              <a-radio-group v-model="t.mode" type="button" size="small">
                <a-radio value="detail">进详情页提取</a-radio>
                <a-radio value="list">列表页直接入库</a-radio>
              </a-radio-group>
            </a-form-item>
          </div>
          <a-form-item label="起始页 URL" required>
            <a-input v-model="t.source_url" placeholder="https://example.com/list/1.html" allow-clear />
            <template #extra>
              <a-typography-text type="secondary" class="text-xs">列表页第一页地址。</a-typography-text>
            </template>
          </a-form-item>
          <div class="grid grid-cols-2 gap-x-4">
            <a-form-item label="翻页模板">
              <a-input v-model="t.page_url_pattern" placeholder="含 {page}，留空只采起始页" allow-clear />
            </a-form-item>
            <a-form-item label="最多翻页">
              <a-input-number v-model="t.max_pages" class="input" :min="1" :max="999" />
            </a-form-item>
          </div>
          <!-- 插件专属翻页方式（HttpSpider：a[href] 下一页；HeadlessSpider：下一页/滚动加载） -->
          <slot name="basic-extra" :task="t" />
          <a-form-item label="增量停止">
            <a-input-number v-model="t.stop_when_exists" class="input" :min="0" :max="100" />
            <span class="text-sm text-gray-400 ml-2">篇</span>
            <template #extra>
              <a-typography-text type="secondary" class="text-xs">
                连续 N 篇文章已存在时提前结束任务（0=不启用）。定时采集最新文章建议设 3~5：遇到旧文自动停，只采新增。
              </a-typography-text>
            </template>
          </a-form-item>
        </section>

        <!-- 列表页取链 -->
        <section data-sec="list" class="section-card">
          <header class="section-title">列表页取链</header>
          <!-- 插件专属取链配置（HeadlessSpider：渲染等待/模拟点击；HttpSpider：无） -->
          <slot name="list-front" :task="t" />
          <a-form-item label="详情链接选择器" required>
            <a-input v-model="t.list_selector" placeholder="如 .video-list a 或 .item" allow-clear />
            <template #extra>
              <a-typography-text type="secondary" class="text-xs">
                列表页条目元素；默认从条目/其内 &lt;a&gt; 的 href 取详情链接，配合下方属性通道可从 data-id 等拼地址。
              </a-typography-text>
            </template>
          </a-form-item>
          <a-form-item label="取链属性通道">
            <a-input v-model="t.link_selector" placeholder="链接元素选择器：空=条目自身/其内 a" allow-clear />
            <AdvAttrRegexInput v-model:attr="t.link_attr" v-model:regex="t.link_extract_regex" class="mt-1" placeholder="高级：@属性 /正则/，如 @data-id；留空=href" />
            <a-input v-model="t.link_url_template" class="mt-1" placeholder="URL 模板：含 {value} 替换，否则前缀拼接，如 https://a.tv/detail/" size="small" allow-clear />
            <template #extra>
              <a-typography-text type="secondary" class="text-xs">
                适配 data-id 拼详情地址、onclick 内嵌地址的站点。
              </a-typography-text>
            </template>
          </a-form-item>
          <div class="grid grid-cols-2 gap-x-4">
            <a-form-item label="链接只保留">
              <a-input v-model="t.link_include" placeholder="子串或 /正则/，留空不过滤" allow-clear />
            </a-form-item>
            <a-form-item label="链接排除">
              <a-input v-model="t.link_exclude" placeholder="子串或 /正则/，留空不排除" allow-clear />
            </a-form-item>
          </div>
        </section>

        <!-- 列表条目字段（list 模式 / 预查重） -->
        <section data-sec="entry" class="section-card">
          <header class="section-title">
            列表条目字段
            <a-typography-text type="secondary" class="text-xs font-normal">
              {{ t.mode === 'list' ? '（list 模式入库数据源）' : '（detail 模式仅用于预查重）' }}
            </a-typography-text>
          </header>
          <div class="grid grid-cols-2 gap-x-4">
            <a-form-item label="条目标题">
              <a-input v-model="t.list_title_sel" placeholder="空=回退 h1~h6/首行文本" allow-clear />
            </a-form-item>
            <a-form-item label="条目摘要">
              <a-input v-model="t.list_desc_sel" placeholder="空=回退条目文本" allow-clear />
            </a-form-item>
          </div>
          <a-form-item label="条目封面">
            <a-input v-model="t.list_cover_sel" placeholder="条目封面选择器" allow-clear />
            <AdvAttrRegexInput v-model:attr="t.list_cover_attr" v-model:regex="t.list_cover_extract_regex" class="mt-1" placeholder="高级：@属性 /正则/，如 @data-src；留空=src" />
          </a-form-item>
        </section>

        <!-- 详情页字段 -->
        <section data-sec="detail" class="section-card">
          <header class="section-title">
            详情页字段
            <a-typography-text v-if="t.mode !== 'detail'" type="secondary" class="text-xs font-normal">
              （当前 list 模式不打开详情页，以下配置不生效）
            </a-typography-text>
          </header>
          <a-typography-text type="secondary" class="text-xs block mb-2">
            统一提取模型：选择器定位元素，高级框一行写 @属性 与 /正则/（空格分隔可同写、可省略）。属性留空=自动（meta 的 content 优先回退文本、封面 src 优先）；正则取第一捕获组。
          </a-typography-text>
          <div class="grid grid-cols-2 gap-x-4">
            <a-form-item label="标题">
              <a-input v-model="t.title_sel" placeholder="选择器，如 h1.title；留空取 <title>" allow-clear />
              <AdvAttrRegexInput v-model:attr="t.title_attr" v-model:regex="t.title_extract_regex" class="mt-1" placeholder="高级：@属性 /正则/，如 @data-title /(.+?)\s*[-|]\s*站名$/" />
            </a-form-item>
            <a-form-item label="关键词">
              <a-input v-model="t.keywords_sel" placeholder="选择器；留空回退 meta[name=keywords]" allow-clear />
              <AdvAttrRegexInput v-model:attr="t.keywords_attr" v-model:regex="t.keywords_extract_regex" class="mt-1" />
            </a-form-item>
          </div>
          <a-form-item label="封面">
            <a-input v-model="t.cover_sel" placeholder="选择器：img / meta[...] / .vjs-poster；留空回退 og:image" allow-clear />
            <AdvAttrRegexInput v-model:attr="t.cover_attr" v-model:regex="t.cover_extract_regex" class="mt-1" placeholder="高级：@属性 /正则/，封面在 style 里时填 @style /url\(([^)]+)\)/" />
            <template #extra>
              <a-typography-text type="secondary" class="text-xs">
                例：封面在 &lt;div class="vjs-poster" style="background-image:url(...)"&gt; 时，高级框填 @style /url\(["']?([^"')]+)["']?\)/。
              </a-typography-text>
            </template>
          </a-form-item>
          <a-form-item label="正文">
            <a-input v-model="t.content_sel" placeholder="正文容器选择器，取 innerHTML" allow-clear />
            <AdvAttrRegexInput v-model:regex="t.content_extract_regex" class="mt-1" placeholder="高级：/正则/ 从容器 HTML 抽片段（script 内嵌正文/播放数据的站）" />
          </a-form-item>
          <div class="grid grid-cols-2 gap-x-4">
            <a-form-item label="正文翻页">
              <a-input v-model="t.content_next_sel" placeholder="「下一页」按钮/链接选择器" allow-clear />
            </a-form-item>
            <a-form-item label="翻页上限">
              <a-input-number v-model="t.content_max_pages" class="input" :min="1" :max="99" />
            </a-form-item>
          </div>
          <a-form-item>
            <template #extra>
              <a-typography-text type="secondary" class="text-xs">
                正文分页时逐页拼接（无新增即停；上限默认 20）。
              </a-typography-text>
            </template>
          </a-form-item>
          <a-form-item label="发布时间">
            <a-input v-model="t.publish_time_sel" placeholder="选择器；留空回退 article:published_time" allow-clear />
            <AdvAttrRegexInput v-model:attr="t.publish_time_attr" v-model:regex="t.publish_time_extract_regex" class="mt-1" placeholder="高级：@属性 /正则/，如 /(\d{4}-\d{2}-\d{2}.*)/" />
          </a-form-item>

          <!-- 渐进披露：图集仅图集类任务需要；已配置过则保持可见 -->
          <a-form-item v-if="t.content_type === 'image' || t.gallery_sel" label="图集">
            <a-input v-model="t.gallery_sel" placeholder="图集页图片选择器；写入 extends[gallery_images]" allow-clear />
            <AdvAttrRegexInput v-model:attr="t.gallery_attr" v-model:regex="t.gallery_extract_regex" class="mt-1" placeholder="高级：@属性 /正则/，如 @data-src；留空=src" />
          </a-form-item>

          <!-- 渐进披露：播放源/集名仅视频类任务需要；已配置过则保持可见（防止旧配置被误隐藏） -->
          <template v-if="t.content_type === 'video' || hasVideoConfig(t)">
            <div class="grid grid-cols-2 gap-x-4">
              <a-form-item label="播放源(直链)">
                <a-input v-model="t.video_src_sel" placeholder="如 .player video；写入 extends[video_sources]" allow-clear />
                <AdvAttrRegexInput v-model:attr="t.video_attr" v-model:regex="t.video_extract_regex" class="mt-1" placeholder="高级：@属性 /正则/，如 @data-src；留空=src" />
              </a-form-item>
              <a-form-item label="播放源(iframe)">
                <a-input v-model="t.video_iframe_sel" placeholder="第三方播放页 iframe 选择器（embed=true）" allow-clear />
                <AdvAttrRegexInput v-model:attr="t.video_iframe_attr" v-model:regex="t.video_iframe_extract_regex" class="mt-1" />
              </a-form-item>
            </div>
            <a-form-item label="集名">
              <a-input v-model="t.video_label_sel" placeholder="集名元素选择器，与播放源同数量同顺序" allow-clear />
              <AdvAttrRegexInput v-model:attr="t.video_label_attr" class="mt-1" placeholder="高级：@属性 读哪个属性；留空=文本" />
              <template #extra>
                <a-typography-text type="secondary" class="text-xs">
                  缺省回退「第01集/第02集...」。
                </a-typography-text>
              </template>
            </a-form-item>
          </template>
          <a-typography-text v-else type="secondary" class="text-xs block mb-2">
            视频字段已按「入库类型=视频」收起，<a-link @click="t.content_type = 'video'">切换为视频任务</a-link>后显示。
          </a-typography-text>

          <!-- 插件专属任务字段（HttpSpider：请求伪装；HeadlessSpider：无） -->
          <slot name="task-extra" :task="t" />
        </section>

        <!-- 扩展字段 -->
        <section data-sec="extra" class="section-card">
          <header class="section-title">扩展字段（extends）</header>
          <div v-for="(ex, j) in (t.extra || [])" :key="j" class="flex items-center gap-2 mb-2 flex-wrap">
            <a-input v-model="ex.key" aria-label="扩展字段键名" placeholder="键名 如 author" style="width: 120px" />
            <a-input v-model="ex.selector" aria-label="扩展字段选择器" placeholder="CSS 选择器 或 @url（从页面 URL 提取）" style="width: 230px" />
            <a-select v-model="ex.attr" aria-label="扩展字段取值方式" style="width: 130px" placeholder="取值方式">
              <a-option value="">自动(文本兜底)</a-option>
              <a-option value="html">innerHTML</a-option>
              <a-option value="src">src 属性</a-option>
              <a-option value="href">href 属性</a-option>
              <a-option value="content">content 属性</a-option>
              <a-option value="style">style 属性</a-option>
            </a-select>
            <a-input v-model="ex.regex" aria-label="扩展字段正则" placeholder="正则抽取，可 /re/ 取第一捕获组" class="flex-1" style="min-width: 150px" />
            <a-tooltip content="多值：聚合所有匹配元素为数组">
              <a-switch v-model="ex.multiple" size="small" />
            </a-tooltip>
            <a-button type="text" size="mini" status="danger" @click="t.extra.splice(j, 1)">
              <template #icon><icon-delete :size="14" /></template>
            </a-button>
          </div>
          <a-button size="small" type="outline" @click="addExtra(t)">
            <template #icon><icon-plus :size="14" /></template>
            添加字段
          </a-button>
        </section>

        <!-- 入库设置 -->
        <section data-sec="store" class="section-card">
          <header class="section-title">入库设置</header>
          <div class="grid grid-cols-2 gap-x-4">
            <a-form-item label="入库类型">
              <a-radio-group v-model="t.content_type" type="button" size="small">
                <a-radio value="">普通</a-radio>
                <a-radio value="novel">小说</a-radio>
                <a-radio value="image">图集</a-radio>
                <a-radio value="video">视频</a-radio>
              </a-radio-group>
            </a-form-item>
            <a-form-item label="入库分类">
              <SelectCategory v-model="t.category_id" />
              <template #extra>
                <a-typography-text type="secondary" class="text-xs">
                  采集文章归入的栏目。
                </a-typography-text>
              </template>
            </a-form-item>
          </div>
          <!-- 插件专属入库设置（HttpSpider：去重键） -->
          <slot name="store-extra" :task="t" />
        </section>
      </a-collapse-item>
    </a-collapse>

    <!-- 任务校验结果（不入库，仅提取展示） -->
    <SpiderValidateModal v-model:visible="validateVisible" :loading="validating >= 0 && !validateResult" :result="validateResult" />
  </div>
</template>

<script setup>
import { inject, onBeforeUnmount, onMounted, ref } from "vue";
import { Message } from "@arco-design/web-vue";
import { pluginSpiderValidate } from "@/api/index.js";
import SelectCategory from "@/components/data/SelectCategory.vue";
import AdvAttrRegexInput from "@/components/utils/AdvAttrRegexInput.vue";
import SpiderValidateModal from "@/components/spider/SpiderValidateModal.vue";

// HttpSpider / HeadlessSpider 共用的「采集任务」结构化编辑器。
// tasks 由页面通过 useSpiderTasks 提供并深度写回插件配置；newTask 为各插件的任务字段工厂。
const props = defineProps({
  tasks: { type: Array, required: true },
  newTask: { type: Function, required: true },
});

defineEmits(["insert"]);

function addTask() {
  props.tasks.push(props.newTask());
}

function addExtra(t) {
  if (!Array.isArray(t.extra)) t.extra = [];
  t.extra.push({ key: "", selector: "", attr: "", regex: "", multiple: false });
}

function copyTask(i) {
  const copy = JSON.parse(JSON.stringify(props.tasks[i]));
  copy.name = (copy.name || "任务") + " 副本";
  copy.enable = false;
  props.tasks.splice(i + 1, 0, copy);
}

function moveTask(i, offset) {
  const j = i + offset;
  if (j < 0 || j >= props.tasks.length) return;
  const [item] = props.tasks.splice(i, 1);
  props.tasks.splice(j, 0, item);
}

function removeTask(i) {
  props.tasks.splice(i, 1);
}

function hostOf(url) {
  try {
    return new URL(url).host;
  } catch {
    return url || "";
  }
}

const contentTypeLabels = { novel: "小说", image: "图集", video: "视频" };
const contentTypeColors = { novel: "orange", image: "green", video: "purple" };
function contentTypeLabel(v) { return contentTypeLabels[v] || v; }
function contentTypeColor(v) { return contentTypeColors[v] || "gray"; }

// 渐进披露兜底：非视频类型但已配置过播放源字段时保持可见，避免旧配置被隐藏后误判丢失
function hasVideoConfig(t) {
  return !!(t.video_src_sel || t.video_iframe_sel || t.video_label_sel);
}

// ==================== 任务校验（不入库，仅提取展示） ====================
const currentID = inject("currentID", ref(""));
const validateVisible = ref(false);
const validating = ref(-1);
const validateResult = ref(null);

async function runValidate(i) {
  const pluginID = currentID.value;
  if (!pluginID) {
    Message.error("无法确定插件 ID，请重新打开配置窗口");
    return;
  }
  validating.value = i;
  validateVisible.value = true;
  validateResult.value = null;
  try {
    const resp = await pluginSpiderValidate(pluginID, props.tasks[i]);
    validateResult.value = resp?.data ?? resp;
  } catch {
    validateVisible.value = false; // 错误提示由 axios 拦截器统一弹出
  } finally {
    validating.value = -1;
  }
}

// ==================== 分区锚点导航（滚动联动高亮） ====================
const SECTIONS = [
  { key: "basic", label: "基本" },
  { key: "list", label: "取链" },
  { key: "entry", label: "条目" },
  { key: "detail", label: "字段" },
  { key: "extra", label: "扩展" },
  { key: "store", label: "入库" },
];
const secRoot = ref(null);
const activeSec = ref("basic");
let scroller = null;

function findScroller() {
  // 滚动容器是页面的 *-spider-tabs（max-height + overflow-y）
  return secRoot.value ? secRoot.value.closest("[class*='spider-tabs']") : null;
}

function jumpSection(key) {
  activeSec.value = key;
  if (!secRoot.value) return;
  const target = secRoot.value.querySelector(`[data-sec="${key}"]`);
  if (!target || !scroller) return;
  const delta = target.getBoundingClientRect().top - scroller.getBoundingClientRect().top;
  scroller.scrollTop += delta - 8;
}

function onScroll() {
  if (!secRoot.value || !scroller) return;
  const top = scroller.getBoundingClientRect().top;
  let current = "basic";
  secRoot.value.querySelectorAll("[data-sec]").forEach((el) => {
    if (el.getBoundingClientRect().top - top <= 40) current = el.dataset.sec;
  });
  activeSec.value = current;
}

onMounted(() => {
  scroller = findScroller();
  if (scroller) scroller.addEventListener("scroll", onScroll, { passive: true });
});

onBeforeUnmount(() => {
  if (scroller) scroller.removeEventListener("scroll", onScroll);
});
</script>

<style scoped>
.input {
  width: 100%;
}

/* Swiss 卡片分区：白底细边框 + 紧凑内边距（dashboard 密度） */
.section-card {
  border: 1px solid var(--color-border-2, #e5e6eb);
  border-radius: 8px;
  padding: 12px 12px 4px;
  margin-bottom: 12px;
  background: var(--color-fill-1, #f7f8fa);
}

.section-title {
  font-size: 12px;
  font-weight: 600;
  letter-spacing: 0.05em;
  color: var(--color-text-2, #4e5969);
  margin-bottom: 8px;
  padding-left: 8px;
  border-left: 3px solid rgb(var(--primary-6, 22, 93, 255));
  line-height: 1.4;
}

/* 粘性锚点导航 */
.section-nav {
  position: sticky;
  top: 0;
  z-index: 4;
  display: flex;
  gap: 4px;
  flex-wrap: wrap;
  padding: 6px;
  margin-bottom: 12px;
  border-radius: 8px;
  background: var(--color-bg-2, #fff);
  border: 1px solid var(--color-border-2, #e5e6eb);
  box-shadow: 0 1px 4px rgba(0, 0, 0, 0.04);
}

.section-nav-item {
  border: none;
  background: transparent;
  border-radius: 9999px;
  padding: 3px 12px;
  font-size: 12px;
  line-height: 1.5;
  color: var(--color-text-2, #4e5969);
  cursor: pointer;
  transition: background-color 0.2s, color 0.2s;
}

.section-nav-item:hover {
  background: var(--color-fill-2, #f2f3f5);
}

.section-nav-item.active {
  background: rgb(var(--primary-6, 22, 93, 255));
  color: #fff;
}

/* 开发者工具感：任务编辑器内的输入值（选择器/正则/URL）用等宽字体 */
:deep(.arco-input-wrapper input),
:deep(.arco-textarea) {
  font-family: ui-monospace, SFMono-Regular, "JetBrains Mono", Menlo, Consolas, "Liberation Mono", monospace;
}
</style>
