<template>
  <a-form-item :label="$t('enable')">
    <a-switch type="round" v-model="data.enable" />
  </a-form-item>

  <a-tabs type="line" default-active-key="functions">
    <a-tab-pane key="functions" title="功能开关">
      <!-- 整合模式 - 强制启用 -->
      <div class="mb-3 p-3 bg-blue-50 rounded-lg border border-blue-100">
        <div class="flex items-center justify-between">
          <div class="flex items-center gap-2">
            <a-tag color="blue">必选</a-tag>
            <span class="text-sm font-medium text-blue-700">整合模式</span>
            <a-tooltip content="自动启用，将所有功能合并为一个 AI 请求，大幅减少 API 调用次数">
              <icon-question-circle class="text-blue-400 text-xs" />
            </a-tooltip>
          </div>
          <a-tag color="green">已启用</a-tag>
        </div>
        <div class="text-xs text-blue-500 mt-1">
          已启用：一次请求处理所有功能，节省 Token
        </div>
      </div>

      <!-- 百度搜索意图分析 - 强制启用 -->
      <div class="mb-3 p-3 bg-green-50 rounded-lg border border-green-100">
        <div class="flex items-center justify-between">
          <div class="flex items-center gap-2">
            <a-tag color="blue">必选</a-tag>
            <span class="text-sm font-medium text-green-700">百度搜索意图分析</span>
            <a-tooltip content="调用百度推荐词 API 分析用户搜索意图，优化文章内容更符合用户需求">
              <icon-question-circle class="text-green-400 text-xs" />
            </a-tooltip>
          </div>
          <a-tag color="green">已启用</a-tag>
        </div>
        <div class="text-xs text-green-500 mt-1">
          自动分析用户搜索意图，优化标题、描述和关键词
        </div>
      </div>

      <!-- 核心优化 - 紧凑网格布局 -->
      <div class="mb-3">
        <div class="text-xs text-gray-500 mb-2">核心优化（推荐全开）</div>
        <div class="grid grid-cols-2 gap-x-6 gap-y-1">
          <div class="flex items-center justify-between py-1">
            <span class="text-sm">标题优化</span>
            <a-switch type="round" v-model="data.enable_title_optimize" size="small" />
          </div>
          <div class="flex items-center justify-between py-1">
            <span class="text-sm">关键词提取</span>
            <a-switch type="round" v-model="data.enable_keywords" size="small" />
          </div>
          <div class="flex items-center justify-between py-1">
            <span class="text-sm">描述优化</span>
            <a-switch type="round" v-model="data.enable_description" size="small" />
          </div>
          <div class="flex items-center justify-between py-1">
            <span class="text-sm">标签生成</span>
            <a-switch type="round" v-model="data.enable_tags" size="small" />
          </div>
        </div>
      </div>

      <!-- 高级优化 -->
      <div class="mb-3">
        <div class="text-xs text-gray-500 mb-2">高级优化（按需开启）</div>
        <div class="grid grid-cols-2 gap-x-6 gap-y-1">
          <div class="flex items-center justify-between py-1">
            <span class="text-sm">内容改写 <a-tooltip content="深度优化内容，消耗较多 Token"><icon-question-circle class="text-gray-400 text-xs" /></a-tooltip></span>
            <a-switch type="round" v-model="data.enable_rewrite" size="small" />
          </div>
          <div class="flex items-center justify-between py-1">
            <span class="text-sm">分类推荐 <a-tooltip content="仅对未分类文章生效"><icon-question-circle class="text-gray-400 text-xs" /></a-tooltip></span>
            <a-switch type="round" v-model="data.enable_category_recommend" size="small" />
          </div>
        </div>
      </div>

      <!-- 其他设置 -->
      <div class="flex items-center gap-6 pt-2 border-t border-gray-100">
        <div class="flex items-center gap-2">
          <span class="text-sm">自动发布</span>
          <a-switch type="round" v-model="data.auto_publish" size="small" />
        </div>
      </div>
    </a-tab-pane>

    <a-tab-pane key="api" title="API 配置">
      <!-- API 配置列表 -->
      <div class="space-y-3">
        <!-- 配置列表 -->
        <div v-for="(cfg, index) in apiConfigs" :key="cfg.id" 
             class="border rounded-lg p-3 hover:border-blue-300 transition-colors">
          <div class="flex items-center justify-between">
            <div class="flex items-center gap-3">
              <a-switch type="round" v-model="cfg.enable" size="small" />
              <span class="font-medium text-sm">{{ cfg.name || '未命名配置' }}</span>
              <a-tag size="small" :color="getAITypeColor(cfg.ai_type)">{{ getAITypeLabel(cfg.ai_type) }}</a-tag>
              <span v-if="!cfg.enable" class="text-xs text-gray-400">已禁用</span>
            </div>
            <div class="flex items-center gap-2">
              <a-button type="text" size="small" @click="editConfig(index)">
                <template #icon><icon-edit /></template>
              </a-button>
              <a-popconfirm content="确定删除此配置？" @ok="deleteConfig(index)">
                <a-button type="text" size="small" status="danger">
                  <template #icon><icon-delete /></template>
                </a-button>
              </a-popconfirm>
            </div>
          </div>
          <div class="mt-2 text-xs text-gray-500">
            {{ cfg.api_url }} | {{ cfg.model }} | 延迟: {{ cfg.request_delay || 1000 }}ms
          </div>
        </div>

        <!-- 添加按钮 -->
        <a-button type="dashed" long @click="addConfig">
          <template #icon><icon-plus /></template>
          添加 API 配置
        </a-button>

        <!-- 提示信息 -->
        <div class="text-xs text-gray-400 mt-2">
          <icon-info-circle class="mr-1" />
          多个 API 配置将并行使用，提升处理速度
        </div>
      </div>
    </a-tab-pane>

    <a-tab-pane key="params" title="参数配置">
      <div class="grid grid-cols-3 gap-x-6 gap-y-3">
        <a-form-item label="最少关键词">
          <a-input-number v-model="data.min_keywords" :min="1" :max="20" />
        </a-form-item>

        <a-form-item label="最多关键词">
          <a-input-number v-model="data.max_keywords" :min="1" :max="20" />
        </a-form-item>

        <a-form-item label="最少长尾词">
          <a-input-number v-model="data.min_long_tail" :min="0" :max="10" />
        </a-form-item>

        <a-form-item label="最少标签">
          <a-input-number v-model="data.min_tags" :min="0" :max="10" />
        </a-form-item>

        <a-form-item label="最多标签">
          <a-input-number v-model="data.max_tags" :min="0" :max="10" />
        </a-form-item>

        <a-form-item label="其他分类">
          <a-input v-model="data.other_category_name" placeholder="其他软件" />
        </a-form-item>
      </div>
    </a-tab-pane>

    <a-tab-pane key="advanced" title="高级配置">
      <div class="grid grid-cols-2 gap-x-6 gap-y-3">
        <a-form-item label="批量数量">
          <a-input-number v-model="data.batch_size" :min="1" :max="100" />
        </a-form-item>

        <a-form-item label="指定文章ID">
          <a-input-number v-model="data.article_id" :min="0" placeholder="0=批量" />
        </a-form-item>
      </div>

      <div class="flex items-center gap-6 pt-2">
        <div class="flex items-center gap-2">
          <span class="text-sm">强制重新生成</span>
          <a-switch type="round" v-model="data.force_regenerate" size="small" />
        </div>
        <div class="flex items-center gap-2">
          <span class="text-sm">跳过已发布</span>
          <a-switch type="round" v-model="data.skip_published" size="small" />
        </div>
      </div>
    </a-tab-pane>
  </a-tabs>

  <!-- 编辑配置弹窗 -->
  <a-modal v-model:visible="modalVisible" :title="isEditing ? '编辑 API 配置' : '添加 API 配置'" 
           @ok="saveConfig" @cancel="modalVisible = false" :width="500">
    <a-form :model="editingConfig" layout="vertical">
      <a-form-item label="配置名称" required>
        <a-input v-model="editingConfig.name" placeholder="如：OpenAI 主力" />
      </a-form-item>

      <a-form-item label="AI 类型" required>
        <a-select v-model="editingConfig.ai_type" @change="onAITypeChangeInModal">
          <a-option value="openai">OpenAI 兼容</a-option>
          <a-option value="nvidia">NVIDIA NIM</a-option>
          <a-option value="zhipu">智普 GLM</a-option>
        </a-select>
      </a-form-item>

      <a-form-item label="API URL" required>
        <a-input v-model="editingConfig.api_url" placeholder="https://api.openai.com/v1" />
      </a-form-item>

      <a-form-item label="API Key" required>
        <a-input-password v-model="editingConfig.api_key" placeholder="sk-..." />
      </a-form-item>

      <a-form-item label="Model" required>
        <a-input v-model="editingConfig.model" placeholder="gpt-3.5-turbo" />
      </a-form-item>

      <a-form-item label="请求延迟(ms)" help="每次请求后的等待时间，避免 API 限流">
        <a-input-number v-model="editingConfig.request_delay" :min="0" :max="10000" :step="100" placeholder="1000" />
      </a-form-item>

      <a-form-item label="启用">
        <a-switch type="round" v-model="editingConfig.enable" />
      </a-form-item>
    </a-form>
  </a-modal>
