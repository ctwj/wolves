<template>
  <a-form-item :label="$t('enable')">
    <a-space>
      <a-switch v-model="data.enable_on_create" type="round">
        <template #checked>{{ $t('onCreate') }}</template>
        <template #unchecked>{{ $t('onCreate') }}</template>
      </a-switch>
      <a-switch v-model="data.enable_on_update" type="round">
        <template #checked>{{ $t('onUpdate') }}</template>
        <template #unchecked>{{ $t('onUpdate') }}</template>
      </a-switch>
    </a-space>
  </a-form-item>

  <a-divider class="w-full" style="margin-top:0" />

  <a-form-item label="allow relative urls">
    <a-switch v-model="data.allow_relative_urls" type="round" />
  </a-form-item>

  <a-form-item label="links nofollow" help="require no follow on links">
    <a-switch v-model="data.require_no_follow_on_links" type="round" />
  </a-form-item>

  <a-form-item label="remove links" help="remove <a></a> tag">
    <a-switch v-model="data.remove_links" type="round" />
  </a-form-item>

  <a-form-item v-if="data.remove_links" label="hold max length" help="remove links hold value max length">
    <a-input-number style="width: 100px" v-model="data.remove_links_hold_length" :min="0" />
  </a-form-item>

  <a-divider class="w-full" />

  <a-form-item label="文本替换" help="将文章中的指定文本替换成目标文本">
    <a-switch v-model="data.enable_text_replace" type="round" />
  </a-form-item>

  <template v-if="data.enable_text_replace">
    <a-form-item label="替换规则">
      <a-space direction="vertical" style="width: 100%">
        <div v-for="(item, index) in data.text_replacements" :key="index" class="replacement-item">
          <a-card size="small" :title="`规则 ${index + 1}`">
            <template #extra>
              <a-button type="text" danger size="small" @click="removeReplacement(index)">
                删除
              </a-button>
            </template>
            <a-form-item label="源文本" :label-col-flex="'80px'" :wrapper-col-flex="'1 1 0'">
              <a-input v-model="item.source" placeholder="要替换的文本" />
            </a-form-item>
            <a-form-item label="目标文本" :label-col-flex="'80px'" :wrapper-col-flex="'1 1 0'">
              <a-input v-model="item.target" placeholder="替换后的文本" />
            </a-form-item>
          </a-card>
        </div>
        <a-button type="dashed" block @click="addReplacement">
          <template #icon>
            <icon-plus />
          </template>
          添加替换规则
        </a-button>
      </a-space>
    </a-form-item>
  </template>

</template>


<script setup>
 import {inject} from "vue";
 import { IconPlus } from '@arco-design/web-vue/es/icon';
 
 const data = inject("options")

 // 添加替换规则
 const addReplacement = () => {
   if (!data.value.text_replacements) {
     data.value.text_replacements = []
   }
   data.value.text_replacements.push({
     source: '',
     target: ''
   })
 }

 // 删除替换规则
 const removeReplacement = (index) => {
   data.value.text_replacements.splice(index, 1)
 }
</script>

<style scoped>
.numberInput{
  width: 220px;
}
.replacement-item {
  margin-bottom: 12px;
}
.replacement-item:last-child {
  margin-bottom: 0;
}
</style>
