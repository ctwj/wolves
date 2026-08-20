<template>

    <a-form-item field="slug" :label="$t('slug')" :style="{marginBottom:record.slug ?'10px':''}"
                 :rules="[{required:!!record.id, message:$t('message.required',[$t('slug')])}]" hide-asterisk>
      <div class="w-full">
        <a-input class="input" v-model="record.slug" :max-length="150" allow-clear show-word-limit />
        <div v-if="record.slug" class="break-all text-gray-600" style="margin-top:10px;font-size:12px;">
          <div v-if="record.id > 0" @click="useOpenLink" class="cursor-pointer hover:underline underline-offset-4 hover:text-blue-500">{{ slugURL }}</div>
          <div v-else>{{ slugURL }}</div>
        </div>
      </div>
    </a-form-item>

    <a-form-item field="thumbnail" :label="$t('thumbnail')">
      <div class="w-full" >
        <UploadImgInput v-model="record.thumbnail" class="w-full" inputStyle="background-color: var(--color-bg-5);" />
        <a-card v-if="record.thumbnail" class="w-full mt-5" size="mini" :bordered="false">
          <template #title><span class="text-sm">{{$t('preview')}}</span></template>
          <template #extra>
            <icon-delete class="cursor-pointer" @click="record.thumbnail=''" />
          </template>
          <div class="text-center"><a-image :src="record.thumbnail" height="170" width="100%" referrerpolicy="no-referrer" /></div>
        </a-card>
      </div>
    </a-form-item>

    <a-form-item field="category_id" :label="$t('category')">
      <SelectCategory v-model="record.category_id" :cascader-style="{backgroundColor:'var(--color-bg-5)'}" />
    </a-form-item>

    <a-form-item field="content_type" label="内容类型" extra="决定文章详情页形态：视频=观看页（选集）、小说=阅读页（分章）、图片=图集浏览">
      <a-select v-model="record.content_type" placeholder="普通（默认）" allow-clear>
        <a-option value="">普通</a-option>
        <a-option value="novel">小说</a-option>
        <a-option value="image">图片</a-option>
        <a-option value="video">视频</a-option>
      </a-select>
    </a-form-item>

    <a-form-item v-if="record.content_type === 'video'" label="视频选集">
      <VideoSourcesEditor v-model="videoSources" class="w-full" />
    </a-form-item>

    <a-form-item v-if="record.content_type === 'image'" label="图集图片">
      <GalleryImagesEditor v-model="galleryImages" class="w-full" />
    </a-form-item>

    <a-form-item v-if="record.content_type === 'novel'" label="章节分隔">
      <div class="text-gray-500 text-xs leading-6">
        正文即小说全文；章节之间单独一行插入分隔符 <code class="px-1 py-0.5 rounded bg-gray-100">===chapter===</code>
        （编辑器中直接作为文本输入，前台自动按章分页阅读）。每章开头建议使用标题（如"第X章 …"）作为章节名。
      </div>
    </a-form-item>

    <a-form-item :label="$t('tag')">
      <Tag />
    </a-form-item>

    <a-form-item field="description" :label="$t('description')">
      <a-textarea class="input" v-model="record.description" :max-length="250" :auto-size="{minRows:3,maxRows:5}" show-word-limit />
    </a-form-item>

    <a-form-item field="keywords" :label="$t('keywords')">
      <a-textarea class="input" v-model="record.keywords" :max-length="250" :auto-size="{minRows:3,maxRows:5}" show-word-limit />
    </a-form-item>

    <a-form-item field="views" :label="$t('views')">
      <a-input-number class="input" v-model="record.views" :min="0" />
    </a-form-item>

    <a-form-item field="status" :label="$t('publishStatus')">
      <a-switch v-model="record.status" :checked-text="$t('published')" :unchecked-text="$t('unpublished')" />
    </a-form-item>

    <a-form-item field="create_time" :label="$t('createTime')">
      <a-date-picker class="w-full input" style="background-color: var(--color-bg-5);"  v-model="createTime" value-format="timestamp" show-time @change="(val)=>record.create_time =parseInt(val / 1000)" />
    </a-form-item>

    <a-collapse expand-icon-position="right" :default-active-key="['extends', 'res']">
      <a-collapse-item :header="$t('extends')" key="extends" style="background: transparent">
        <a-form :model="record" :label-col-props="{span: 8}" :wrapper-col-props="{span: 16}">
          <a-form-item v-for="(item,index) in record.extends" :label-col-style="{paddingRight:'10px'}">
            <template #label>
              <div class="flex">
                <a-input class="input input_extends" v-model="item.key" /><span class="ml-2">:</span>
              </div>
            </template>
            <!-- 如果 value 是布尔类型，显示为只读标签 -->
            <div v-if="typeof item.value === 'boolean'" class="w-full">
              <a-tag :color="item.value ? 'green' : 'red'">{{ item.value ? 'true' : 'false' }}</a-tag>
            </div>
            <!-- 如果 value 是对象类型，显示格式化的 JSON（只读） -->
            <div v-else-if="typeof item.value === 'object' && item.value !== null" class="w-full">
              <div class="relative">
                <pre class="json-display">{{ JSON.stringify(item.value, null, 2) }}</pre>
                <a-tag color="arcoblue" size="small" class="absolute top-0 right-0">只读</a-tag>
              </div>
            </div>
            <!-- 如果 value 是字符串类型，可编辑 -->
            <a-textarea v-else class="input input_extends" :auto-size="{minRows:1,maxRows:5}" v-model="item.value" />
              <a-button class="ml-1" type="text" @click="record.extends.splice(index,1)"><template #icon><icon-close-circle :stroke-width="3" /></template></a-button>
          </a-form-item>
        </a-form>
        <a-button size="mini" long @click="addExtends">
          <template #icon><icon-plus /></template>{{$t('add')}}
        </a-button>
      </a-collapse-item>

      <a-collapse-item :header="$t('res')" key="res" style="background: transparent">
        <a-form :model="record" :label-col-props="{span: 8}" :wrapper-col-props="{span: 16}">
          <a-form-item v-for="(item,index) in record.res" :label-col-style="{paddingRight:'10px'}">
            <template #label>
              <div class="flex">
                <a-input class="input input_extends" v-model="item.key" /><span class="ml-2">:</span>
              </div>
            </template>
            <!-- 如果 value 是对象类型，显示格式化的 JSON（只读） -->
            <div v-if="typeof item.value === 'object' && item.value !== null" class="w-full">
              <div class="relative">
                <pre class="json-display">{{ JSON.stringify(item.value, null, 2) }}</pre>
                <a-tag color="arcoblue" size="small" class="absolute top-0 right-0">只读</a-tag>
              </div>
            </div>
            <!-- 如果 value 是布尔类型，显示为只读文本 -->
            <div v-else-if="typeof item.value === 'boolean'" class="w-full">
              <a-tag :color="item.value ? 'green' : 'red'">{{ item.value ? 'true' : 'false' }}</a-tag>
            </div>
            <!-- 如果 value 是字符串类型，可编辑 -->
            <a-textarea v-else class="input input_extends" :auto-size="{minRows:1,maxRows:5}" v-model="item.value" />
            <a-button class="ml-1" type="text" @click="record.res.splice(index,1)"><template #icon><icon-close-circle :stroke-width="3" /></template></a-button>
          </a-form-item>
        </a-form>
        <a-button size="mini" long @click="addRes">
          <template #icon><icon-plus /></template>{{$t('add')}}
        </a-button>
      </a-collapse-item>
    </a-collapse>

