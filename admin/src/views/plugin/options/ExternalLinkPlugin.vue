<template>
  <a-form-item :label="$t('enable')">
    <a-switch type="round" v-model="data.enable" />
  </a-form-item>

  <a-tabs type="line" default-active-key="basic" :style="{ height: '500px' }">
    <a-tab-pane key="basic" title="基础配置">
      <a-space direction="vertical" :size="16" fill>
        <a-form-item label="域名">
          <a-input v-model="data.domain" placeholder="example.com" />
          <template #extra>
            <a-text type="secondary">目标域名，会替换 URL 模板中的 ****</a-text>
          </template>
        </a-form-item>

        <a-form-item label="请求超时">
          <a-input-number v-model="data.timeout" :min="1" :max="300" :style="{ width: '200px' }" />
          <template #extra>
            <a-text type="secondary">单个请求的超时时间（秒）</a-text>
          </template>
        </a-form-item>

        <a-form-item label="请求间隔">
          <a-input-number v-model="data.delay" :min="0" :max="10000" :step="100" :style="{ width: '200px' }" />
          <template #extra>
            <a-text type="secondary">每个请求之间的延迟时间（毫秒）</a-text>
          </template>
        </a-form-item>
      </a-space>
    </a-tab-pane>

    <a-tab-pane key="urls" title="URL 模板">
      <a-space direction="vertical" :size="16" fill>
        <a-form-item label="URL 模板">
          <a-textarea
            v-model="data.url_templates"
            placeholder="每行一个 URL，**** 会被替换为域名&#10;例如：&#10;https://example1.com/search?q=****&#10;https://example2.com/api?url=****"
            :auto-size="{ minRows: 20, maxRows: 50 }"
            :max-length="1000000"
            allow-clear
            show-word-limit
          />
          <template #extra>
            <a-space direction="vertical" :size="4">
              <a-text type="secondary">每行一个 URL，**** 会被替换为配置的域名</a-text>
              <a-text type="secondary">示例：https://example.com/search?q=****</a-text>
              <a-text type="secondary">支持大量 URL 模板（最多 100 万字符）</a-text>
            </a-space>
          </template>
        </a-form-item>
      </a-space>
    </a-tab-pane>

    <a-tab-pane key="browser" title="浏览器模拟">
      <a-space direction="vertical" :size="16" fill>
        <a-form-item label="User-Agent">
          <a-textarea
            v-model="data.user_agent"
            placeholder="Mozilla/5.0 ..."
            :auto-size="{ minRows: 3, maxRows: 6 }"
          />
          <template #extra>
            <a-text type="secondary">模拟浏览器的 User-Agent，留空使用默认值</a-text>
          </template>
        </a-form-item>

        <a-form-item label="Referer">
          <a-input v-model="data.referer" placeholder="留空则自动生成" />
          <template #extra>
            <a-space direction="vertical" :size="4">
              <a-text type="secondary">请求来源页面</a-text>
              <a-text type="secondary">留空则根据目标 URL 自动生成（推荐）</a-text>
            </a-space>
          </template>
        </a-form-item>

        <a-form-item label="Cookies">
          <a-textarea
            v-model="data.cookies"
            placeholder="name1=value1; name2=value2"
            :auto-size="{ minRows: 2, maxRows: 4 }"
          />
          <template #extra>
            <a-text type="secondary">格式：name1=value1; name2=value2</a-text>
          </template>
        </a-form-item>
      </a-space>
    </a-tab-pane>
  </a-tabs>
</template>

<script setup>
import { inject } from "vue";

const data = inject("options");
</script>