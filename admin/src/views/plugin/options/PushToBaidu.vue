<template>
  <a-form-item :label="$t('enable_on_create')">
    <a-switch type="round" v-model="data.enable_on_create" />
  </a-form-item>

  <a-form-item :label="$t('enable_on_update')">
    <a-switch type="round" v-model="data.enable_on_update" />
  </a-form-item>

  <a-form-item :label="$t('api_url')">
    <a-input v-model="data.api_url" :placeholder="$t('api_url_placeholder')" />
  </a-form-item>

  <a-divider>{{ $t('cron_settings') }}</a-divider>

  <a-form-item :label="$t('cron_within_hours')">
    <a-input-number v-model="data.cron_within_hours" :min="1" :max="24" />
  </a-form-item>
</template>

<script setup>
  import { inject } from 'vue'

  const data = inject('options')
</script>