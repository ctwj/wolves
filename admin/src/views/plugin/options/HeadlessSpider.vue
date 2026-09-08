<template>

  <a-tabs v-model:active-key="activeKey" @change="onTabChange" class="headless-spider-tabs">
    <!-- ============================== 浏览器全局配置 ============================== -->
    <a-tab-pane key="browser" title="浏览器">
      <a-form-item label="浏览器路径">
        <a-input v-model="data.browser_path" placeholder="留空自动查找本机 Chrome / Edge" allow-clear />
        <template #extra>
          <a-typography-text type="secondary" class="text-xs">
            无头浏览器可执行文件路径（Chrome 或 Edge）；留空时自动查找，本机未安装浏览器时必填。
          </a-typography-text>
        </template>
      </a-form-item>

      <a-form-item label="浏览器代理">
        <a-input v-model="data.proxy" placeholder="如 http://127.0.0.1:7890，留空不使用" allow-clear />
        <template #extra>
          <a-typography-text type="secondary" class="text-xs">
            采集走代理时填写，支持 http/socks5，留空直连。
          </a-typography-text>
        </template>
      </a-form-item>

      <a-form-item label="无头模式">
        <a-switch v-model="data.headless" />
        <template #extra>
          <a-typography-text type="secondary" class="text-xs">
            开启后浏览器后台运行；调试选择器时可关闭，直接观察页面加载过程。
          </a-typography-text>
        </template>
      </a-form-item>

      <a-form-item label="页面超时">
        <a-input-number v-model="data.timeout" class="input" :min="5" :max="300" />
        <span class="text-sm text-gray-400 ml-3">秒</span>
        <template #extra>
          <a-typography-text type="secondary" class="text-xs">
            整页总预算（默认 30）：导航 + 渲染等待 + 滚动 + 取链共享，任一阶段用完即超时；慢站/广告多的站建议 60~90。
          </a-typography-text>
        </template>
      </a-form-item>

      <a-form-item label="页面间隔">
        <a-input-number v-model="data.interval" class="input" :min="0" :max="600" />
        <span class="text-sm text-gray-400 ml-3">秒</span>
        <template #extra>
          <a-typography-text type="secondary" class="text-xs">
            每篇文章采集完成后的等待间隔，默认 3 秒，避免请求过快触发封禁。
          </a-typography-text>
        </template>
      </a-form-item>

      <a-divider orientation="left" :margin="8">调试与限额</a-divider>
      <div class="grid grid-cols-2 gap-x-4">
        <a-form-item label="失败重试">
          <a-input-number v-model="data.retry" class="input" :min="0" :max="10" />
          <template #extra>
            <a-typography-text type="secondary" class="text-xs">
              详情页采集失败后的重试次数，默认 0。
            </a-typography-text>
          </template>
        </a-form-item>
        <a-form-item label="采集限量">
          <a-input-number v-model="data.limit" class="input" :min="0" :max="100000" />
          <template #extra>
            <a-typography-text type="secondary" class="text-xs">
              每任务最多入库篇数，0=不限；调试时配 1~5 小样跑。
            </a-typography-text>
          </template>
        </a-form-item>
      </div>
      <a-form-item label="试运行">
        <a-switch v-model="data.dry_run" />
        <template #extra>
          <a-typography-text type="secondary" class="text-xs">
            只解析与打日志、不入库，调试选择器用。
          </a-typography-text>
        </template>
      </a-form-item>
      <a-form-item label="调试目录">
        <a-input v-model="data.debug_dir" placeholder="如 /tmp/spider_debug，留空关闭" allow-clear />
        <template #extra>
          <a-typography-text type="secondary" class="text-xs">
            列表页/详情页失败时保存页面截图（页面超时后也能拍到最后画面），排查选择器必备。
          </a-typography-text>
        </template>
      </a-form-item>
    </a-tab-pane>

    <!-- ============================== 采集任务（结构化编辑） ============================== -->
    <a-tab-pane key="tasks">
      <template #title>
        采集任务
        <a-tag v-if="tasks.length" color="arcoblue" class="ml-1">{{ enabledCount }}/{{ tasks.length }}</a-tag>
      </template>

      <div class="flex items-center justify-between mb-3">
        <a-typography-text type="secondary" class="text-xs">
          每个任务独立配置选择器与入库方式，按顺序执行；点击任务标题展开编辑。
        </a-typography-text>
        <a-space>
          <a-button size="small" @click="insertExample">
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
              <a-button type="text" size="mini" :disabled="i === 0" @click.stop="moveTask(i, -1)"><template #icon><icon-up :size="14" /></template></a-button>
              <a-button type="text" size="mini" :disabled="i === tasks.length - 1" @click.stop="moveTask(i, 1)"><template #icon><icon-down :size="14" /></template></a-button>
              <a-button type="text" size="mini" @click.stop="copyTask(i)"><template #icon><icon-copy :size="14" /></template></a-button>
              <a-popconfirm content="确定删除该任务？" type="warning" @ok="removeTask(i)">
                <a-button type="text" size="mini" status="danger"><template #icon><icon-delete :size="14" /></template></a-button>
              </a-popconfirm>
            </span>
          </template>

          <!-- 基本设置 -->
          <a-divider orientation="left" :margin="8">基本设置</a-divider>
          <a-form-item label="任务名">
            <a-input v-model="t.name" placeholder="用于日志区分，如 示例-视频站" allow-clear />
          </a-form-item>
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
          <a-form-item label="增量停止">
            <a-input-number v-model="t.stop_when_exists" class="input" :min="0" :max="100" />
            <span class="text-sm text-gray-400 ml-2">篇</span>
            <template #extra>
              <a-typography-text type="secondary" class="text-xs">
                连续 N 篇文章已存在时提前结束任务（0=不启用，总是翻满页数）。定时采集最新文章建议设 3~5：遇到旧文自动停，只采新增。
              </a-typography-text>
            </template>
          </a-form-item>

          <a-form-item label="采集模式">
            <a-radio-group v-model="t.mode" type="button" size="small">
              <a-radio value="detail">进详情页提取</a-radio>
              <a-radio value="list">列表页直接入库</a-radio>
            </a-radio-group>
            <template #extra>
              <a-typography-text type="secondary" class="text-xs">
                detail 打开每个链接提取标题/正文等字段；list 只用列表页信息快速入库。
              </a-typography-text>
            </template>
          </a-form-item>

          <!-- 列表页提取 -->
          <a-divider orientation="left" :margin="8">列表页提取</a-divider>
          <a-form-item label="渲染等待元素">
            <a-input v-model="t.wait_selector" placeholder="如 .video-list，留空等 DOM 稳定" allow-clear />
            <template #extra>
              <a-typography-text type="secondary" class="text-xs">
                JS 渲染站点填写：该元素出现才算页面就绪，同时作用于详情页。正文为 JS 延迟填充的站可填「正文选择器:not(:empty)」（如 article.read-content:not(:empty)），等到正文有内容才算就绪。
              </a-typography-text>
            </template>
          </a-form-item>
          <a-form-item label="详情链接选择器" required>
            <a-input v-model="t.list_selector" placeholder="如 .video-list a" allow-clear />
            <template #extra>
              <a-typography-text type="secondary" class="text-xs">列表页中指向详情页的 &lt;a&gt; 元素选择器。</a-typography-text>
            </template>
          </a-form-item>
          <a-form-item label="链接只保留">
            <a-input v-model="t.link_include" placeholder="子串或 /正则/，留空不过滤" allow-clear />
          </a-form-item>
          <a-form-item label="链接排除">
            <a-input v-model="t.link_exclude" placeholder="子串或 /正则/，留空不排除" allow-clear />
          </a-form-item>
          <a-form-item label="条目封面">
            <a-input v-model="t.list_cover_sel" placeholder="mode=list 条目封面选择器（list 模式入库用）" allow-clear />
            <div class="grid grid-cols-2 gap-2 mt-1">
              <a-input v-model="t.list_cover_attr" placeholder="属性：默认 src；可 data-src" size="small" allow-clear />
              <a-input v-model="t.list_cover_extract_regex" placeholder="正则：background-image 条目图" size="small" allow-clear />
            </div>
          </a-form-item>

          <!-- 详情页字段 -->
          <a-divider orientation="left" :margin="8">详情页字段（detail 模式）</a-divider>
          <a-typography-text type="secondary" class="text-xs block mb-2">
            统一提取模型：选择器定位 → 属性（留空=自动，meta 的 content 优先回退文本；封面是 src 优先）→ 正则抽取（可 /re/ 包裹，取第一捕获组，如 /url\(["']?([^"')]+)["']?\)/ 抽 background-image 图址）。只填选择器时与旧版行为一致。
          </a-typography-text>
          <a-form-item label="标题">
            <a-input v-model="t.title_sel" placeholder="选择器，如 h1.title；留空取 <title>" allow-clear />
            <div class="grid grid-cols-2 gap-2 mt-1">
              <a-input v-model="t.title_attr" placeholder="属性：空=自动；可 data-title" size="small" allow-clear />
              <a-input v-model="t.title_extract_regex" placeholder="正则：如 /(.+?)\s*[-|]\s*站名$/ 去后缀" size="small" allow-clear />
            </div>
          </a-form-item>
          <a-form-item label="封面">
            <a-input v-model="t.cover_sel" placeholder="选择器：img / meta[...] / .vjs-poster；留空回退 og:image" allow-clear />
            <div class="grid grid-cols-2 gap-2 mt-1">
              <a-input v-model="t.cover_attr" placeholder="属性：空=自动(src→content)；可 style" size="small" allow-clear />
              <a-input v-model="t.cover_extract_regex" placeholder="正则：抽 style 里 background-image 图址" size="small" allow-clear />
            </div>
            <template #extra>
              <a-typography-text type="secondary" class="text-xs">
                例：封面在 &lt;div class="vjs-poster" style="background-image:url(...)"&gt; 时，属性填 style、正则填 /url\(["']?([^"')]+)["']?\)/。
              </a-typography-text>
            </template>
          </a-form-item>
          <a-form-item label="正文">
            <a-input v-model="t.content_sel" placeholder="正文容器选择器，取 innerHTML" allow-clear />
            <a-input v-model="t.content_extract_regex" class="mt-1" placeholder="正则：从容器 HTML 抽片段（script 内嵌正文/播放数据的站）" size="small" allow-clear />
            <template #extra>
              <a-typography-text type="secondary" class="text-xs">
                正文容器选择器（取 innerHTML）。命中后日志会记录正文长度与前 150 字预览；容器先渲染、正文 JS 后填充的站点，把渲染等待元素配成「本选择器:not(:empty)」。
              </a-typography-text>
            </template>
          </a-form-item>
          <a-form-item label="关键词">
            <a-input v-model="t.keywords_sel" placeholder="选择器；留空回退 meta[name=keywords]" allow-clear />
            <div class="grid grid-cols-2 gap-2 mt-1">
              <a-input v-model="t.keywords_attr" placeholder="属性：空=自动" size="small" allow-clear />
              <a-input v-model="t.keywords_extract_regex" placeholder="正则抽取" size="small" allow-clear />
            </div>
          </a-form-item>
          <a-form-item label="发布时间">
            <a-input v-model="t.publish_time_sel" placeholder="选择器；留空回退 article:published_time" allow-clear />
            <div class="grid grid-cols-2 gap-2 mt-1">
              <a-input v-model="t.publish_time_attr" placeholder="属性：空=自动" size="small" allow-clear />
              <a-input v-model="t.publish_time_extract_regex" placeholder="正则：从值中抽日期时间" size="small" allow-clear />
            </div>
          </a-form-item>
          <a-form-item label="播放源(直链)">
            <a-input v-model="t.video_src_sel" placeholder="如 .player video；写入 extends[video_sources]" allow-clear />
            <div class="grid grid-cols-2 gap-2 mt-1">
              <a-input v-model="t.video_attr" placeholder="属性：默认 src；可 data-src" size="small" allow-clear />
              <a-input v-model="t.video_extract_regex" placeholder="正则：从属性值抽 m3u8 等" size="small" allow-clear />
            </div>
          </a-form-item>
          <a-form-item label="播放源(iframe)">
            <a-input v-model="t.video_iframe_sel" placeholder="第三方播放页 iframe 选择器（embed=true）" allow-clear />
            <div class="grid grid-cols-2 gap-2 mt-1">
              <a-input v-model="t.video_iframe_attr" placeholder="属性：默认 src；可 data-src" size="small" allow-clear />
              <a-input v-model="t.video_iframe_extract_regex" placeholder="正则抽取" size="small" allow-clear />
            </div>
          </a-form-item>
          <a-form-item label="图集">
            <a-input v-model="t.gallery_sel" placeholder="图集页图片选择器；写入 extends[gallery_images]" allow-clear />
            <div class="grid grid-cols-2 gap-2 mt-1">
              <a-input v-model="t.gallery_attr" placeholder="属性：默认 src；可 data-src" size="small" allow-clear />
              <a-input v-model="t.gallery_extract_regex" placeholder="正则：background-image 图集" size="small" allow-clear />
            </div>
          </a-form-item>

          <!-- 扩展字段 -->
          <a-divider orientation="left" :margin="8">扩展字段（extends）</a-divider>
          <div v-for="(ex, j) in (t.extra || [])" :key="j" class="flex items-center gap-2 mb-2 flex-wrap">
            <a-input v-model="ex.key" placeholder="键名 如 author" style="width: 120px" />
            <a-input v-model="ex.selector" placeholder="CSS 选择器 或 @url（从页面 URL 提取）" style="width: 230px" />
            <a-select v-model="ex.attr" style="width: 130px" placeholder="取值方式">
              <a-option value="">自动(文本兜底)</a-option>
              <a-option value="html">innerHTML</a-option>
              <a-option value="src">src 属性</a-option>
              <a-option value="href">href 属性</a-option>
              <a-option value="content">content 属性</a-option>
              <a-option value="style">style 属性</a-option>
            </a-select>
            <a-input v-model="ex.regex" placeholder="正则抽取，可 /re/ 取第一捕获组" class="flex-1" style="min-width: 150px" />
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

          <!-- 入库设置 -->
          <a-divider orientation="left" :margin="8">入库设置</a-divider>
          <a-form-item label="入库类型">
            <a-radio-group v-model="t.content_type" type="button" size="small">
              <a-radio value="">普通</a-radio>
              <a-radio value="novel">小说</a-radio>
              <a-radio value="image">图集</a-radio>
              <a-radio value="video">视频</a-radio>
            </a-radio-group>
            <template #extra>
              <a-typography-text type="secondary" class="text-xs">
                决定前台 wolves 主题用哪种模板渲染，普通文章选「普通」。
              </a-typography-text>
            </template>
          </a-form-item>
          <a-form-item label="入库分类">
            <SelectCategory v-model="t.category_id" />
            <template #extra>
              <a-typography-text type="secondary" class="text-xs">
                采集文章归入的栏目；右侧按钮可直接输入栏目 ID。
              </a-typography-text>
            </template>
          </a-form-item>
        </a-collapse-item>
      </a-collapse>
    </a-tab-pane>

    <!-- ============================== JSON 源码 ============================== -->
    <a-tab-pane key="json" title="JSON 源码">
      <a-alert class="mb-2" type="warning">
        在此编辑任务 JSON 后，请点击「解析并应用」同步到「采集任务」表单；切换页签时也会自动尝试应用。
      </a-alert>
      <a-textarea v-model="data.tasks" class="font-mono text-xs" placeholder="[]" :auto-size="{ minRows: 12, maxRows: 24 }" />
      <a-space class="mt-2">
        <a-button size="small" type="primary" @click="applyJSON">
          <template #icon><icon-check :size="14" /></template>
          解析并应用
        </a-button>
        <a-button size="small" @click="formatJSON">格式化 / 校验</a-button>
        <a-typography-text type="secondary" class="text-xs">{{ jsonSummary }}</a-typography-text>
      </a-space>
    </a-tab-pane>
  </a-tabs>

