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

    <!-- ============================== 采集任务（结构化编辑，共用组件） ============================== -->
    <a-tab-pane key="tasks">
      <template #title>
        采集任务
        <a-tag v-if="tasks.length" color="arcoblue" class="ml-1">{{ enabledCount }}/{{ tasks.length }}</a-tag>
      </template>

      <SpiderTasksTab :tasks="tasks" :new-task="newTask" @insert="insertExample">
        <template #intro>
          每个任务独立配置选择器与入库方式；无头浏览器采集，适用于 JS 渲染、需要滚动/点击才能加载内容的站点。
        </template>
        <template #basic-extra="{ task: t }">
          <div class="grid grid-cols-2 gap-x-4">
            <a-form-item label="下一页选择器">
              <a-input v-model="t.next_page_sel" placeholder="「下一页」按钮/链接；SPA 站用" allow-clear />
            </a-form-item>
            <a-form-item label="滚动加载">
              <a-input-number v-model="t.scroll_times" class="input" :min="0" :max="50" />
              <span class="text-sm text-gray-400 ml-2">次</span>
            </a-form-item>
          </div>
          <a-form-item>
            <template #extra>
              <a-typography-text type="secondary" class="text-xs">
                翻页三选一：URL 模板（默认）/「下一页」选择器（a[href] 直接导航、无 href 则点击）/ 列表页加载后滚动到底 N 次（无限滚动、懒加载列表）。
              </a-typography-text>
            </template>
          </a-form-item>
        </template>
        <template #list-front="{ task: t }">
          <a-form-item label="渲染等待元素">
            <a-input v-model="t.wait_selector" placeholder="如 .video-list，留空等 DOM 稳定" allow-clear />
            <template #extra>
              <a-typography-text type="secondary" class="text-xs">
                JS 渲染站点填写：该元素出现才算页面就绪，同时作用于详情页。正文为 JS 延迟填充的站可填「正文选择器:not(:empty)」（如 article.read-content:not(:empty)），等到正文有内容才算就绪。
              </a-typography-text>
            </template>
          </a-form-item>
          <a-form-item label="取链方式">
            <a-radio-group v-model="t.link_mode" type="button" size="small">
              <a-radio value="">属性提取</a-radio>
              <a-radio value="click">模拟点击</a-radio>
            </a-radio-group>
            <template #extra>
              <a-typography-text type="secondary" class="text-xs">
                属性提取（默认）读 href 等属性；模拟点击适配 href="javascript:void(0)" 的站：劫持 window.open 逐条目点击捕获跳转地址，同时叠加属性提取兜底。
              </a-typography-text>
            </template>
          </a-form-item>
          <div v-if="t.link_mode === 'click'" class="grid grid-cols-2 gap-x-4">
            <a-form-item label="允许跨域跳转">
              <a-switch v-model="t.link_cross_origin" />
              <template #extra>
                <a-typography-text type="secondary" class="text-xs">点击默认只收同源跳转（自动丢弃广告弹窗）；详情确在另一域名时才开。</a-typography-text>
              </template>
            </a-form-item>
            <a-form-item label="点击等待">
              <a-input-number v-model="t.click_wait_ms" class="input" :min="50" :max="5000" />
              <span class="text-sm text-gray-400 ml-2">毫秒</span>
            </a-form-item>
          </div>
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

const { activeKey, tasks, enabledCount, jsonSummary, onTabChange, applyJSON, formatJSON } = useSpiderTasks(data, { initialTab: "browser" });

// 字段与后端 spiderTask 对齐（含无头专属：渲染等待/翻页滚动/取链方式/正文翻页/集名）
function newTask() {
  return {
    name: "新任务",
    enable: false,
    source_url: "",
    page_url_pattern: "",
    max_pages: 1,
    next_page_sel: "",
    scroll_times: 0,
    mode: "detail",
    wait_selector: "",
    list_selector: "",
    link_mode: "",
    link_selector: "",
    link_attr: "",
    link_extract_regex: "",
    link_url_template: "",
    link_cross_origin: false,
    click_wait_ms: 250,
    link_include: "",
    link_exclude: "",
    stop_when_exists: 0,
    list_title_sel: "",
    list_cover_sel: "",
    list_cover_attr: "",
    list_cover_extract_regex: "",
    list_desc_sel: "",
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
  };
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
</script>

<style scoped>
.input {
  width: 100%;
}

.headless-spider-tabs {
  max-height: 70vh;
  overflow-y: auto;
}

.headless-spider-tabs :deep(.arco-tabs-content) {
  padding-top: 8px;
}
</style>
