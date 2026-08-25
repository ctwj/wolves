<template>
  <div class="video-sources-editor">
    <div v-if="!sources.length" class="text-gray-400 text-sm py-2">暂无选集，点击下方"添加选集"录入视频地址</div>
    <div v-for="(item, i) in sources" :key="i" class="source-row">
      <a-input v-model="item.label" :max-length="100" placeholder="集数名称，如：第01集" class="label-input" />
      <a-input v-model="item.url" placeholder="视频地址（直链 mp4 或 播放页地址）" allow-clear class="url-input" />
      <a-tooltip content="嵌入页地址（第三方播放页）用 iframe 打开；直链 mp4 关闭此项">
        <a-switch v-model="item.embed" type="round" checked-text="嵌入" unchecked-text="直链" />
      </a-tooltip>
      <a-button size="small" @click="move(i, -1)" :disabled="i === 0"><icon-up /></a-button>
      <a-button size="small" @click="move(i, 1)" :disabled="i === sources.length - 1"><icon-down /></a-button>
      <a-button size="small" status="danger" @click="remove(i)"><icon-delete /></a-button>
    </div>
    <a-button size="small" type="primary" long @click="add"><template #icon><icon-plus /></template>添加选集</a-button>
  </div>
</template>

<script setup>
  import {ref, watch} from 'vue'

  const props = defineProps({modelValue: {type: Array, default: () => []}})
  const emit = defineEmits(['update:modelValue'])

  // 内部维护行对象；提交时序列化为 extends["video_sources"] 的数组结构
  const sources = ref(normalize(props.modelValue))

  watch(() => props.modelValue, (val) => {
    const norm = normalize(val)
    // 避免回显时与自身 emit 的值互相触发重置
    if (JSON.stringify(norm) !== JSON.stringify(sources.value)) sources.value = norm
  })
  watch(sources, (val) => emit('update:modelValue', val.map(v => ({...v}))), {deep: true})

  function normalize(val) {
    if (!Array.isArray(val)) return []
    return val.map(v => ({
      label: typeof v?.label === 'string' ? v.label : '',
      url: typeof v?.url === 'string' ? v.url : '',
      embed: v?.embed === true,
    }))
  }

  function add() {
    sources.value.push({label: '', url: '', embed: false})
  }

  function remove(i) {
    sources.value.splice(i, 1)
  }

  function move(i, offset) {
    const j = i + offset
    if (j < 0 || j >= sources.value.length) return
    const [item] = sources.value.splice(i, 1)
    sources.value.splice(j, 0, item)
  }
</script>

<style scoped>
  .source-row {
    display: flex;
    gap: 6px;
    align-items: center;
    margin-bottom: 6px;
  }
  .label-input { width: 140px; flex: none; }
  .url-input { flex: 1; }
</style>
