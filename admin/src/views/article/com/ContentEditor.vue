<template>
  <Editor
      api-key="5pfu2qh1f87sn457oomloc7pfvx7uvdp4ot6xacn1v1s8lks"
      v-model="content"
      :init="editorConfig"
      :disabled="disabled"
      :api-key="apiKey"
      class="tinymce-editor"
      :class="{dark:store.dark}"
  />
  <div class="absolute z-5 bottom-2 right-3 cursor-pointer opacity-10 hover:opacity-20 hover:text-blue-800 transition"
       :class="{'hover:text-white':store.dark}" @click="visible = true">
    <icon-code-square :size="50" />
  </div>
  <a-modal width="96%" v-model:visible="visible" @cancel="modalClose" unmount-on-close
           modal-class="codeModal"
           :mask-style="{backdropFilter: 'blur(2px)'}"
           :modal-style="{height:'96%',padding:'10px',backgroundColor:store.dark ? '#282c34':'#f5f5f5'}"
           :body-style="{height:'100%',overflow:'hidden'}"
           simple
           :footer="false"
  >
    <ContentHtmlCode ref="codeRef" />
    <div class="cursor-pointer absolute right-1 top-1 opacity-10 hover:opacity-20 hover:text-blue-800 transition"
         :class="{'opacity-20 hover:text-white':store.dark}" @click="modalClose">
      <icon-close-circle :size="40" />
    </div>
  </a-modal>
</template>

<script setup>
  import {inject, ref, watch, onBeforeUnmount, computed } from 'vue'
  import Editor from '@tinymce/tinymce-vue'
  import ContentHtmlCode from './ContentHtmlCode.vue'
  import { useStore } from "@/store"
  import { upload } from "@/api"
  import { t } from "@/locale"

  const store = useStore()
  const record = inject('record')
  const content = ref('')
  const editorRef = ref(null)
  const disabled = ref(false)
  const visible = ref(false)
  const codeRef = ref(null)

  // TinyMCE 配置
  const apiKey = 'no-api-key' // 使用无 API Key 模式（仅限功能）
  const editorConfig = {
    height: '100%',
    menubar: false,
    language: store.locale === 'zh-cn' ? 'zh_CN' : 'en',
    plugins: [
      'advlist', 'autolink', 'lists', 'link', 'image', 'charmap', 'preview',
      'anchor', 'searchreplace', 'visualblocks', 'code', 'fullscreen',
      'insertdatetime', 'media', 'table', 'help', 'wordcount'
    ],
    toolbar: [
      'undo redo | formatselect | bold italic underline strikethrough | forecolor backcolor | removeformat | ' +
      'alignleft aligncenter alignright alignjustify | ' +
      'bullist numlist outdent indent | ' +
      'link unlink | image media | code | fullscreen | help'
    ],
    toolbar_mode: 'wrap',
    branding: false,
    promotion: false,
    // 关键配置：允许所有 HTML 标签和属性
    valid_elements: '*[*]',
    extended_valid_elements: '@[id|class|style|data-*]',
    // 图片上传配置
    images_upload_handler: (blobInfo, success, failure) => {
      const formData = new FormData()
      formData.append('file', blobInfo.blob(), blobInfo.filename())
      upload(formData).then((resp) => {
        if (!resp.success || resp.data.length === 0) {
          failure('上传失败')
          return
        }
        success(resp.data[0])
      }).catch(() => {
        failure('上传失败')
      })
    },
    // 自动调整高度
    autoresize_min_height: 300,
    autoresize_bottom_margin: 20,
    // 占位符
    placeholder: t('content') + ' ......',
    // 深色主题
    skin: store.dark ? 'oxide-dark' : 'oxide',
    content_style: store.dark ? 'body { background-color: #282c34; color: #9db1c5; }' : 'body { background-color: #ffffff; }'
  }

  // 监听文章内容变化，同步到编辑器
  watch(() => record.value.content, (newContent) => {
    if (newContent !== content.value) {
      content.value = newContent
    }
  })

  // 监听编辑器内容变化，同步到 record
  watch(content, (newContent) => {
    if (newContent !== record.value.content) {
      record.value.content = newContent
    }
  })

  function modalClose() {
    visible.value = false
    if (codeRef.value) codeRef.value.setContent()
  }

  // 暴露方法供外部调用
  function getContent() {
    return content.value
  }

  defineExpose({
    getContent
  })
</script>

<style scoped>
.tinymce-editor {
  border: 1px solid var(--color-fill-2);
}

.tinymce-editor.dark {
  border: none;
  border-radius: 3px;
  border-bottom: 1px solid #282c34;
}
</style>

<style>
/* TinyMCE 深色主题 */
.tinymce-editor.dark .tox-editor-header {
  background-color: #282c34 !important;
  border-color: #2d3239 !important;
}
.tinymce-editor.dark .tox-toolbar,
.tinymce-editor.dark .tox-toolbar__overflow,
.tinymce-editor.dark .tox-toolbar__primary {
  background-color: #282c34 !important;
}
.tinymce-editor.dark .tox-tbtn {
  color: #b6c5d4;
}
.tinymce-editor.dark .tox-tbtn--enabled:hover {
  background-color: #21252b !important;
  color: #fff;
}
.tinymce-editor.dark .tox-tbtn--enabled.tox-tbtn--active {
  background-color: #21252b !important;
  color: #fff;
}
.tinymce-editor.dark .tox-edit-area__iframe {
  background-color: #282c34 !important;
}
.tinymce-editor.dark .tox-statusbar {
  background-color: #282c34 !important;
  border-color: #2d3239 !important;
}
.tinymce-editor.dark .tox-statusbar__path-item {
  color: #757c83;
}
.tinymce-editor.dark .tox-statusbar__wordcount {
  color: #757c83;
}
.tinymce-editor.dark .tox-split-button {
  background-color: #282c34 !important;
}
</style>