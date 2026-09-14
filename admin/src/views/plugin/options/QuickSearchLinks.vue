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
  </a-form-item>

  <a-divider orientation="left" :margin="8">
    快捷入口
    <a-tag v-if="items.length" color="arcoblue" size="small" class="ml-1">{{ items.length }}</a-tag>
  </a-divider>
  <a-typography-text type="secondary" class="text-xs block mb-2">
    点击直达站内搜索结果页；留空搜索词时按展示文案搜索。
  </a-typography-text>

  <div v-for="(it, i) in items" :key="i" class="flex items-center gap-2 mb-2 flex-wrap">
    <span class="text-gray-400 text-xs w-5 text-center">{{ i + 1 }}</span>
    <a-input v-model="it.label" aria-label="展示文案" placeholder="文案 如 微信多开" style="width: 140px" />
    <a-input v-model="it.keyword" aria-label="搜索词" placeholder="搜索词 留空=文案" style="width: 130px" />
    <a-input v-model="it.icon" aria-label="图标" placeholder="fas fa-bolt" style="width: 150px" />
    <a-select v-model="it.color" aria-label="色系" style="width: 100px" placeholder="色系">
      <a-option value="blue">蓝</a-option>
      <a-option value="green">绿</a-option>
      <a-option value="amber">琥珀</a-option>
      <a-option value="pink">粉</a-option>
      <a-option value="violet">紫</a-option>
      <a-option value="cyan">青</a-option>
    </a-select>
    <a-button type="text" size="mini" :disabled="i === 0" @click="move(i, -1)"><template #icon><icon-up :size="14" /></template></a-button>
    <a-button type="text" size="mini" :disabled="i === items.length - 1" @click="move(i, 1)"><template #icon><icon-down :size="14" /></template></a-button>
    <a-popconfirm content="删除该入口？" type="warning" @ok="items.splice(i, 1)">
      <a-button type="text" size="mini" status="danger"><template #icon><icon-delete :size="14" /></template></a-button>
    </a-popconfirm>
  </div>
  <a-button size="small" type="outline" @click="add">
    <template #icon><icon-plus :size="14" /></template>
    添加入口
  </a-button>

</template>

<script setup>
import { computed, inject } from "vue";

const data = inject("options");

// 兼容旧数据：items 缺失时给空数组
const items = computed(() => {
  if (!Array.isArray(data.value.items)) data.value.items = [];
  return data.value.items;
});

function add() {
  items.value.push({ label: "", keyword: "", icon: "fas fa-bolt", color: "blue" });
}

function move(i, offset) {
  const j = i + offset;
  if (j < 0 || j >= items.value.length) return;
  const [item] = items.value.splice(i, 1);
  items.value.splice(j, 0, item);
}
</script>