</template>

<script setup>
  import {computed, inject} from "vue";
  import UploadImgInput from '@/components/utils/UploadImgInput.vue'
  import Tag from "./com/Tag.vue"
  import VideoSourcesEditor from "./com/VideoSourcesEditor.vue"
  import GalleryImagesEditor from "./com/GalleryImagesEditor.vue"
  import {useStore} from "@/store/index.js";
  import {useOpenLink} from '@/hooks/utils.js'
  import {useAppendSiteURL} from "@/hooks/app/index.js";
  import SelectCategory from "@/components/data/SelectCategory.vue"


  const record = inject('record')
  const createTime = computed(()=>record.value.create_time*1000)
  const store = useStore()

  // ===== 多媒体类型专属数据：读写 extends 键（切换类型不清除已录入数据） =====
  function getExtends(key) {
    const item = (record.value.extends || []).find(e => e.key === key)
    return item ? item.value : []
  }
  function setExtends(key, value) {
    if (!record.value.extends) record.value.extends = []
    const item = record.value.extends.find(e => e.key === key)
    if (item) item.value = value
    else record.value.extends.push({key, value})
  }
  const videoSources = computed({
    get: () => getExtends('video_sources'),
    set: (val) => setExtends('video_sources', val),
  })
  const galleryImages = computed({
    get: () => getExtends('gallery_images'),
    set: (val) => setExtends('gallery_images', val),
  })

  function addExtends(){
    if(!record.value.extends) record.value.extends = []
    record.value.extends.push({key:'',value:''})
  }

  function addRes(){
    if(!record.value.res) record.value.res = []
    record.value.res.push({key:'',value:''})
  }

  const slugURL = computed(()=> useAppendSiteURL(store, store.config.router.article_rule.replace('{slug}', record.value.slug)))

  function formatLabel(val){
    console.log(val)
    return "aaa"
  }

</script>


<style scoped>
  .input{
    background-color: var(--color-bg-5);
  }
  .input_extends{
    border-color: var(--color-border-3);
  }
  .json-display{
    background-color: var(--color-bg-5);
    padding: 10px;
    border-radius: 4px;
    border: 1px solid var(--color-border-3);
    font-size: 12px;
    color: var(--color-text-1);
    white-space: pre-wrap;
    word-wrap: break-word;
    max-height: 200px;
    overflow-y: auto;
  }
</style>
