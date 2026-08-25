<template>
  <div class="gallery-images-editor">
    <div v-if="!images.length" class="text-gray-400 text-sm py-2">暂无图片，点击下方"上传图片"或"添加图片地址"</div>
    <div class="gallery-grid">
      <div v-for="(url, i) in images" :key="i" class="gallery-item">
        <img :src="url" alt="" loading="lazy" @error="onImgError($event)" />
        <div class="gallery-ops">
          <a-button size="mini" @click="move(i, -1)" :disabled="i === 0"><icon-up /></a-button>
          <a-button size="mini" @click="move(i, 1)" :disabled="i === images.length - 1"><icon-down /></a-button>
          <a-button size="mini" status="danger" @click="remove(i)"><icon-delete /></a-button>
        </div>
      </div>
    </div>
    <a-input-search v-model="manualUrl" placeholder="粘贴图片地址后回车添加" search-button @search="addManual" class="mb-2" />
    <a-button size="small" type="primary" long :loading="uploading" @click="fileInput.click()">
      <template #icon><icon-upload /></template>上传图片（可多选）
    </a-button>
    <input class="hidden" type="file" ref="fileInput" accept="image/*" multiple @change="getFile" />
  </div>
</template>

<script setup>
  import {ref, watch} from 'vue'
  import {useRequest} from "vue-request";
  import {upload as uploadAPI} from "@/api/index.js";
  import {Message} from "@arco-design/web-vue";

  const props = defineProps({modelValue: {type: Array, default: () => []}})
  const emit = defineEmits(['update:modelValue'])

  const images = ref(normalize(props.modelValue))
  const manualUrl = ref('')
  const fileInput = ref()
  const uploading = ref(false)

  watch(() => props.modelValue, (val) => {
    const norm = normalize(val)
    if (JSON.stringify(norm) !== JSON.stringify(images.value)) images.value = norm
  })
  watch(images, (val) => emit('update:modelValue', [...val]), {deep: true})

  function normalize(val) {
    if (!Array.isArray(val)) return []
    return val.filter(v => typeof v === 'string' && v.trim() !== '')
  }

  function addManual() {
    const url = manualUrl.value.trim()
    if (!url) return
    images.value.push(url)
    manualUrl.value = ''
  }

  function remove(i) {
    images.value.splice(i, 1)
  }

  function move(i, offset) {
    const j = i + offset
    if (j < 0 || j >= images.value.length) return
    const [item] = images.value.splice(i, 1)
    images.value.splice(j, 0, item)
  }

  const {run: upload} = useRequest(uploadAPI, {
    manual: true,
    onBefore: () => uploading.value = true,
    onSuccess: (resp) => {
      if (resp.success && Array.isArray(resp.data)) {
        for (const url of resp.data) {
          if (typeof url === 'string' && url) images.value.push(url)
        }
      } else {
        Message.error('上传失败')
      }
    },
    onFinally: () => uploading.value = false,
  })

  function getFile(event) {
    let formFile = new FormData();
    for (let f of event.target.files) {
      formFile.append("file", f)
    }
    if (formFile.has("file")) upload(formFile)
    fileInput.value.value = ""
  }

  function onImgError(e) {
    e.target.style.opacity = '0.3'
  }
</script>

<style scoped>
  .gallery-grid {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(120px, 1fr));
    gap: 8px;
    margin-bottom: 8px;
  }
  .gallery-item {
    position: relative;
    border: 1px solid var(--color-border-2);
    border-radius: 6px;
    overflow: hidden;
    background: var(--color-fill-2);
  }
  .gallery-item img {
    width: 100%;
    height: 90px;
    object-fit: cover;
    display: block;
  }
  .gallery-ops {
    display: flex;
    gap: 4px;
    justify-content: center;
    padding: 4px 0;
  }
</style>
