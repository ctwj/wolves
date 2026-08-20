<template>
  <a-form-item label="启用">
    <a-space>
      <a-switch v-model="data.enable_on_create" type="round">
        <template #checked>创建时</template>
        <template #unchecked>创建时</template>
      </a-switch>
      <a-switch v-model="data.enable_on_update" type="round">
        <template #checked>更新时</template>
        <template #unchecked>更新时</template>
      </a-switch>
    </a-space>
  </a-form-item>

  <a-form-item label="保存方式">
    <a-select v-model="data.upload_target" class="w-full">
      <a-option value="local">本地</a-option>
      <a-option value="api">API图床</a-option>
    </a-select>
  </a-form-item>

  <a-form-item label="启用水印">
    <a-space>
      <a-switch type="round" v-model="data.watermark_enable">
        <template #checked>启用</template>
        <template #unchecked>禁用</template>
      </a-switch>
      <a-button v-if="data.watermark_enable" size="small" type="outline" @click="previewWatermark" :loading="loadingPreview">
        <template #icon><icon-eye /></template>
        预览水印
      </a-button>
    </a-space>
  </a-form-item>

  <!-- 水印预览模态框 -->
  <a-modal v-model:visible="visiblePreview" title="水印预览" :width="500" :footer="false" unmount-on-close>
    <div class="text-center">
      <a-spin :loading="loadingPreview" tip="生成预览中...">
        <img v-if="previewUrl" :src="previewUrl" alt="水印预览" class="max-w-full rounded shadow" />
      </a-spin>
      <a-alert v-if="previewError" type="error" class="mt-4">{{ previewError }}</a-alert>
    </div>
  </a-modal>

  <template v-if="data.enable_on_create || data.enable_on_update">

  <a-divider class="w-full" style="margin-top:0" />

  <a-tabs type="rounded">
    <a-tab-pane key="base" title="基础">
      <a-form-item label="最大宽度">
        <a-input-number v-model="data.max_width" class="numberInput" :min="0" />
      </a-form-item>
      <a-form-item label="最大高度">
        <a-input-number v-model="data.max_height" class="numberInput" :min="0" />
      </a-form-item>
      <a-form-item label="缩略图宽度">
        <a-input-number v-model="data.thumb_width" class="numberInput" :min="0" />
      </a-form-item>
      <a-form-item label="缩略图高度">
        <a-input-number v-model="data.thumb_height" class="numberInput" :min="0" />
      </a-form-item>
      <a-form-item label="缩略图最小宽度">
        <a-input-number v-model="data.thumb_min_width" class="numberInput" :min="0" />
      </a-form-item>
      <a-form-item label="缩略图最小高度">
        <a-input-number v-model="data.thumb_min_height" class="numberInput" :min="0" />
      </a-form-item>
    </a-tab-pane>

    <a-tab-pane key="more" title="高级">
      <a-form-item label="下载重试次数">
        <a-input-number v-model="data.down_retry" class="numberInput" :min="0" :max="10" />
      </a-form-item>
      <a-form-item label="始终压缩尺寸">
        <a-switch type="round" v-model="data.always_resize"/>
      </a-form-item>
      <a-form-item label="缩略图焦点裁剪">
        <a-switch type="round" v-model="data.thumb_extract_focus"/>
      </a-form-item>
      <a-form-item label="下载失败时移除">
        <a-switch type="round" v-model="data.remove_if_down_fail"/>
      </a-form-item>

      <a-form-item label="下载代理">
        <a-input v-model="data.down_proxy" class="w-full" />
      </a-form-item>

      <a-form-item label="下载 Referer">
        <a-textarea v-model="data.down_referer" :auto-size="{minRows:4,maxRows:6}"/>
      </a-form-item>
    </a-tab-pane>

    <a-tab-pane key="api" title="图床API">
      <template v-if="data.upload_target === 'api'">
        <a-form-item label="上传接口地址">
          <a-input v-model="data.api_upload_url" class="w-full" placeholder="https://api.example.com/upload" />
        </a-form-item>

        <a-form-item label="文件字段名">
          <a-input v-model="data.api_file_field" class="w-full" placeholder="file" />
        </a-form-item>

        <a-form-item label="请求超时(秒)">
          <a-input-number v-model="data.api_timeout" class="numberInput" :min="5" :max="300" />
        </a-form-item>

        <a-form-item label="上传代理">
          <a-input v-model="data.api_proxy" class="w-full" placeholder="http://127.0.0.1:7890" />
        </a-form-item>

        <a-form-item label="图床域名">
          <a-input v-model="data.api_image_domain" class="w-full" placeholder="https://img.example.com/" />
        </a-form-item>

        <a-form-item label="返回图片URL路径">
          <a-input v-model="data.api_url_path" class="w-full" placeholder="data.url" />
        </a-form-item>

        <a-form-item label="成功标识路径">
          <a-input v-model="data.api_success_path" class="w-full" placeholder="success" />
        </a-form-item>

        <a-form-item label="成功标识值">
          <a-input v-model="data.api_success_value" class="w-full" placeholder="true" />
        </a-form-item>

        <a-form-item label="请求头" help="每行一条，格式：key: value">
          <a-textarea v-model="data.api_headers" :auto-size="{minRows:3,maxRows:6}" />
        </a-form-item>

        <a-form-item label="附加表单参数" help="每行一条，格式：key=value">
          <a-textarea v-model="data.api_form_data" :auto-size="{minRows:3,maxRows:6}" />
        </a-form-item>

        <a-divider>频率限制设置</a-divider>

        <a-form-item label="每分钟调用限制" help="API每分钟最多调用次数，超出将排队等待">
          <a-input-number v-model="data.api_rate_limit_per_minute" class="numberInput" :min="1" :max="1000" />
        </a-form-item>

        <a-form-item label="队列最大长度" help="上传任务队列的最大长度，超出将拒绝上传">
          <a-input-number v-model="data.api_max_queue_size" class="numberInput" :min="10" :max="10000" />
        </a-form-item>

        <a-form-item label="队列任务超时(秒)" help="队列中任务的最长等待时间">
          <a-input-number v-model="data.api_queue_timeout" class="numberInput" :min="30" :max="3600" />
        </a-form-item>
      </template>

      <a-alert v-else type="info">
        当前"保存方式"不是 API 图床，无需配置此分组。
      </a-alert>
    </a-tab-pane>

    <a-tab-pane key="watermark" title="水印设置">
      <template v-if="data.watermark_enable">
        <a-form-item label="水印类型">
          <a-select v-model="data.watermark_type" class="w-full">
            <a-option value="text">文字水印</a-option>
            <a-option value="image">图片水印</a-option>
          </a-select>
        </a-form-item>

        <!-- 文字水印配置 -->
        <template v-if="data.watermark_type === 'text'">
          <a-divider>文字水印配置</a-divider>
          <a-form-item label="水印文字">
            <a-input v-model="data.watermark_text" class="w-full" placeholder="请输入水印文字" />
          </a-form-item>
          <a-form-item label="字体大小(像素)">
            <a-input-number v-model="data.watermark_font_size" class="numberInput" :min="12" :max="100" />
          </a-form-item>
          <a-form-item label="字体颜色">
            <a-color-picker v-model="data.watermark_font_color" />
          </a-form-item>
          <a-form-item label="旋转角度(度)">
            <a-slider v-model="data.watermark_text_rotate" :min="-45" :max="45" show-input />
          </a-form-item>

          <a-divider>描边设置</a-divider>
          <a-form-item label="描边颜色" help="留空则不启用描边">
            <a-space>
              <a-color-picker v-model="data.watermark_stroke_color" :show-history="true" />
              <a-button v-if="data.watermark_stroke_color" size="small" @click="data.watermark_stroke_color = ''">清除</a-button>
            </a-space>
          </a-form-item>
          <a-form-item v-if="data.watermark_stroke_color" label="描边宽度(像素)">
            <a-input-number v-model="data.watermark_stroke_width" class="numberInput" :min="1" :max="10" />
          </a-form-item>

          <a-divider>背景设置</a-divider>
          <a-form-item label="背景颜色" help="留空则无背景，与渐变背景互斥">
            <a-space>
              <a-color-picker v-model="data.watermark_bg_color" :show-history="true" />
              <a-button v-if="data.watermark_bg_color" size="small" @click="data.watermark_bg_color = ''">清除</a-button>
            </a-space>
          </a-form-item>
          <a-form-item label="圆角半径(像素)">
            <a-input-number v-model="data.watermark_bg_radius" class="numberInput" :min="0" :max="50" />
          </a-form-item>
          <a-form-item label="内边距(像素)" help="留空使用字体大小的1/3">
            <a-input-number v-model="data.watermark_bg_padding" class="numberInput" :min="0" :max="50" />
          </a-form-item>

          <a-divider>渐变背景</a-divider>
          <a-form-item label="渐变起始颜色" help="设置后将覆盖纯色背景">
            <a-space>
              <a-color-picker v-model="data.watermark_bg_gradient_start" :show-history="true" />
              <a-button v-if="data.watermark_bg_gradient_start" size="small" @click="data.watermark_bg_gradient_start = ''">清除</a-button>
            </a-space>
          </a-form-item>
          <a-form-item v-if="data.watermark_bg_gradient_start" label="渐变结束颜色">
            <a-color-picker v-model="data.watermark_bg_gradient_end" :show-history="true" />
          </a-form-item>
          <a-form-item v-if="data.watermark_bg_gradient_start && data.watermark_bg_gradient_end" label="渐变角度">
            <a-slider v-model="data.watermark_bg_gradient_angle" :min="0" :max="360" show-input />
            <div class="text-xs text-gray-500 mt-1">0=从左到右，90=从上到下，180=从右到左，270=从下到上</div>
          </a-form-item>
        </template>

        <!-- 图片水印配置 -->
        <template v-if="data.watermark_type === 'image'">
          <a-divider>图片水印配置</a-divider>
          <a-form-item label="水印图片路径" help="支持本地文件路径或远程URL">
            <a-input v-model="data.watermark_image_path" class="w-full" placeholder="例如: watermark.png 或 https://example.com/watermark.png" />
          </a-form-item>
          <a-form-item label="缩放比例(%)">
            <a-slider v-model="data.watermark_image_scale" :min="5" :max="100" show-input />
          </a-form-item>
          <a-form-item label="旋转角度(度)">
            <a-slider v-model="data.watermark_image_rotate" :min="-45" :max="45" show-input />
          </a-form-item>
        </template>

        <!-- 通用配置 -->
        <a-divider>通用设置</a-divider>
        <a-form-item label="最小宽度限制(像素)" help="只有图片宽度大于等于此值才添加水印，0表示不限制">
          <a-input-number v-model="data.watermark_min_width" class="numberInput" :min="0" :max="5000" />
        </a-form-item>
        <a-form-item label="水印位置">
          <a-select v-model="data.watermark_position" class="w-full">
            <a-option value="top_left">左上</a-option>
            <a-option value="top_right">右上</a-option>
            <a-option value="bottom_left">左下</a-option>
            <a-option value="bottom_right">右下</a-option>
            <a-option value="center">中心</a-option>
            <a-option value="tile">平铺</a-option>
          </a-select>
        </a-form-item>
        <a-form-item label="透明度(%)">
          <a-slider v-model="data.watermark_opacity" :min="10" :max="100" show-input />
        </a-form-item>
        <a-form-item label="边距(像素)">
          <a-input-number v-model="data.watermark_margin" class="numberInput" :min="0" :max="100" />
        </a-form-item>
        <a-form-item v-if="data.watermark_position === 'tile'" label="平铺间距(像素)">
          <a-input-number v-model="data.watermark_tile_spacing" class="numberInput" :min="50" :max="500" />
        </a-form-item>
      </template>

      <a-alert v-else type="info">
        请先在上方开启"启用水印"功能。
      </a-alert>
    </a-tab-pane>
  </a-tabs>

  </template>

</template>


<script setup>
 import {inject, ref} from "vue";
 import {pluginPreviewWatermark, pluginSaveOptions} from "@/api/index.js";
 import {Message} from '@arco-design/web-vue';
 import {t} from '@/locale';

 const data = inject("options")
 const currentID = inject("currentID")

 // 预览相关
 const visiblePreview = ref(false)
 const previewUrl = ref('')
 const previewError = ref('')
 const loadingPreview = ref(false)

 async function previewWatermark() {
   loadingPreview.value = true
   previewError.value = ''
   previewUrl.value = ''

   try {
     // 先保存当前配置
     await pluginSaveOptions(currentID.value, data.value)
     Message.success(t('message.success', [t('save')]))

     // 再生成预览
     visiblePreview.value = true
     const blob = await pluginPreviewWatermark(currentID.value)
     previewUrl.value = URL.createObjectURL(blob)
   } catch (e) {
     previewError.value = e.message || '预览生成失败'
     visiblePreview.value = true
   } finally {
     loadingPreview.value = false
   }
 }
</script>

<style scoped>
.numberInput{
  width: 220px;
}
</style>