</template>

<script setup>
import { computed, inject, onMounted, ref, watch } from "vue";
import { Message } from "@arco-design/web-vue";
import SelectCategory from "@/components/data/SelectCategory.vue";

const data = inject("options");

if (!data.value.tasks) {
  data.value.tasks = "[]";
}

// 解析已有 tasks JSON；失败则停在 JSON 源码页让用户先修复
let initTasks = [];
let initError = "";
try {
  const parsed = JSON.parse(data.value.tasks || "[]");
  if (Array.isArray(parsed)) {
    initTasks = parsed;
  } else {
    initError = "tasks 必须是 JSON 数组";
  }
} catch (e) {
  initError = e.message;
}

const activeKey = ref(initError ? "json" : "browser");
const tasks = ref(initTasks);

// 表单编辑 → 写回插件配置（初始化赋值发生在 watch 建立之前，不会误触发）
watch(tasks, (v) => {
  data.value.tasks = JSON.stringify(v, null, 2);
}, { deep: true });

if (initError) {
  onMounted(() => Message.warning(`tasks JSON 解析失败（${initError}），请在本页修复后再使用表单`));
}

const enabledCount = computed(() => tasks.value.filter((t) => t.enable).length);

const jsonSummary = computed(() => {
  try {
    const parsed = JSON.parse(data.value.tasks || "[]");
    if (!Array.isArray(parsed)) return "tasks 必须是 JSON 数组";
    return `共 ${parsed.length} 个任务，${parsed.filter((t) => t.enable).length} 个启用`;
  } catch (e) {
    return "JSON 格式错误：" + e.message;
  }
});

