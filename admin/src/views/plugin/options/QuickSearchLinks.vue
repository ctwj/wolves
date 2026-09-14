<template>

  <a-form-item label="启用">
    <a-switch v-model="data.enable" />
    <template #extra>
      <a-typography-text type="secondary" class="text-xs">
        开启后 wolves 主题首页（轮播下方）展示快捷搜索入口；其他主题不受影响。
      </a-typography-text>
    </template>
  </a-form-item>
  <a-form-item label="区块标题">
    <a-input v-model="data.title" placeholder="如 快速搜索 / 热门搜索" allow-clear />
    <template #extra>
      <a-typography-text type="secondary" class="text-xs">当前版本前台已隐藏标题，仅作保留配置。</a-typography-text>
    </template>
  </a-form-item>

  <a-divider orientation="left" :margin="8">
    快捷入口
    <a-tag v-if="items.length" color="arcoblue" size="small" class="ml-1">{{ items.length }}</a-tag>
  </a-divider>

  <!-- 所见即所得预览：模拟 wolves 首页暗色环境 -->
  <div class="pv-bar" role="status" aria-atomic="true">
    <span class="pv-label"><icon-eye :size="14" /> 前台预览</span>
    <div v-if="items.length" class="pv-list">
      <span v-for="(it, i) in items" :key="i" class="pv-chip" :class="safeColor(it.color)">
        <i v-if="safeIcon(it.icon)" :class="safeIcon(it.icon)" aria-hidden="true"></i>{{ it.label || '未填文案' }}
      </span>
    </div>
    <span v-else class="pv-empty">暂无入口，点击下方「添加入口」</span>
  </div>

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
        </div>
      </template>
    </a-popover>

    <a-input v-model="it.label" aria-label="展示文案" placeholder="文案 如 微信多开" style="width: 150px" />
    <a-input v-model="it.keyword" aria-label="搜索词" placeholder="搜索词 留空=文案" style="width: 130px" />

    <!-- 安全色系：6 个预设圆点，不可自由输入 -->
    <div class="qs-colors" role="radiogroup" :aria-label="`第${i + 1}项色系`">
      <button v-for="c in COLORS" :key="c.value" type="button" class="qs-color-dot" :class="[c.value, { active: safeColor(it.color) === c.value }]"
        role="radio" :aria-checked="safeColor(it.color) === c.value" :title="c.name" @click="it.color = c.value">
        <i class="fas fa-check" aria-hidden="true"></i>
      </button>
    </div>

    <a-button type="text" size="mini" :disabled="i === 0" aria-label="上移" @click="move(i, -1)"><template #icon><icon-up :size="14" /></template></a-button>
    <a-button type="text" size="mini" :disabled="i === items.length - 1" aria-label="下移" @click="move(i, 1)"><template #icon><icon-down :size="14" /></template></a-button>
    <a-popconfirm content="删除该入口？" type="warning" @ok="items.splice(i, 1)">
      <a-button type="text" size="mini" status="danger" aria-label="删除"><template #icon><icon-delete :size="14" /></template></a-button>
    </a-popconfirm>
  </div>
  <a-button size="small" type="outline" @click="add">
    <template #icon><icon-plus :size="14" /></template>
    添加入口
  </a-button>

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

// ===== 安全色系（与 wolves 前台 .w-qs-chip 六色一一对应，不开放自由输入）=====
const COLORS = [
  { value: "blue", name: "蓝" },
  { value: "green", name: "绿" },
  { value: "amber", name: "琥珀" },
  { value: "pink", name: "粉" },
  { value: "violet", name: "紫" },
  { value: "cyan", name: "青" },
];
function safeColor(v) {
  return COLORS.some((c) => c.value === v) ? v : "blue";
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
  items.value.push({ label: "", keyword: "", icon: "fas fa-bolt", color: "blue" });
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
/* ===== 前台预览条（模拟 wolves 暗色背景）===== */
.pv-bar {
  background: #0f172a;
  border-radius: 8px;
  padding: 12px 16px;
  margin-bottom: 14px;
}
.pv-label {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  font-size: 11px;
  color: rgba(255, 255, 255, 0.4);
  margin-bottom: 8px;
}
.pv-list { display: flex; flex-wrap: wrap; gap: 10px 8px; }
.pv-empty { font-size: 12px; color: rgba(255, 255, 255, 0.35); }
.pv-chip {
  display: inline-flex; align-items: center; gap: 6px; padding: 6px 14px; border-radius: 9999px;
  font-size: 13px; font-weight: 500; border: 1px solid transparent; line-height: 1.4;
}
.pv-chip.blue   { background: rgba(59, 130, 246, .14);  color: #93c5fd; border-color: rgba(59, 130, 246, .28); }
.pv-chip.green  { background: rgba(52, 211, 153, .12);  color: #6ee7b7; border-color: rgba(52, 211, 153, .26); }
.pv-chip.amber  { background: rgba(251, 191, 36, .12);  color: #fcd34d; border-color: rgba(251, 191, 36, .26); }
.pv-chip.pink   { background: rgba(244, 114, 182, .12); color: #f9a8d4; border-color: rgba(244, 114, 182, .26); }
.pv-chip.violet { background: rgba(167, 139, 250, .12); color: #c4b5fd; border-color: rgba(167, 139, 250, .26); }
.pv-chip.cyan   { background: rgba(34, 211, 238, .12);  color: #67e8f9; border-color: rgba(34, 211, 238, .26); }

/* ===== 行布局 ===== */
.qs-row { display: flex; align-items: center; gap: 8px; margin-bottom: 8px; flex-wrap: wrap; }
.qs-seq { color: var(--color-text-3, #86909c); font-size: 12px; width: 20px; text-align: center; }

/* 图标按钮：实时渲染当前图标 */
.qs-icon-btn {
  width: 36px; height: 32px; border-radius: 6px; border: 1px solid var(--color-border-2, #e5e6eb);
  background: var(--color-fill-1, #f7f8fa); cursor: pointer; color: var(--color-text-2, #4e5969);
  display: inline-flex; align-items: center; justify-content: center; transition: background-color .2s, border-color .2s;
}
.qs-icon-btn:hover { background: var(--color-fill-3, #e5e6eb); }

/* ===== 安全色圆点 ===== */
.qs-colors { display: inline-flex; gap: 6px; }
.qs-color-dot {
  width: 22px; height: 22px; border-radius: 9999px; border: 1px solid transparent; cursor: pointer;
  display: inline-flex; align-items: center; justify-content: center; padding: 0;
  transition: transform .2s, box-shadow .2s;
}
.qs-color-dot i { font-size: 10px; color: #fff; opacity: 0; }
.qs-color-dot.active i { opacity: 1; }
.qs-color-dot:hover { transform: scale(1.15); }
.qs-color-dot.blue   { background: #3b82f6; }
.qs-color-dot.green  { background: #34d399; }
.qs-color-dot.amber  { background: #fbbf24; }
.qs-color-dot.pink   { background: #f472b6; }
.qs-color-dot.violet { background: #a78bfa; }
.qs-color-dot.cyan   { background: #22d3ee; }
.qs-color-dot.active { box-shadow: 0 0 0 2px var(--color-bg-2, #fff), 0 0 0 4px rgb(var(--primary-6, 22, 93, 255)); }
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
</style>
