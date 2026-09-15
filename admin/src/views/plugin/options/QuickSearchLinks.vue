<template>

  <a-tabs default-active-key="items" size="small">
    <!-- ==================== 入口配置 ==================== -->
    <a-tab-pane key="items" title="入口配置">
      <a-form-item label="启用">
        <a-switch v-model="data.enable" />
        <template #extra>
          <a-typography-text type="secondary" class="text-xs">
            开启后 wolves 主题首页（轮播下方）展示快捷搜索入口；其他主题不受影响。效果预览见「前台预览」页签。
          </a-typography-text>
        </template>
      </a-form-item>

      <a-divider orientation="left" :margin="8">
        快捷入口（每行一条）
        <a-tag v-if="items.length" color="arcoblue" size="small" class="ml-1">{{ items.length }}</a-tag>
      </a-divider>

      <div class="qs-list-scroll">
        <div v-for="(it, i) in items" :key="i" class="qs-row">
        <span class="qs-seq">{{ i + 1 }}</span>

        <!-- 图标选择器：精选库网格 + 折叠自定义（格式校验） -->
        <a-popover trigger="click" position="bottom" content-class="qs-icon-popover" :unmount-on-close="true">
          <button type="button" class="qs-icon-btn" :aria-label="`选择第${i + 1}项图标`">
            <i v-if="safeIcon(it.icon)" :class="safeIcon(it.icon)" aria-hidden="true"></i>
            <i v-else class="fas fa-circle-question" aria-hidden="true"></i>
          </button>
          <template #content>
            <div class="qs-icon-picker">
              <input v-model="iconSearch" class="qs-icon-search" placeholder="搜索图标（中文名）" aria-label="搜索图标" />
              <div v-for="g in filteredIconGroups" :key="g.name" class="qs-icon-group">
                <div class="qs-icon-group-name">{{ g.name }}</div>
                <div class="qs-icon-grid">
                  <button v-for="ic in g.items" :key="ic.cls" type="button" class="qs-icon-cell"
                    :class="{ active: it.icon === ic.cls }" :title="ic.name + '  ' + ic.cls" @click="it.icon = ic.cls; iconSearch = ''">
                    <i :class="ic.cls" aria-hidden="true"></i>
                  </button>
                </div>
              </div>
              <div v-if="!filteredIconGroups.length" class="qs-icon-none">无匹配图标，可展开自定义输入</div>
          <details class="qs-icon-custom">
            <summary>自定义 class（限 Font Awesome 格式）</summary>
            <input :value="it.icon" class="qs-icon-custom-input" placeholder="如 fas fa-bolt 或 fab fa-weixin"
              aria-label="自定义图标class" @input="onCustomIcon(it, $event)" />
            <div class="qs-icon-custom-tip">仅支持 Font Awesome 6 免费图标 class（fas/fab 前缀），完整列表见 fontawesome.com/icons（Free）</div>
          </details>
          <button type="button" class="qs-icon-clear" @click="it.icon = ''">清除图标（纯文字胶囊）</button>
            </div>
          </template>
        </a-popover>

        <a-input v-model="it.label" aria-label="展示文案" placeholder="文案 如 微信多开" style="width: 120px" />
        <a-radio-group :model-value="it.link ? 'link' : 'search'" type="button" size="mini"
          @update:model-value="(v) => { if (v === 'link') { it.link = 'https://'; it.keyword = ''; } else { it.link = ''; } }">
          <a-radio value="search">搜索</a-radio>
          <a-radio value="link">链接</a-radio>
        </a-radio-group>
        <a-input v-if="!it.link" v-model="it.keyword" aria-label="搜索词" placeholder="搜索词 留空=文案" class="qs-flex-input" />
        <a-input v-else v-model="it.link" aria-label="链接地址" placeholder="https:// 外链或 /category/xx" class="qs-flex-input" />

        <!-- 安全色系：下拉选择（默认黑 + 12 个预设），可不配置 -->
        <a-select :model-value="safeColor(it.color)" aria-label="色系" style="width: 104px" @update:model-value="(v) => it.color = v">
          <a-option v-for="c in COLORS" :key="c.value" :value="c.value">
            <span class="qs-option"><span class="qs-dot" :class="c.value || 'default'"></span>{{ c.name }}</span>
          </a-option>
        </a-select>

        <span class="qs-ops">
          <a-button type="text" size="mini" :disabled="i === 0" aria-label="上移" @click="move(i, -1)"><template #icon><icon-up :size="14" /></template></a-button>
          <a-button type="text" size="mini" :disabled="i === items.length - 1" aria-label="下移" @click="move(i, 1)"><template #icon><icon-down :size="14" /></template></a-button>
          <a-popconfirm content="删除该入口？" type="warning" @ok="items.splice(i, 1)">
            <a-button type="text" size="mini" status="danger" aria-label="删除"><template #icon><icon-delete :size="14" /></template></a-button>
          </a-popconfirm>
        </span>
        </div>
      </div>
      <a-button size="small" type="outline" @click="add">
        <template #icon><icon-plus :size="14" /></template>
        添加入口
      </a-button>
    </a-tab-pane>

    <!-- ==================== 前台预览 ==================== -->
    <a-tab-pane key="preview" title="前台预览">
      <a-typography-text type="secondary" class="text-xs block mb-2">
        模拟 wolves 首页暗色环境实时渲染；保存后在插件弹窗点「确定」生效。
      </a-typography-text>
      <div class="pv-bar" role="status" aria-atomic="true">
        <div v-if="items.length" class="pv-list">
          <span v-for="(it, i) in items" :key="i" class="pv-chip" :class="safeColor(it.color)">
            <i v-if="safeIcon(it.icon)" :class="safeIcon(it.icon)" aria-hidden="true"></i>{{ it.label || '未填文案' }}
            <span v-if="it.link" class="pv-ext" aria-hidden="true">↗</span>
          </span>
        </div>
        <span v-else class="pv-empty">暂无入口，回到「入口配置」添加</span>
      </div>
    </a-tab-pane>
  </a-tabs>

