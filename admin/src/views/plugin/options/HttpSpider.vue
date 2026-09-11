<template>

  <a-tabs v-model:active-key="activeKey" @change="onTabChange" class="http-spider-tabs">
    <!-- ============================== 全局设置 ============================== -->
    <a-tab-pane key="global" title="全局设置">
      <a-form-item label="HTTP 代理">
        <a-input v-model="data.proxy" placeholder="如 http://127.0.0.1:7890，留空不使用" allow-clear />
        <template #extra>
          <a-typography-text type="secondary" class="text-xs">
            采集走代理时填写，支持 http/socks5，留空直连。
          </a-typography-text>
        </template>
      </a-form-item>
      <a-form-item label="请求超时">
        <a-input-number v-model="data.timeout" class="input" :min="5" :max="300" />
        <span class="text-sm text-gray-400 ml-3">秒</span>
        <template #extra>
          <a-typography-text type="secondary" class="text-xs">
            单个 HTTP 请求超时（默认 30）；慢站可调大到 60~90。
          </a-typography-text>
        </template>
      </a-form-item>
      <a-form-item label="详情并发">
        <a-input-number v-model="data.concurrency" class="input" :min="1" :max="32" />
        <span class="text-sm text-gray-400 ml-3">路</span>
        <template #extra>
          <a-typography-text type="secondary" class="text-xs">
            任务内详情页并发抓取数（默认 4，提速核心）；目标站有限频/封 IP 时调小到 1~2。
          </a-typography-text>
        </template>
      </a-form-item>
      <a-form-item label="入库间隔">
        <a-input-number v-model="data.interval" class="input" :min="0" :max="600" />
        <span class="text-sm text-gray-400 ml-3">秒</span>
        <template #extra>
          <a-typography-text type="secondary" class="text-xs">
            单个 worker 每篇入库后的等待秒数（默认 0）；注意并发下不构成全局节流，严格限频请调低「详情并发」。
          </a-typography-text>
        </template>
      </a-form-item>

      <a-divider orientation="left" :margin="8">调试与限额</a-divider>
      <div class="grid grid-cols-2 gap-x-4">
        <a-form-item label="失败重试">
          <a-input-number v-model="data.retry" class="input" :min="0" :max="10" />
          <template #extra>
            <a-typography-text type="secondary" class="text-xs">
              详情页采集失败后的重试次数（默认 0，间隔逐次递增 1s/2s/...）。
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
            列表页/详情页失败时保存响应 HTML，排查选择器必备。
          </a-typography-text>
        </template>
      </a-form-item>
    </a-tab-pane>

    <!-- ============================== 采集任务（结构化编辑，共用组件） ============================== -->
    <a-tab-pane key="tasks">
      <template #title>
        采集任务
        <a-tag v-if="tasks.length" color="arcoblue" class="ml-1">{{ enabledCount }}/{{ tasks.length }}</a-tag>
      </template>

      <SpiderTasksTab :tasks="tasks" :new-task="newTask" @insert="insertExample">
        <template #intro>
          每个任务独立配置选择器与入库方式；纯 HTTP 采集，适用于查看源代码（Ctrl+U）能看到完整内容的站点。
        </template>
        <template #basic-extra="{ task: t }">
          <a-form-item label="下一页选择器">
            <a-input v-model="t.next_page_sel" placeholder="如 a.next-page（仅支持 a[href]）" allow-clear />
            <template #extra>
              <a-typography-text type="secondary" class="text-xs">
                从页面解析「下一页」链接逐页 GET（不填 max_pages 时默认上限 50 页）；JS 按钮翻页（无 href）请用 HeadlessSpider。
              </a-typography-text>
            </template>
          </a-form-item>
        </template>
        <template #task-extra="{ task: t }">
          <!-- 请求伪装（HTTP 专属） -->
          <a-divider orientation="left" :margin="8">请求头与编码（反爬）</a-divider>
          <a-form-item label="User-Agent">
            <a-input v-model="t.user_agent" placeholder="留空用内置默认 Chrome UA" allow-clear />
          </a-form-item>
          <a-form-item label="自定义 Headers">
            <a-input v-model="t.headers" placeholder='JSON 对象，如 {"Referer":"https://a.tv/"}' allow-clear />
            <template #extra>
              <a-typography-text type="secondary" class="text-xs">
                JSON 对象格式；非法 JSON 会在任务装载时报错。
              </a-typography-text>
            </template>
          </a-form-item>
          <a-form-item label="Cookies">
            <a-input v-model="t.cookies" placeholder="原始 Cookie 头字符串，如 uid=1; token=abc" allow-clear />
            <template #extra>
              <a-typography-text type="secondary" class="text-xs">
                从浏览器 F12 → 网络请求 → 请求头 Cookie 复制整串。
              </a-typography-text>
            </template>
          </a-form-item>
          <a-form-item label="响应编码">
            <a-input v-model="t.encoding" placeholder="如 gbk；留空自动检测" allow-clear />
            <template #extra>
              <a-typography-text type="secondary" class="text-xs">
                强制解码（gbk/gb18030/big5 等）；留空自动检测：header 声明 → meta 声明 → 非 UTF-8 回退 GBK。
              </a-typography-text>
            </template>
          </a-form-item>
        </template>
        <template #store-extra="{ task: t }">
          <a-form-item label="去重键">
            <a-radio-group v-model="t.dedup_by" type="button" size="small">
              <a-radio value="">URL+标题</a-radio>
              <a-radio value="url">仅 URL</a-radio>
            </a-radio-group>
            <template #extra>
              <a-typography-text type="secondary" class="text-xs">
                默认 URL+标题 哈希；仅 URL 时站点标题微调不会重复入库。注意切换后既有文章会按新键被重新采集。
              </a-typography-text>
            </template>
          </a-form-item>
        </template>
      </SpiderTasksTab>
    </a-tab-pane>

    <!-- ============================== JSON 源码（共用组件） ============================== -->
    <a-tab-pane key="json" title="JSON 源码">
      <SpiderJsonTab :data="data" :summary="jsonSummary" @apply="applyJSON()" @format="formatJSON" />
    </a-tab-pane>
  </a-tabs>

