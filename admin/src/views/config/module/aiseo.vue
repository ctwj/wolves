<template>
  <div class="aiseo-config">
    <a-alert type="info" class="mb-6">
      <template #title>
        <span class="flex items-center">
          <icon-robot class="mr-2" />
          AI SEO 功能说明
        </span>
      </template>
      <div class="text-sm mt-2">
        开启后，网站将生成以下文件供 AI 和内容聚合器使用：
      </div>
      <ul class="list-disc list-inside mt-3 space-y-1 text-sm text-gray-600">
        <li><code class="bg-gray-100 px-1 rounded">/rss.xml</code> - RSS 2.0 订阅源</li>
        <li><code class="bg-gray-100 px-1 rounded">/atom.xml</code> - Atom 1.0 订阅源</li>
        <li><code class="bg-gray-100 px-1 rounded">/llms.txt</code> - AI 爬虫专用内容概要</li>
        <li><code class="bg-gray-100 px-1 rounded">/api.json</code> - 结构化 JSON API</li>
      </ul>
    </a-alert>

    <a-divider />

    <div class="config-items space-y-6">
      <!-- RSS 订阅 -->
      <div class="config-item">
        <div class="flex items-center justify-between">
          <div class="flex items-center">
            <div class="config-icon bg-orange-100">
              <icon-rss class="text-orange-500" />
            </div>
            <div>
              <div class="font-medium">{{ $t('rssEnable') }}</div>
              <div class="text-xs text-gray-500">{{ $t('rssEnableTip') }}</div>
            </div>
          </div>
          <a-switch type="round" v-model="data.rss_enable" />
        </div>
        <div v-if="data.rss_enable" class="mt-3 ml-12">
          <a-input-number v-model="data.rss_limit" :min="1" :max="500" placeholder="文章数量" />
          <span class="ml-2 text-sm text-gray-500">篇文章</span>
        </div>
      </div>

      <!-- llms.txt -->
      <div class="config-item">
        <div class="flex items-center justify-between">
          <div class="flex items-center">
            <div class="config-icon bg-purple-100">
              <icon-file class="text-purple-500" />
            </div>
            <div>
              <div class="font-medium">{{ $t('llmsEnable') }}</div>
              <div class="text-xs text-gray-500">{{ $t('llmsEnableTip') }}</div>
            </div>
          </div>
          <a-switch type="round" v-model="data.llms_enable" />
        </div>
        <div v-if="data.llms_enable" class="mt-3 ml-12">
          <a-input-number v-model="data.llms_limit" :min="1" :max="100" placeholder="文章数量" />
          <span class="ml-2 text-sm text-gray-500">篇文章</span>
        </div>
      </div>

      <!-- API 端点 -->
      <div class="config-item">
        <div class="flex items-center justify-between">
          <div class="flex items-center">
            <div class="config-icon bg-blue-100">
              <icon-api class="text-blue-500" />
            </div>
            <div>
              <div class="font-medium">{{ $t('apiEnable') }}</div>
              <div class="text-xs text-gray-500">{{ $t('apiEnableTip') }}</div>
            </div>
          </div>
          <a-switch type="round" v-model="data.api_enable" />
        </div>
        <div v-if="data.api_enable" class="mt-3 ml-12">
          <a-input-number v-model="data.api_limit" :min="1" :max="100" placeholder="文章数量" />
          <span class="ml-2 text-sm text-gray-500">篇文章</span>
        </div>
      </div>
    </div>

    <a-divider />

    <a-alert type="warning" class="mt-4">
      <template #title>提示</template>
      <div class="text-sm">
        启用这些功能后，搜索引擎和 AI 可以更容易地读取您的网站内容。
        如不需要，可以随时关闭。
      </div>
    </a-alert>
  </div>
</template>

<script setup>
  import {inject} from 'vue'

  const data = inject('data')
</script>

<style scoped>
.aiseo-config {
  max-width: 700px;
}

.config-item {
  padding: 16px;
  background: linear-gradient(135deg, #f8f9fa 0%, #ffffff 100%);
  border-radius: 12px;
  border: 1px solid #e5e7eb;
  transition: all 0.3s ease;
}

.config-item:hover {
  border-color: #c0c4cc;
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.06);
}

.config-icon {
  width: 40px;
  height: 40px;
  border-radius: 10px;
  display: flex;
  align-items: center;
  justify-content: center;
  margin-right: 12px;
  font-size: 18px;
}

:deep(.arco-divider) {
  margin: 24px 0;
}
</style>