<template>
  <a-layout class="h-full">
    <a-layout>
      <a-layout-content class="h-full" style="padding:0 12px 0 8px">
        <PostLeft ref="postLeftRef" />
      </a-layout-content>
    </a-layout>
    <PostRight />
  </a-layout>
</template>

<script setup>
  import PostRight from "@/views/article/PostRight.vue";
  import PostLeft from "@/views/article/PostLeft.vue";
  import {inject, ref} from 'vue'

  const record = inject('record')
  const postLeftRef = ref(null)

  // 暴露方法供父组件获取内容
  function getContent() {
    return postLeftRef.value?.getContent() || ''
  }

  defineExpose({
    getContent
  })
</script>