</template>

<script setup>
import { inject } from "vue";
import { Message } from "@arco-design/web-vue";
import { useSpiderTasks } from "@/hooks/spiderTasks";
import SpiderTasksTab from "@/components/spider/SpiderTasksTab.vue";
import SpiderJsonTab from "@/components/spider/SpiderJsonTab.vue";

const data = inject("options");

const { activeKey, tasks, enabledCount, jsonSummary, onTabChange, applyJSON, formatJSON } = useSpiderTasks(data, { initialTab: "global" });

// 字段与后端 httpTask 对齐（含 HTTP 专属：next_page_sel/取链属性通道/正文翻页/请求伪装/dedup_by）
function newTask() {
  return {
    name: "新任务",
    enable: false,
    source_url: "",
    page_url_pattern: "",
    max_pages: 1,
    next_page_sel: "",
    mode: "detail",
    list_selector: "",
    link_include: "",
    link_exclude: "",
    link_selector: "",
    link_attr: "",
    link_extract_regex: "",
    link_url_template: "",
    list_title_sel: "",
    list_cover_sel: "",
    list_cover_attr: "",
    list_cover_extract_regex: "",
    list_desc_sel: "",
    stop_when_exists: 0,
    title_sel: "",
    title_attr: "",
    title_extract_regex: "",
    cover_sel: "",
    cover_attr: "",
    cover_extract_regex: "",
    content_sel: "",
    content_extract_regex: "",
    content_next_sel: "",
    content_max_pages: 20,
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
    video_label_sel: "",
    video_label_attr: "",
    gallery_sel: "",
    gallery_attr: "",
    gallery_extract_regex: "",
    extra: [],
    content_type: "",
    category_id: 0,
    user_agent: "",
    headers: "",
    cookies: "",
    encoding: "",
    dedup_by: "",
  };
}

function insertExample() {
  tasks.value.push({
    ...newTask(),
    name: "示例-视频站",
    source_url: "https://example.com/list/1.html",
    page_url_pattern: "https://example.com/list/1-{page}.html",
    max_pages: 3,
    list_selector: ".video-list a",
    link_include: "/detail-\\d+/",
    title_sel: "h1.title",
    // 封面在 <div class="vjs-poster" style="background-image:url(...)"> 的站：
    // 属性填 style，正则从属性值里抽图址
    cover_sel: ".vjs-poster",
    cover_attr: "style",
    cover_extract_regex: '/url\\(["\']?([^"\')]+)["\']?\\)/',
    content_sel: ".detail-content",
    video_src_sel: ".player video[src]",
    extra: [
      { key: "author", selector: ".author", attr: "", regex: "", multiple: false },
      // @url 伪选择器：从详情页 URL 抽视频 ID
      { key: "vid", selector: "@url", attr: "", regex: "/video/(\\d+)/", multiple: false },
    ],
    content_type: "video",
    category_id: 1,
    stop_when_exists: 3,
  });
  Message.success("已插入示例任务（默认停用，改好选择器后再启用）");
}
</script>

<style scoped>
.input {
  width: 100%;
}

.http-spider-tabs {
  max-height: 70vh;
  overflow-y: auto;
}

.http-spider-tabs :deep(.arco-tabs-content) {
  padding-top: 8px;
}
</style>