</template>

<script setup>
import { computed, inject, onMounted, ref } from "vue";
import { Message } from "@arco-design/web-vue";

const data = inject("options");

// 兼容旧数据：items 缺失时给空数组
const items = computed(() => {
  if (!Array.isArray(data.value.items)) data.value.items = [];
  return data.value.items;
});

// ===== 安全色系（默认黑 + 12 个预设，与 wolves 前台一一对应，不开放自由输入）=====
const COLORS = [
  { value: "", name: "默认(黑)" },
  { value: "blue", name: "蓝" },
  { value: "green", name: "绿" },
  { value: "amber", name: "琥珀" },
  { value: "orange", name: "橙" },
  { value: "pink", name: "粉" },
  { value: "violet", name: "紫" },
  { value: "cyan", name: "青" },
  { value: "teal", name: "青绿" },
  { value: "red", name: "红" },
  { value: "indigo", name: "靛蓝" },
  { value: "lime", name: "黄绿" },
  { value: "slate", name: "灰" },
];
function safeColor(v) {
  return v === "" || COLORS.some((c) => c.value === v) ? v : "";
}

// ===== 精选图标库（Font Awesome 6 Free，前台加载同一 CDN 资源）=====
const ICON_GROUPS = [
  { name: "通用", items: [
    { cls: "fas fa-bolt", name: "闪电" }, { cls: "fas fa-star", name: "星" }, { cls: "fas fa-heart", name: "心" },
    { cls: "fas fa-fire", name: "火" }, { cls: "fas fa-download", name: "下载" }, { cls: "fas fa-magnifying-glass", name: "搜索" },
    { cls: "fas fa-house", name: "主页" }, { cls: "fas fa-gear", name: "设置" }, { cls: "fas fa-list-ul", name: "列表" },
    { cls: "fas fa-tags", name: "标签" }, { cls: "fas fa-folder", name: "文件夹" }, { cls: "fas fa-bookmark", name: "书签" },
    { cls: "fas fa-link", name: "链接" }, { cls: "fas fa-gift", name: "礼物" }, { cls: "fas fa-bell", name: "铃铛" },
    { cls: "fas fa-clock", name: "时钟" }, { cls: "fas fa-moon", name: "月亮" }, { cls: "fas fa-sun", name: "太阳" },
  ]},
  { name: "多媒体", items: [
    { cls: "fas fa-play", name: "播放" }, { cls: "fas fa-film", name: "电影" }, { cls: "fas fa-tv", name: "电视" },
    { cls: "fas fa-music", name: "音乐" }, { cls: "fas fa-image", name: "图片" }, { cls: "fas fa-images", name: "图集" },
    { cls: "fas fa-camera", name: "相机" }, { cls: "fas fa-video", name: "视频" }, { cls: "fas fa-headphones", name: "耳机" },
    { cls: "fas fa-microphone", name: "麦克风" }, { cls: "fas fa-book", name: "书" }, { cls: "fas fa-book-open", name: "翻开书" },
    { cls: "fas fa-newspaper", name: "报纸" }, { cls: "fas fa-clapperboard", name: "场记板" },
  ]},
  { name: "软件工具", items: [
    { cls: "fas fa-window-maximize", name: "窗口" }, { cls: "fas fa-cube", name: "立方体" }, { cls: "fas fa-terminal", name: "终端" },
    { cls: "fas fa-code", name: "代码" }, { cls: "fas fa-database", name: "数据库" }, { cls: "fas fa-server", name: "服务器" },
    { cls: "fas fa-cloud", name: "云" }, { cls: "fas fa-wifi", name: "无线" }, { cls: "fas fa-plug", name: "插件" },
    { cls: "fas fa-shield-halved", name: "盾牌" }, { cls: "fas fa-lock", name: "锁" }, { cls: "fas fa-key", name: "钥匙" },
    { cls: "fas fa-wrench", name: "扳手" }, { cls: "fas fa-hammer", name: "锤子" }, { cls: "fas fa-broom", name: "扫帚" },
    { cls: "fas fa-recycle", name: "回收" }, { cls: "fas fa-toolbox", name: "工具箱" },
  ]},
  { name: "网络通讯", items: [
    { cls: "fas fa-globe", name: "地球" }, { cls: "fas fa-rocket", name: "火箭" }, { cls: "fas fa-satellite", name: "卫星" },
    { cls: "fas fa-share-nodes", name: "分享" }, { cls: "fas fa-rss", name: "订阅" }, { cls: "fas fa-comments", name: "评论" },
    { cls: "fas fa-envelope", name: "邮件" }, { cls: "fas fa-signal", name: "信号" },
  ]},
  { name: "品牌", items: [
    { cls: "fab fa-weixin", name: "微信" }, { cls: "fab fa-qq", name: "QQ" }, { cls: "fab fa-windows", name: "Windows" },
    { cls: "fab fa-apple", name: "苹果" }, { cls: "fab fa-android", name: "安卓" }, { cls: "fab fa-google-play", name: "谷歌商店" },
    { cls: "fab fa-github", name: "GitHub" }, { cls: "fab fa-bilibili", name: "B站" }, { cls: "fab fa-telegram", name: "电报" },
    { cls: "fab fa-discord", name: "Discord" }, { cls: "fab fa-youtube", name: "油管" }, { cls: "fab fa-chrome", name: "Chrome" },
    { cls: "fab fa-edge", name: "Edge" }, { cls: "fab fa-firefox", name: "Firefox" },
  ]},
];
const iconSearch = ref("");
const filteredIconGroups = computed(() => {
  const kw = iconSearch.value.trim().toLowerCase();
  if (!kw) return ICON_GROUPS;
  return ICON_GROUPS.map((g) => ({
    name: g.name,
    items: g.items.filter((ic) => ic.name.includes(kw) || ic.cls.toLowerCase().includes(kw)),
  })).filter((g) => g.items.length);
});

