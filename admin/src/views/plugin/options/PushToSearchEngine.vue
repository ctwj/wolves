<template>
  <a-tabs type="rounded">
    <!-- 百度推送配置 -->
    <a-tab-pane key="baidu" :title="$t('PushToBaidu')">
      <a-form-item :label="$t('baidu_enable')">
        <a-switch type="round" v-model="data.baidu_enable" />
      </a-form-item>

      <a-form-item :label="$t('enable_on_create')">
        <a-switch type="round" v-model="data.baidu_enable_on_create" />
      </a-form-item>

      <a-form-item :label="$t('enable_on_update')">
        <a-switch type="round" v-model="data.baidu_enable_on_update" />
      </a-form-item>

      <a-form-item :label="$t('api_url')">
        <a-input v-model="data.baidu_api_url" :placeholder="$t('baidu_api_url_placeholder')" />
      </a-form-item>

      <a-form-item :label="$t('cron_within_hours')">
        <a-input-number v-model="data.baidu_cron_within_hours" :min="1" :max="24" />
      </a-form-item>
    </a-tab-pane>

    <!-- Bing 推送配置 -->
    <a-tab-pane key="bing" :title="$t('PushToBing')">
      <a-form-item :label="$t('bing_enable')">
        <a-switch type="round" v-model="data.bing_enable" />
      </a-form-item>

      <a-form-item :label="$t('enable_on_create')">
        <a-switch type="round" v-model="data.bing_enable_on_create" />
      </a-form-item>

      <a-form-item :label="$t('enable_on_update')">
        <a-switch type="round" v-model="data.bing_enable_on_update" />
      </a-form-item>

      <a-form-item :label="$t('api_key')">
        <a-input v-model="data.bing_api_key" :placeholder="$t('bing_api_key_placeholder')" />
      </a-form-item>

      <a-form-item :label="$t('cron_within_hours')">
        <a-input-number v-model="data.bing_cron_within_hours" :min="1" :max="24" />
      </a-form-item>
    </a-tab-pane>
  </a-tabs>
</template>

<script setup>
import { inject } from 'vue'

const data = inject('options')
</script>