</template>

<script setup>
import { inject, ref, computed, watch, onMounted } from "vue";
const data = inject("options");

// API 配置列表
const apiConfigs = computed({
  get: () => data.value.api_configs || [],
  set: (val) => { data.value.api_configs = val; }
});

// 弹窗相关
const modalVisible = ref(false);
const isEditing = ref(false);
const editingIndex = ref(-1);
const editingConfig = ref({
  id: '',
  name: '',
  ai_type: 'openai',
  api_url: '',
  api_key: '',
  model: '',
  request_delay: 1000, // 默认 1000ms
  enable: true
});

// AI 类型配置
const aiTypeConfig = {
  openai: {
    url: "https://api.openai.com/v1",
    model: "gpt-3.5-turbo",
    request_delay: 500 // OpenAI 限流较宽松
  },
  nvidia: {
    url: "https://integrate.api.nvidia.com/v1",
    model: "abacusai/dracarys-llama-3.1-70b-instruct",
    request_delay: 2000 // NVIDIA 限流严格，建议 2s 以上
  },
  zhipu: {
    url: "https://open.bigmodel.cn/api/paas/v4",
    model: "glm-4.7-flash",
    request_delay: 1000
  },
};

// AI 类型标签颜色
function getAITypeColor(type) {
  const colors = {
    openai: 'green',
    nvidia: 'orangered',
    zhipu: 'blue'
  };
  return colors[type] || 'gray';
}