// 切换页签：离开 JSON 源码页时自动解析应用，失败则拉回
function onTabChange(key) {
  if (key !== "json") {
    applyJSON(true);
  }
}

function parseJSON() {
  const parsed = JSON.parse(data.value.tasks || "[]");
  if (!Array.isArray(parsed)) {
    throw new Error("tasks 必须是 JSON 数组");
  }
  return parsed;
}

// JSON → 表单。silent 模式（切页触发）下 JSON 与表单一致时不打扰用户
function applyJSON(silent) {
  let parsed;
  try {
    parsed = parseJSON();
  } catch (e) {
    activeKey.value = "json";
    Message.error("tasks 解析失败：" + e.message);
    return;
  }
  const next = JSON.stringify(parsed, null, 2);
  if (silent && next === JSON.stringify(tasks.value, null, 2)) return;
  tasks.value = parsed;
  Message.success(`已应用到表单，共 ${parsed.length} 个任务`);
}

function formatJSON() {
  try {
    const parsed = parseJSON();
    data.value.tasks = JSON.stringify(parsed, null, 2);
    Message.success(`JSON 校验通过，共 ${parsed.length} 个任务`);
  } catch (e) {
    Message.error("tasks 解析失败：" + e.message);
  }
}

function newTask() {
  return {
    name: "新任务",
    enable: false,
    source_url: "",
    page_url_pattern: "",
    max_pages: 1,
    mode: "detail",
    wait_selector: "",
    list_selector: "",
    link_include: "",
    link_exclude: "",
    stop_when_exists: 0,
    list_cover_sel: "",
    list_cover_attr: "",
    list_cover_extract_regex: "",
    title_sel: "",
    title_attr: "",
    title_extract_regex: "",
    cover_sel: "",
    cover_attr: "",
    cover_extract_regex: "",
    content_sel: "",
    content_extract_regex: "",
    keywords_sel: "",
    keywords_attr: "",
    keywords_extract_regex: "",
    publish_time_sel: "",
    publish_time_attr: "",
    publish_time_extract_regex: "",
    video_src_sel: "",
    video_attr: "",
    video_extract_regex: "",
    video_iframe_sel: "",
    video_iframe_attr: "",
    video_iframe_extract_regex: "",
    gallery_sel: "",
    gallery_attr: "",
    gallery_extract_regex: "",
    extra: [],
    content_type: "",
    category_id: 0,
  };
}

