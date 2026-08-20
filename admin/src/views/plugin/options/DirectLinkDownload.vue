<template>
  <a-alert type="info" style="margin-bottom: 16px;">
    直链下载转存：从文章的 download_links 中提取直链下载文件，支持文件验证、压缩包处理和 API 上传。通过定时任务或手动触发执行。
  </a-alert>

  <a-divider class="w-full" style="margin-top:0" />

  <a-tabs type="rounded">
    <a-tab-pane key="base" title="基础">
      <a-form-item label="指定文章ID" help="填写后只处理该文章，留空则批量处理所有文章">
        <a-input-number v-model="data.article_id" class="numberInput" :min="0" placeholder="留空处理全部" />
      </a-form-item>

      <a-form-item label="允许的文件后缀" help="逗号分隔，如：.zip,.rar,.7z">
        <a-input v-model="data.allowed_extensions" class="w-full" placeholder=".zip,.rar,.7z" />
      </a-form-item>

      <a-form-item label="最大文件大小 (MB)">
        <a-input-number v-model="data.max_file_size_mb" class="numberInput" :min="0" :max="10240" />
      </a-form-item>

      <a-form-item label="允许的域名" help="逗号分隔，留空表示不限制">
        <a-input v-model="data.allowed_domains" class="w-full" placeholder="example.com,cdn.example.com" />
      </a-form-item>
    </a-tab-pane>

    <a-tab-pane key="download" title="下载">
      <a-form-item label="下载重试次数">
        <a-input-number v-model="data.down_retry" class="numberInput" :min="0" :max="10" />
      </a-form-item>

      <a-form-item label="下载超时 (秒)">
        <a-input-number v-model="data.down_timeout" class="numberInput" :min="10" :max="600" />
      </a-form-item>

      <a-form-item label="下载代理">
        <a-input v-model="data.down_proxy" class="w-full" placeholder="http://proxy.example.com:8080" />
      </a-form-item>

      <a-form-item label="下载 Referer 映射" help="每行一个，格式：domain=referer。支持子域名匹配，如 itmopcdn.com=https://www.itmop.com">
        <a-textarea v-model="data.down_referer" :auto-size="{minRows:3,maxRows:5}" placeholder="itmopcdn.com=https://www.itmop.com&#10;example.com=https://example.com" />
      </a-form-item>

      <a-divider />

      <a-form-item label="文件名替换规则" help="每行一个，格式：old=new。用于处理下载文件名，如去除品牌名">
        <a-textarea v-model="data.file_name_replace" :auto-size="{minRows:5,maxRows:8}" placeholder="旧品牌=新品牌&#10;[广告]=&#10;\\s+=_" />
      </a-form-item>
    </a-tab-pane>

    <a-tab-pane key="api" title="API上传">
      <a-form-item label="API上传地址">
        <a-input v-model="data.api_upload_url" class="w-full" placeholder="https://api.example.com/upload" />
      </a-form-item>

      <a-form-item label="API文件字段">
        <a-input v-model="data.api_file_field" class="w-full" placeholder="file" />
      </a-form-item>

      <a-form-item label="API Token">
        <a-input v-model="data.api_token" class="w-full" placeholder="your-token-here" />
      </a-form-item>

      <a-form-item label="API请求头" help="每行一个，格式：key: value">
        <a-textarea v-model="data.api_headers" :auto-size="{minRows:3,maxRows:5}" placeholder="Authorization: Bearer token" />
      </a-form-item>

      <a-form-item label="API附加表单" help="每行一个，格式：key=value">
        <a-textarea v-model="data.api_form_data" :auto-size="{minRows:3,maxRows:5}" placeholder="category=download" />
      </a-form-item>

      <a-form-item label="API返回URL路径" help="如：data.url 或 result.link">
        <a-input v-model="data.api_url_path" class="w-full" placeholder="data.url" />
      </a-form-item>

      <a-form-item label="API成功标识路径" help="如：code 或 status">
        <a-input v-model="data.api_success_path" class="w-full" placeholder="code" />
      </a-form-item>

      <a-form-item label="API成功标识值" help="如：0 或 success">
        <a-input v-model="data.api_success_value" class="w-full" placeholder="0" />
      </a-form-item>

      <a-form-item label="API超时 (秒)">
        <a-input-number v-model="data.api_timeout" class="numberInput" :min="10" :max="300" />
      </a-form-item>

      <a-form-item label="API上传代理">
        <a-input v-model="data.api_proxy" class="w-full" placeholder="http://proxy.example.com:8080" />
      </a-form-item>

      <a-form-item label="API频率限制 (次/分钟)">
        <a-input-number v-model="data.api_rate_limit_per_minute" class="numberInput" :min="1" :max="1000" />
      </a-form-item>

      <a-form-item label="最大队列大小">
        <a-input-number v-model="data.api_max_queue_size" class="numberInput" :min="10" :max="10000" />
      </a-form-item>
    </a-tab-pane>

    <a-tab-pane key="package" title="压缩包处理">
      <a-form-item label="启用重新打包">
        <a-switch type="round" v-model="data.re_package">
          <template #checked>启用</template>
          <template #unchecked>禁用</template>
        </a-switch>
      </a-form-item>

      <template v-if="data.re_package">
        <a-form-item label="要删除的文件" help="逗号分隔，支持模糊匹配。如：广告（删除所有包含'广告'的文件）、*.txt（删除所有txt文件）、广告.txt（精确匹配）">
          <a-input v-model="data.delete_files" class="w-full" placeholder="广告,推广,readme.txt" />
        </a-form-item>

        <a-form-item label="要添加的文件" help="逗号分隔，需使用绝对路径。如：/path/to/readme.txt">
          <a-input v-model="data.add_files" class="w-full" placeholder="/path/to/readme.txt,/path/to/license.txt" />
        </a-form-item>

        <a-form-item label="压缩包密码" help="留空则不加密，设置后使用 ZipCrypto 加密（兼容性好）">
          <a-input v-model="data.zip_password" class="w-full" placeholder="留空不加密" />
        </a-form-item>

        <a-form-item label="不加密的文件" help="逗号分隔，支持模糊匹配。如：README.txt（精确匹配）、说明（模糊匹配）、*.nfo（通配符）">
          <a-input v-model="data.no_encrypt_files" class="w-full" placeholder="README.txt,说明.txt,*.nfo" />
        </a-form-item>
      </template>
    </a-tab-pane>

  </a-tabs>
</template>

<script setup>
import {inject} from "vue";
const data = inject("options")
</script>

<style scoped>
.numberInput {
  width: 220px;
}
</style>