// 图标 class 安全校验：仅 Font Awesome 格式，非法值前台按默认隐藏
const FA_RE = /^fa[bsr]?\s+fa-[a-z0-9-]+$/i;
function safeIcon(v) {
  return FA_RE.test(v || "") ? v : "";
}
function onCustomIcon(it, e) {
  const v = e.target.value.trim();
  if (v === "" || FA_RE.test(v)) {
    it.icon = v;
    return;
  }
  Message.warning("图标 class 格式应为 fas fa-xxx 或 fab fa-xxx");
}

function add() {
  // 最简配置：只填文案即可用（图标/色系留空 → 前台默认黑色纯文字胶囊）
  items.value.push({ label: "", keyword: "", link: "", icon: "", color: "" });
}

function move(i, offset) {
  const j = i + offset;
  if (j < 0 || j >= items.value.length) return;
  const [item] = items.value.splice(i, 1);
  items.value.splice(j, 0, item);
}

// 管理后台本身不含 Font Awesome，按需注入与前台一致的 CDN（已存在则跳过）
onMounted(() => {
  const href = "https://cdnjs.cloudflare.com/ajax/libs/font-awesome/6.4.0/css/all.min.css";
  if (!document.querySelector(`link[href="${href}"]`)) {
    const link = document.createElement("link");
    link.rel = "stylesheet";
    link.href = href;
    document.head.appendChild(link);
  }
});
</script>