function addTask() {
  tasks.value.push(newTask());
}

function addExtra(t) {
  if (!Array.isArray(t.extra)) t.extra = [];
  t.extra.push({ key: "", selector: "", attr: "", regex: "", multiple: false });
}

function insertExample() {
  tasks.value.push({
    ...newTask(),
    name: "示例-视频站",
    source_url: "https://example.com/list/1.html",
    page_url_pattern: "https://example.com/list/1-{page}.html",
    wait_selector: ".video-list",
    list_selector: ".video-list a",
    title_sel: "h1.title",
    // 封面在 <div class="vjs-poster" style="background-image:url(...)"> 的站：
    // 属性填 style，正则从属性值里抽图址
    cover_sel: ".vjs-poster",
    cover_attr: "style",
    cover_extract_regex: '/url\\(["\']?([^"\')]+)["\']?\\)/',
    content_sel: ".detail-content",
    video_src_sel: ".player video",
    extra: [
      { key: "author", selector: ".author", attr: "", regex: "", multiple: false },
      // @url 伪选择器：从详情页 URL 抽视频 ID
      { key: "vid", selector: "@url", attr: "", regex: "/video/(\\d+)/", multiple: false },
    ],
    content_type: "video",
    category_id: 1,
  });
  Message.success("已插入示例任务（默认停用，改好选择器后再启用）");
}

function copyTask(i) {
  const copy = JSON.parse(JSON.stringify(tasks.value[i]));
  copy.name = (copy.name || "任务") + " 副本";
  copy.enable = false;
  tasks.value.splice(i + 1, 0, copy);
}

function moveTask(i, offset) {
  const j = i + offset;
  if (j < 0 || j >= tasks.value.length) return;
  const [item] = tasks.value.splice(i, 1);
  tasks.value.splice(j, 0, item);
}

function removeTask(i) {
  tasks.value.splice(i, 1);
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
</script>

<style scoped>
.input {
  width: 100%;
}

.headless-spider-tabs {
  max-height: 62vh;
  overflow-y: auto;
}

.headless-spider-tabs :deep(.arco-tabs-content) {
  padding-top: 8px;
}
</style>