// AI 类型标签文字
function getAITypeLabel(type) {
  const labels = {
    openai: 'OpenAI',
    nvidia: 'NVIDIA',
    zhipu: '智普'
  };
  return labels[type] || type;
}

// 弹窗中 AI 类型变更
function onAITypeChangeInModal(value) {
  if (aiTypeConfig[value]) {
    editingConfig.value.api_url = aiTypeConfig[value].url;
    editingConfig.value.model = aiTypeConfig[value].model;
    editingConfig.value.request_delay = aiTypeConfig[value].request_delay;
  }
}

// 生成唯一 ID
function generateId() {
  return 'cfg_' + Date.now() + '_' + Math.random().toString(36).substr(2, 9);
}

// 添加配置
function addConfig() {
  isEditing.value = false;
  editingIndex.value = -1;
  editingConfig.value = {
    id: generateId(),
    name: '',
    ai_type: 'openai',
    api_url: aiTypeConfig.openai.url,
    api_key: '',
    model: aiTypeConfig.openai.model,
    request_delay: aiTypeConfig.openai.request_delay,
    enable: true
  };
  modalVisible.value = true;
}

// 编辑配置
function editConfig(index) {
  isEditing.value = true;
  editingIndex.value = index;
  editingConfig.value = { ...apiConfigs.value[index] };
  modalVisible.value = true;
}

// 保存配置
function saveConfig() {
  if (!editingConfig.value.name) {
    editingConfig.value.name = editingConfig.value.ai_type.toUpperCase() + ' 配置';
  }
  
  if (isEditing.value) {
    // 编辑模式：更新现有配置
    const configs = [...apiConfigs.value];
    configs[editingIndex.value] = { ...editingConfig.value };
    apiConfigs.value = configs;
  } else {
    // 添加模式：追加新配置
    apiConfigs.value = [...apiConfigs.value, { ...editingConfig.value }];
  }
  
  modalVisible.value = false;
}

// 删除配置
function deleteConfig(index) {
  const configs = [...apiConfigs.value];
  configs.splice(index, 1);
  apiConfigs.value = configs;
}

// 初始化：迁移旧配置
onMounted(() => {
  // 如果 api_configs 不存在或为空，检查是否有旧配置需要迁移
  if (!data.value.api_configs || data.value.api_configs.length === 0) {
    // 检查旧字段是否存在
    if (data.value.api_url && data.value.api_key) {
      const aiType = data.value.ai_type || 'openai';
      data.value.api_configs = [{
        id: generateId(),
        name: '默认配置',
        ai_type: aiType,
        api_url: data.value.api_url,
        api_key: data.value.api_key,
        model: data.value.model || 'gpt-3.5-turbo',
        request_delay: aiTypeConfig[aiType]?.request_delay || 1000,
        enable: true
      }];
      // 清空旧字段
      data.value.ai_type = '';
      data.value.api_url = '';
      data.value.api_key = '';
      data.value.model = '';
    } else {
      // 初始化为空数组
      data.value.api_configs = [];
    }
  }
});
</script>