<style scoped>
/* ===== 单行条目 ===== */
/* 超量滚动：条目多时不撑开弹窗，固定高度内滚动；「添加入口」按钮保持在容器外常显 */
.qs-list-scroll { max-height: 46vh; overflow-y: auto; padding-right: 4px; margin-bottom: 8px; }
.qs-list-scroll::-webkit-scrollbar { width: 6px; }
.qs-list-scroll::-webkit-scrollbar-thumb { background: var(--color-border-2, #e5e6eb); border-radius: 9999px; }
.qs-row { display: flex; align-items: center; gap: 6px; margin-bottom: 8px; flex-wrap: nowrap; }
.qs-seq { color: var(--color-text-3, #86909c); font-size: 12px; width: 18px; text-align: center; flex-shrink: 0; }
.qs-flex-input { flex: 1 1 140px; min-width: 120px; }
.qs-ops { display: inline-flex; gap: 2px; flex-shrink: 0; }

/* 图标按钮：实时渲染当前图标 */
.qs-icon-btn {
  width: 34px; height: 30px; border-radius: 6px; border: 1px solid var(--color-border-2, #e5e6eb);
  background: var(--color-fill-1, #f7f8fa); cursor: pointer; color: var(--color-text-2, #4e5969);
  display: inline-flex; align-items: center; justify-content: center; flex-shrink: 0;
  transition: background-color .2s, border-color .2s;
}
.qs-icon-btn:hover { background: var(--color-fill-3, #e5e6eb); }

/* 下拉选项中的色点 */
.qs-option { display: inline-flex; align-items: center; gap: 6px; }
.qs-dot { width: 12px; height: 12px; border-radius: 9999px; display: inline-block; }
.qs-dot.default { background: #1f2937; border: 1px solid rgba(255, 255, 255, .25); }
.qs-dot.blue   { background: #3b82f6; }
.qs-dot.green  { background: #34d399; }
.qs-dot.amber  { background: #fbbf24; }
.qs-dot.orange { background: #fb923c; }
.qs-dot.pink   { background: #f472b6; }
.qs-dot.violet { background: #a78bfa; }
.qs-dot.cyan   { background: #22d3ee; }
.qs-dot.teal   { background: #2dd4bf; }
.qs-dot.red    { background: #f87171; }
.qs-dot.indigo { background: #818cf8; }
.qs-dot.lime   { background: #a3e635; }
.qs-dot.slate  { background: #94a3b8; }

/* ===== 前台预览（模拟 wolves 暗色背景）===== */
.pv-bar { background: #0f172a; border-radius: 8px; padding: 20px 16px 16px; }
.pv-list { display: flex; flex-wrap: wrap; gap: 10px 8px; }
.pv-empty { font-size: 12px; color: rgba(255, 255, 255, 0.35); }
.pv-chip {
  display: inline-flex; align-items: center; gap: 6px; padding: 6px 14px; border-radius: 9999px;
  font-size: 13px; font-weight: 500; border: 1px solid transparent; line-height: 1.4;
  background: rgba(0, 0, 0, .45); color: rgba(255, 255, 255, .78); border-color: rgba(255, 255, 255, .12);
}
.pv-ext { font-size: 10px; opacity: .7; }
.pv-chip.blue   { background: rgba(59, 130, 246, .14);  color: #93c5fd; border-color: rgba(59, 130, 246, .28); }
.pv-chip.green  { background: rgba(52, 211, 153, .12);  color: #6ee7b7; border-color: rgba(52, 211, 153, .26); }
.pv-chip.amber  { background: rgba(251, 191, 36, .12);  color: #fcd34d; border-color: rgba(251, 191, 36, .26); }
.pv-chip.orange { background: rgba(251, 146, 60, .12);  color: #fdba74; border-color: rgba(251, 146, 60, .26); }
.pv-chip.pink   { background: rgba(244, 114, 182, .12); color: #f9a8d4; border-color: rgba(244, 114, 182, .26); }
.pv-chip.violet { background: rgba(167, 139, 250, .12); color: #c4b5fd; border-color: rgba(167, 139, 250, .26); }
.pv-chip.cyan   { background: rgba(34, 211, 238, .12);  color: #67e8f9; border-color: rgba(34, 211, 238, .26); }
.pv-chip.teal   { background: rgba(45, 212, 191, .12);  color: #5eead4; border-color: rgba(45, 212, 191, .26); }
.pv-chip.red    { background: rgba(248, 113, 113, .12); color: #fca5a5; border-color: rgba(248, 113, 113, .26); }
.pv-chip.indigo { background: rgba(129, 140, 248, .12); color: #a5b4fc; border-color: rgba(129, 140, 248, .26); }
.pv-chip.lime   { background: rgba(163, 230, 53, .12);  color: #bef264; border-color: rgba(163, 230, 53, .26); }
.pv-chip.slate  { background: rgba(148, 163, 184, .14); color: #cbd5e1; border-color: rgba(148, 163, 184, .26); }
</style>

<style>
/* 图标选择弹层（popover 渲染在 body 下，需全局样式）*/
.qs-icon-popover .arco-popover-content { margin-top: 2px; }
.qs-icon-picker { width: 400px; max-height: 320px; overflow-y: auto; }
.qs-icon-search {
  width: 100%; box-sizing: border-box; padding: 6px 10px; margin-bottom: 8px;
  border: 1px solid var(--color-border-2, #e5e6eb); border-radius: 6px; outline: none;
}
.qs-icon-search:focus { border-color: rgb(var(--primary-6, 22, 93, 255)); }
.qs-icon-group-name { font-size: 11px; color: var(--color-text-3, #86909c); margin: 8px 0 4px; }
.qs-icon-grid { display: grid; grid-template-columns: repeat(10, 1fr); gap: 4px; }
.qs-icon-cell {
  height: 32px; border: 1px solid transparent; border-radius: 6px; background: transparent; cursor: pointer;
  color: var(--color-text-2, #4e5969); display: inline-flex; align-items: center; justify-content: center;
  transition: background-color .15s, border-color .15s;
}
.qs-icon-cell:hover { background: var(--color-fill-2, #f2f3f5); }
.qs-icon-cell.active { background: rgb(var(--primary-1, 232, 243, 255)); border-color: rgb(var(--primary-6, 22, 93, 255)); color: rgb(var(--primary-6, 22, 93, 255)); }
.qs-icon-none { font-size: 12px; color: var(--color-text-3, #86909c); padding: 8px 0; }
.qs-icon-custom { margin-top: 10px; border-top: 1px solid var(--color-border-2, #e5e6eb); padding-top: 8px; }
.qs-icon-custom summary { font-size: 12px; color: var(--color-text-3, #86909c); cursor: pointer; }
.qs-icon-custom-input {
  width: 100%; box-sizing: border-box; padding: 6px 10px; margin-top: 6px;
  border: 1px solid var(--color-border-2, #e5e6eb); border-radius: 6px; outline: none;
  font-family: ui-monospace, Consolas, monospace;
}
.qs-icon-custom-tip { font-size: 11px; color: var(--color-text-3, #86909c); margin-top: 4px; }
.qs-icon-clear {
  margin-top: 8px; width: 100%; padding: 6px 0; font-size: 12px; cursor: pointer;
  border: 1px dashed var(--color-border-2, #e5e6eb); border-radius: 6px;
  background: transparent; color: var(--color-text-3, #86909c); transition: color .15s, border-color .15s;
}
.qs-icon-clear:hover { color: rgb(var(--danger-6, 245, 63, 63)); border-color: rgb(var(--danger-6, 245, 63, 63)); }
</style>
