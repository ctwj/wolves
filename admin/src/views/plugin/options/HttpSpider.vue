<template>

  <a-tabs v-model:active-key="activeKey" @change="onTabChange" class="http-spider-tabs">
    <!-- ============================== 全局设置 ============================== -->
    <a-tab-pane key="global" title="全局设置">
      <a-form-item label="HTTP 代理">
        <a-input v-model="data.proxy" placeholder="如 http://127.0.0.1:7890，留空不使用" allow-clear />
        <template #extra>
          <a-typography-text type="secondary" class="text-xs">
            采集走代理时填写，支持 http/socks5，留空直连。
          </a-typography-text>
        </template>
      </a-form-item>
      <a-form-item label="请求超时">
        <a-input-number v-model="data.timeout" class="input" :min="5" :max="300" />
        <span class="text-sm text-gray-400 ml-3">秒</span>
        <template #extra>
          <a-typography-text type="secondary" class="text-xs">
            单个 HTTP 请求超时（默认 30）；慢站可调大到 60~90。
          </a-typography-text>
        </template>
      </a-form-item>
      <a-form-item label="详情并发">
        <a-input-number v-model="data.concurrency" class="input" :min="1" :max="32" />
        <span class="text-sm text-gray-400 ml-3">路</span>
        <template #extra>
          <a-typography-text type="secondary" class="text-xs">
            任务内详情页并发抓取数（默认 4，提速核心）；目标站有限频/封 IP 时调小到 1~2。
          </a-typography-text>
        </template>
      </a-form-item>
      <a-form-item label="入库间隔">
        <a-input-number v-model="data.interval" class="input" :min="0" :max="600" />
        <span class="text-sm text-gray-400 ml-3">秒</span>
        <template #extra>
          <a-typography-text type="secondary" class="text-xs">
            单个 worker 每篇入库后的等待秒数（默认 0）；注意并发下不构成全局节流，严格限频请调低「详情并发」。
          </a-typography-text>
        </template>
      </a-form-item>

      <a-divider orientation="left" :margin="8">调试与限额</a-divider>
      <div class="grid grid-cols-2 gap-x-4">
        <a-form-item label="失败重试">
          <a-input-number v-model="data.retry" class="input" :min="0" :max="10" />
          <template #extra>
            <a-typography-text type="secondary" class="text-xs">
              详情页采集失败后的重试次数（默认 0，间隔逐次递增 1s/2s/...）。
            </a-typography-text>
          </template>
        </a-form-item>
        <a-form-item label="采集限量">
          <a-input-number v-model="data.limit" class="input" :min="0" :max="100000" />
          <template #extra>
            <a-typography-text type="secondary" class="text-xs">
              每任务最多入库篇数，0=不限；调试时配 1~5 小样跑。
            </a-typography-text>
          </template>
        </a-form-item>
      </div>
      <a-form-item label="试运行">
        <a-switch v-model="data.dry_run" />
        <template #extra>
          <a-typography-text type="secondary" class="text-xs">
            只解析与打日志、不入库，调试选择器用。
          </a-typography-text>
        </template>
      </a-form-item>
      <a-form-item label="调试目录">
        <a-input v-model="data.debug_dir" placeholder="如 /tmp/spider_debug，留空关闭" allow-clear />
        <template #extra>
          <a-typography-text type="secondary" class="text-xs">
            列表页/详情页失败时保存响应 HTML，排查选择器必备。
          </a-typography-text>
        </template>
      </a-form-item>
    </a-tab-pane>

    <!-- ============================== 采集任务（结构化编辑） ============================== -->
    <a-tab-pane key="tasks">
      <template #title>
        采集任务
        <a-tag v-if="tasks.length" color="arcoblue" class="ml-1">{{ enabledCount }}/{{ tasks.length }}</a-tag>
      </template>

      <div class="flex items-center justify-between mb-3">
        <a-typography-text type="secondary" class="text-xs">
          每个任务独立配置选择器与入库方式；纯 HTTP 采集，适用于查看源代码（Ctrl+U）能看到完整内容的站点。
        </a-typography-text>
        <a-space>
          <a-button size="small" @click="insertExample">
            <template #icon><icon-code :size="16" /></template>
            插入示例
          </a-button>
          <a-button size="small" type="primary" @click="addTask">
            <template #icon><icon-plus :size="16" /></template>
            新增任务
          </a-button>
        </a-space>
      </div>

      <a-empty v-if="!tasks.length" description="暂无任务，点击「新增任务」创建，或到「JSON 源码」页粘贴已有配置" />

      <a-collapse v-else accordion expand-icon-position="left">
        <a-collapse-item v-for="(t, i) in tasks" :key="i" hide-follow-theme>
          <template #title>
            <span class="inline-flex items-center gap-2 min-w-0">
              <span class="text-gray-400 text-xs">#{{ i + 1 }}</span>
              <span class="truncate">{{ t.name || '未命名任务' }}</span>
              <a-tag v-if="t.content_type" :color="contentTypeColor(t.content_type)">{{ contentTypeLabel(t.content_type) }}</a-tag>
              <a-typography-text type="secondary" class="text-xs truncate">{{ hostOf(t.source_url) }}</a-typography-text>
            </span>
          </template>
          <template #extra>
            <span class="inline-flex items-center gap-2" @click.stop>
              <a-switch v-model="t.enable" size="small" type="round">
                <template #checked>启用</template>
                <template #unchecked>停用</template>
              </a-switch>
              <a-button type="text" size="mini" :disabled="i === 0" @click.stop="moveTask(i, -1)"><template #icon><icon-up :size="14" /></template></a-button>
              <a-button type="text" size="mini" :disabled="i === tasks.length - 1" @click.stop="moveTask(i, 1)"><template #icon><icon-down :size="14" /></template></a-button>
              <a-button type="text" size="mini" @click.stop="copyTask(i)"><template #icon><icon-copy :size="14" /></template></a-button>
              <a-popconfirm content="确定删除该任务？" type="warning" @ok="removeTask(i)">
                <a-button type="text" size="mini" status="danger"><template #icon><icon-delete :size="14" /></template></a-button>
              </a-popconfirm>
            </span>
          </template>

          <!-- 基本设置 -->
          <a-divider orientation="left" :margin="8">基本设置</a-divider>
          <a-form-item label="任务名">
            <a-input v-model="t.name" placeholder="用于日志区分，如 示例-视频站" allow-clear />
          </a-form-item>
          <a-form-item label="起始页 URL" required>
            <a-input v-model="t.source_url" placeholder="https://example.com/list/1.html" allow-clear />
            <template #extra>
              <a-typography-text type="secondary" class="text-xs">列表页第一页地址。</a-typography-text>
            </template>
          </a-form-item>
          <div class="grid grid-cols-2 gap-x-4">
            <a-form-item label="翻页模板">
              <a-input v-model="t.page_url_pattern" placeholder="含 {page}，留空只采起始页" allow-clear />
              <template #extra>
                <a-typography-text type="secondary" class="text-xs">
                  URL 模板翻页方式；与「下一页选择器」二选一，都配时优先模板。
                </a-typography-text>
              </template>
            </a-form-item>
            <a-form-item label="最多翻页">
              <a-input-number v-model="t.max_pages" class="input" :min="1" :max="999" />
            </a-form-item>
          </div>
          <a-form-item label="下一页选择器">
            <a-input v-model="t.next_page_sel" placeholder="如 a.next-page（仅支持 a[href]）" allow-clear />
            <template #extra>
              <a-typography-text type="secondary" class="text-xs">
                从页面解析「下一页」链接逐页 GET（不填 max_pages 时默认上限 50 页）；JS 按钮翻页（无 href）请用 HeadlessSpider。
              </a-typography-text>
            </template>
          </a-form-item>
          <a-form-item label="增量停止">
            <a-input-number v-model="t.stop_when_exists" class="input" :min="0" :max="100" />
            <span class="text-sm text-gray-400 ml-2">篇</span>
            <template #extra>
              <a-typography-text type="secondary" class="text-xs">
                连续 N 篇文章已存在时提前结束整个任务（0=不启用）。定时采集最新文章建议设 3~5：遇到旧文自动停，只采新增。
              </a-typography-text>
            </template>
          </a-form-item>
          <a-form-item label="采集模式">
            <a-radio-group v-model="t.mode" type="button" size="small">
              <a-radio value="detail">进详情页提取</a-radio>
              <a-radio value="list">列表页直接入库</a-radio>
            </a-radio-group>
            <template #extra>
              <a-typography-text type="secondary" class="text-xs">
                detail 并发打开每个链接提取标题/正文等字段；list 只用列表页信息快速入库（配下方列表条目字段）。
              </a-typography-text>
            </template>
          </a-form-item>

          <!-- 列表页提取 -->
          <a-divider orientation="left" :margin="8">列表页取链</a-divider>
          <a-form-item label="条目选择器" required>
            <a-input v-model="t.list_selector" placeholder="如 .video-list a 或 .item" allow-clear />
            <template #extra>
              <a-typography-text type="secondary" class="text-xs">
                列表页条目元素；默认从条目/其内 &lt;a&gt; 的 href 取详情链接，配合下方属性通道可从 data-id 等拼地址。
              </a-typography-text>
            </template>
          </a-form-item>
          <a-form-item label="链接只保留">
            <a-input v-model="t.link_include" placeholder="子串或 /正则/，留空不过滤" allow-clear />
          </a-form-item>
          <a-form-item label="链接排除">
            <a-input v-model="t.link_exclude" placeholder="子串或 /正则/，留空不排除" allow-clear />
          </a-form-item>
          <a-form-item label="取链属性通道">
            <a-input v-model="t.link_selector" placeholder="链接元素选择器：空=条目自身/其内 a" allow-clear />
            <div class="grid grid-cols-2 gap-2 mt-1">
              <a-input v-model="t.link_attr" placeholder="属性：默认 href；可 data-id" size="small" allow-clear />
              <a-input v-model="t.link_extract_regex" placeholder="正则：从属性值抽取" size="small" allow-clear />
            </div>
            <a-input v-model="t.link_url_template" class="mt-1" placeholder="URL 模板：含 {value} 替换，否则前缀拼接，如 https://a.tv/detail/" size="small" allow-clear />
            <template #extra>
              <a-typography-text type="secondary" class="text-xs">
                适配 href="javascript:void(0)"、data-id 拼详情地址的站点。
              </a-typography-text>
            </template>
          </a-form-item>

          <!-- 列表条目字段（list 模式 / 预查重） -->
          <a-divider orientation="left" :margin="8">列表条目字段（list 模式）</a-divider>
          <a-form-item label="条目标题">
            <a-input v-model="t.list_title_sel" placeholder="空=回退 h1~h6/首行文本" allow-clear />
            <template #extra>
              <a-typography-text type="secondary" class="text-xs">
                同时用于 detail 模式的列表预查重（已存在则不请求详情页）。
              </a-typography-text>
            </template>
          </a-form-item>
          <a-form-item label="条目封面">
            <a-input v-model="t.list_cover_sel" placeholder="条目封面选择器" allow-clear />
            <div class="grid grid-cols-2 gap-2 mt-1">
              <a-input v-model="t.list_cover_attr" placeholder="属性：默认 src；可 data-src" size="small" allow-clear />
              <a-input v-model="t.list_cover_extract_regex" placeholder="正则：background-image 条目图" size="small" allow-clear />
            </div>
          </a-form-item>
          <a-form-item label="条目摘要">
            <a-input v-model="t.list_desc_sel" placeholder="空=回退条目文本" allow-clear />
          </a-form-item>

          <!-- 详情页字段 -->
          <a-divider orientation="left" :margin="8">详情页字段（detail 模式）</a-divider>
          <a-typography-text type="secondary" class="text-xs block mb-2">
            统一提取模型：选择器定位 → 属性（留空=自动，meta 的 content 优先回退文本；封面是 src 优先）→ 正则抽取（可 /re/ 包裹，取第一捕获组，如 /url\(["']?([^"')]+)["']?\)/ 抽 background-image 图址）。
          </a-typography-text>
          <a-form-item label="标题">
            <a-input v-model="t.title_sel" placeholder="选择器，如 h1.title；留空取 <title>" allow-clear />
            <div class="grid grid-cols-2 gap-2 mt-1">
              <a-input v-model="t.title_attr" placeholder="属性：空=自动；可 data-title" size="small" allow-clear />
              <a-input v-model="t.title_extract_regex" placeholder="正则：如 /(.+?)\s*[-|]\s*站名$/ 去后缀" size="small" allow-clear />
            </div>
          </a-form-item>
          <a-form-item label="封面">
            <a-input v-model="t.cover_sel" placeholder="选择器：img / meta[...] / .vjs-poster；留空回退 og:image" allow-clear />
            <div class="grid grid-cols-2 gap-2 mt-1">
              <a-input v-model="t.cover_attr" placeholder="属性：空=自动(src→content)；可 style" size="small" allow-clear />
              <a-input v-model="t.cover_extract_regex" placeholder="正则：抽 style 里 background-image 图址" size="small" allow-clear />
            </div>
            <template #extra>
              <a-typography-text type="secondary" class="text-xs">
                例：封面在 &lt;div class="vjs-poster" style="background-image:url(...)"&gt; 时，属性填 style、正则填 /url\(["']?([^"')]+)["']?\)/。
              </a-typography-text>
            </template>
          </a-form-item>
          <a-form-item label="正文">
            <a-input v-model="t.content_sel" placeholder="正文容器选择器，取 innerHTML" allow-clear />
            <a-input v-model="t.content_extract_regex" class="mt-1" placeholder="正则：从容器 HTML 抽片段（script 内嵌正文/播放数据的站）" size="small" allow-clear />
            <template #extra>
              <a-typography-text type="secondary" class="text-xs">
                正文容器选择器（取 innerHTML）。命中后日志会记录正文长度与前 150 字预览。
              </a-typography-text>
            </template>
          </a-form-item>
          <div class="grid grid-cols-2 gap-x-4">
            <a-form-item label="正文翻页">
              <a-input v-model="t.content_next_sel" placeholder="「下一页」a[href] 选择器" allow-clear />
              <template #extra>
                <a-typography-text type="secondary" class="text-xs">
                  正文分页时逐页 GET 拼接（内容无新增即停）。
                </a-typography-text>
              </template>
            </a-form-item>
            <a-form-item label="翻页上限">
              <a-input-number v-model="t.content_max_pages" class="input" :min="1" :max="99" />
              <template #extra>
                <a-typography-text type="secondary" class="text-xs">
                  正文翻页安全上限，默认 20。
                </a-typography-text>
              </template>
            </a-form-item>
          </div>
          <a-form-item label="关键词">
            <a-input v-model="t.keywords_sel" placeholder="选择器；留空回退 meta[name=keywords]" allow-clear />
            <div class="grid grid-cols-2 gap-2 mt-1">
              <a-input v-model="t.keywords_attr" placeholder="属性：空=自动" size="small" allow-clear />
              <a-input v-model="t.keywords_extract_regex" placeholder="正则抽取" size="small" allow-clear />
            </div>
          </a-form-item>
          <a-form-item label="发布时间">
            <a-input v-model="t.publish_time_sel" placeholder="选择器；留空回退 article:published_time" allow-clear />
            <div class="grid grid-cols-2 gap-2 mt-1">
              <a-input v-model="t.publish_time_attr" placeholder="属性：空=自动" size="small" allow-clear />
              <a-input v-model="t.publish_time_extract_regex" placeholder="正则：从值中抽日期时间" size="small" allow-clear />
            </div>
          </a-form-item>
          <a-form-item label="播放源(直链)">
            <a-input v-model="t.video_src_sel" placeholder="如 .player video[src]；写入 extends[video_sources]" allow-clear />
            <div class="grid grid-cols-2 gap-2 mt-1">
              <a-input v-model="t.video_attr" placeholder="属性：默认 src；可 data-src" size="small" allow-clear />
              <a-input v-model="t.video_extract_regex" placeholder="正则：从属性值抽 m3u8 等" size="small" allow-clear />
            </div>
            <template #extra>
              <a-typography-text type="secondary" class="text-xs">
                mp4/m3u8 直链；前台 wolves 主题由 ArtPlayer+hls.js 播放。多集时直链在前、iframe 在后按序聚合。
              </a-typography-text>
            </template>
          </a-form-item>
          <a-form-item label="播放源(iframe)">
            <a-input v-model="t.video_iframe_sel" placeholder="第三方播放页 iframe 选择器（embed=true）" allow-clear />
            <div class="grid grid-cols-2 gap-2 mt-1">
              <a-input v-model="t.video_iframe_attr" placeholder="属性：默认 src；可 data-src" size="small" allow-clear />
              <a-input v-model="t.video_iframe_extract_regex" placeholder="正则抽取" size="small" allow-clear />
            </div>
          </a-form-item>
          <a-form-item label="集名">
            <a-input v-model="t.video_label_sel" placeholder="集名元素选择器，与播放源同数量同顺序" allow-clear />
            <div class="grid grid-cols-2 gap-2 mt-1">
              <a-input v-model="t.video_label_attr" placeholder="属性：空=文本" size="small" allow-clear />
            </div>
            <template #extra>
              <a-typography-text type="secondary" class="text-xs">
                缺省回退「第01集/第02集...」。
              </a-typography-text>
            </template>
          </a-form-item>
          <a-form-item label="图集">
            <a-input v-model="t.gallery_sel" placeholder="图集页图片选择器；写入 extends[gallery_images]" allow-clear />
            <div class="grid grid-cols-2 gap-2 mt-1">
              <a-input v-model="t.gallery_attr" placeholder="属性：默认 src；可 data-src" size="small" allow-clear />
              <a-input v-model="t.gallery_extract_regex" placeholder="正则：background-image 图集" size="small" allow-clear />
            </div>
          </a-form-item>

          <!-- 请求伪装 -->
          <a-divider orientation="left" :margin="8">请求头与编码（反爬）</a-divider>
          <a-form-item label="User-Agent">
            <a-input v-model="t.user_agent" placeholder="留空用内置默认 Chrome UA" allow-clear />
          </a-form-item>
          <a-form-item label="自定义 Headers">
            <a-input v-model="t.headers" placeholder='JSON 对象，如 {"Referer":"https://a.tv/"}' allow-clear />
            <template #extra>
              <a-typography-text type="secondary" class="text-xs">
                JSON 对象格式；非法 JSON 会在任务装载时报错。
              </a-typography-text>
            </template>
          </a-form-item>
          <a-form-item label="Cookies">
            <a-input v-model="t.cookies" placeholder="原始 Cookie 头字符串，如 uid=1; token=abc" allow-clear />
            <template #extra>
              <a-typography-text type="secondary" class="text-xs">
                从浏览器 F12 → 网络请求 → 请求头 Cookie 复制整串。
              </a-typography-text>
            </template>
          </a-form-item>
          <a-form-item label="响应编码">
            <a-input v-model="t.encoding" placeholder="如 gbk；留空自动检测" allow-clear />
            <template #extra>
              <a-typography-text type="secondary" class="text-xs">
                强制解码（gbk/gb18030/big5 等）；留空自动检测：header 声明 → meta 声明 → 非 UTF-8 回退 GBK。
              </a-typography-text>
            </template>
          </a-form-item>

          <!-- 扩展字段 -->
          <a-divider orientation="left" :margin="8">扩展字段（extends）</a-divider>
          <div v-for="(ex, j) in (t.extra || [])" :key="j" class="flex items-center gap-2 mb-2 flex-wrap">
            <a-input v-model="ex.key" placeholder="键名 如 author" style="width: 120px" />
            <a-input v-model="ex.selector" placeholder="CSS 选择器 或 @url（从页面 URL 提取）" style="width: 230px" />
            <a-select v-model="ex.attr" style="width: 130px" placeholder="取值方式">
              <a-option value="">自动(文本兜底)</a-option>
              <a-option value="html">innerHTML</a-option>
              <a-option value="src">src 属性</a-option>
              <a-option value="href">href 属性</a-option>
              <a-option value="content">content 属性</a-option>
              <a-option value="style">style 属性</a-option>
            </a-select>
            <a-input v-model="ex.regex" placeholder="正则抽取，可 /re/ 取第一捕获组" class="flex-1" style="min-width: 150px" />
            <a-tooltip content="多值：聚合所有匹配元素为数组">
              <a-switch v-model="ex.multiple" size="small" />
            </a-tooltip>
            <a-button type="text" size="mini" status="danger" @click="t.extra.splice(j, 1)">
              <template #icon><icon-delete :size="14" /></template>
            </a-button>
          </div>
          <a-button size="small" type="outline" @click="addExtra(t)">
            <template #icon><icon-plus :size="14" /></template>
            添加字段
          </a-button>

          <!-- 入库设置 -->
          <a-divider orientation="left" :margin="8">入库设置</a-divider>
          <a-form-item label="入库类型">
            <a-radio-group v-model="t.content_type" type="button" size="small">
              <a-radio value="">普通</a-radio>
              <a-radio value="novel">小说</a-radio>
              <a-radio value="image">图集</a-radio>
              <a-radio value="video">视频</a-radio>
            </a-radio-group>
            <template #extra>
              <a-typography-text type="secondary" class="text-xs">
                决定前台 wolves 主题用哪种模板渲染，普通文章选「普通」。
              </a-typography-text>
            </template>
          </a-form-item>
          <a-form-item label="入库分类">
            <SelectCategory v-model="t.category_id" />
            <template #extra>
              <a-typography-text type="secondary" class="text-xs">
                采集文章归入的栏目；右侧按钮可直接输入栏目 ID。
              </a-typography-text>
            </template>
          </a-form-item>
          <a-form-item label="去重键">
            <a-radio-group v-model="t.dedup_by" type="button" size="small">
              <a-radio value="">URL+标题</a-radio>
              <a-radio value="url">仅 URL</a-radio>
            </a-radio-group>
            <template #extra>
              <a-typography-text type="secondary" class="text-xs">
                默认 URL+标题 哈希；仅 URL 时站点标题微调不会重复入库。注意切换后既有文章会按新键被重新采集。
              </a-typography-text>
            </template>
          </a-form-item>
        </a-collapse-item>
      </a-collapse>
    </a-tab-pane>

    <!-- ============================== JSON 源码 ============================== -->
    <a-tab-pane key="json" title="JSON 源码">
      <a-alert class="mb-2" type="warning">
        在此编辑任务 JSON 后，请点击「解析并应用」同步到「采集任务」表单；切换页签时也会自动尝试应用。任务 JSON 与 HeadlessSpider 配置模型对齐，可平移复用。
      </a-alert>
      <a-textarea v-model="data.tasks" class="font-mono text-xs" placeholder="[]" :auto-size="{ minRows: 12, maxRows: 24 }" />
      <a-space class="mt-2">
        <a-button size="small" type="primary" @click="applyJSON">
          <template #icon><icon-check :size="14" /></template>
          解析并应用
        </a-button>
        <a-button size="small" @click="formatJSON">格式化 / 校验</a-button>
        <a-typography-text type="secondary" class="text-xs">{{ jsonSummary }}</a-typography-text>
      </a-space>
    </a-tab-pane>
  </a-tabs>

</template>

<script setup>
import { computed, inject, onMounted, ref, watch } from "vue";
import { Message } from "@arco-design/web-vue";
import SelectCategory from "@/components/data/SelectCategory.vue";

const data = inject("options");

if (!data.value.tasks) {
  data.value.tasks = "[]";
}

// 解析已有 tasks JSON；失败则停在 JSON 源码页让用户先修复
let initTasks = [];
let initError = "";
try {
  const parsed = JSON.parse(data.value.tasks || "[]");
  if (Array.isArray(parsed)) {
    initTasks = parsed;
  } else {
    initError = "tasks 必须是 JSON 数组";
  }
} catch (e) {
  initError = e.message;
}

const activeKey = ref(initError ? "json" : "global");
const tasks = ref(initTasks);

// 表单编辑 → 写回插件配置（初始化赋值发生在 watch 建立之前，不会误触发）
watch(tasks, (v) => {
  data.value.tasks = JSON.stringify(v, null, 2);
}, { deep: true });

if (initError) {
  onMounted(() => Message.warning(`tasks JSON 解析失败（${initError}），请在本页修复后再使用表单`));
}

const enabledCount = computed(() => tasks.value.filter((t) => t.enable).length);

const jsonSummary = computed(() => {
  try {
    const parsed = JSON.parse(data.value.tasks || "[]");
    if (!Array.isArray(parsed)) return "tasks 必须是 JSON 数组";
    return `共 ${parsed.length} 个任务，${parsed.filter((t) => t.enable).length} 个启用`;
  } catch (e) {
    return "JSON 格式错误：" + e.message;
  }
});

// 切换页签：离开 JSON 源码页时自动解析应用，失败则拉回
function onTabChange(key) {
  if (key !== "json") {
    applyJSON(true);
  }
}

function parseJSON() {
  const parsed = JSON.parse(data.value.tasks || "[]");
  if (!Array.isArray(parsed)) {
    throw new Error("tasks 必须是 JSON 数组");
  }
  return parsed;
}

// JSON → 表单。silent 模式（切页触发）下 JSON 与表单一致时不打扰用户
function applyJSON(silent) {
  let parsed;
  try {
    parsed = parseJSON();
  } catch (e) {
    activeKey.value = "json";
    Message.error("tasks 解析失败：" + e.message);
    return;
  }
  const next = JSON.stringify(parsed, null, 2);
  if (silent && next === JSON.stringify(tasks.value, null, 2)) return;
  tasks.value = parsed;
  Message.success(`已应用到表单，共 ${parsed.length} 个任务`);
}

function formatJSON() {
  try {
    const parsed = parseJSON();
    data.value.tasks = JSON.stringify(parsed, null, 2);
    Message.success(`JSON 校验通过，共 ${parsed.length} 个任务`);
  } catch (e) {
    Message.error("tasks 解析失败：" + e.message);
  }
}

// 字段与后端 httpTask 对齐（含 HTTP 专属：next_page_sel/取链属性通道/正文翻页/请求伪装/dedup_by）
function newTask() {
  return {
    name: "新任务",
    enable: false,
    source_url: "",
    page_url_pattern: "",
    max_pages: 1,
    next_page_sel: "",
    mode: "detail",
    list_selector: "",
    link_include: "",
    link_exclude: "",
    link_selector: "",
    link_attr: "",
    link_extract_regex: "",
    link_url_template: "",
    list_title_sel: "",
    list_cover_sel: "",
    list_cover_attr: "",
    list_cover_extract_regex: "",
    list_desc_sel: "",
    stop_when_exists: 0,
    title_sel: "",
    title_attr: "",
    title_extract_regex: "",
    cover_sel: "",
    cover_attr: "",
    cover_extract_regex: "",
    content_sel: "",
    content_extract_regex: "",
    content_next_sel: "",
    content_max_pages: 20,
    keywords_sel: "",
    keywords_attr: "",
    keywords_extract_regex: "",
    publish_time_sel: "",
    publish_time_attr: "",
    publish_time_extract_regex: "",
    video_src_sel: "",
    video_attr: "",
    video_extract_regex: "",
    video_iframe_sel: "",
    video_iframe_attr: "",
    video_iframe_extract_regex: "",
    video_label_sel: "",
    video_label_attr: "",
    gallery_sel: "",
    gallery_attr: "",
    gallery_extract_regex: "",
    extra: [],
    content_type: "",
    category_id: 0,
    user_agent: "",
    headers: "",
    cookies: "",
    encoding: "",
    dedup_by: "",
  };
}

function addTask() {
  tasks.value.push(newTask());
}

function addExtra(t) {
  if (!Array.isArray(t.extra)) t.extra = [];
  t.extra.push({ key: "", selector: "", attr: "", regex: "", multiple: false });
}

function insertExample() {
  tasks.value.push({
    ...newTask(),
    name: "示例-视频站",
    source_url: "https://example.com/list/1.html",
    page_url_pattern: "https://example.com/list/1-{page}.html",
    max_pages: 3,
    list_selector: ".video-list a",
    link_include: "/detail-\\d+/",
    title_sel: "h1.title",
    // 封面在 <div class="vjs-poster" style="background-image:url(...)"> 的站：
    // 属性填 style，正则从属性值里抽图址
    cover_sel: ".vjs-poster",
    cover_attr: "style",
    cover_extract_regex: '/url\\(["\']?([^"\')]+)["\']?\\)/',
    content_sel: ".detail-content",
    video_src_sel: ".player video[src]",
    extra: [
      { key: "author", selector: ".author", attr: "", regex: "", multiple: false },
      // @url 伪选择器：从详情页 URL 抽视频 ID
      { key: "vid", selector: "@url", attr: "", regex: "/video/(\\d+)/", multiple: false },
    ],
    content_type: "video",
    category_id: 1,
    stop_when_exists: 3,
  });
  Message.success("已插入示例任务（默认停用，改好选择器后再启用）");
}

function copyTask(i) {
  const copy = JSON.parse(JSON.stringify(tasks.value[i]));
  copy.name = (copy.name || "任务") + " 副本";
  copy.enable = false;
  tasks.value.splice(i + 1, 0, copy);
}

function moveTask(i, offset) {
  const j = i + offset;
  if (j < 0 || j >= tasks.value.length) return;
  const [item] = tasks.value.splice(i, 1);
  tasks.value.splice(j, 0, item);
}

function removeTask(i) {
  tasks.value.splice(i, 1);
}

function hostOf(url) {
  try {
    return new URL(url).host;
  } catch {
    return url || "";
  }
}

const contentTypeLabels = { novel: "小说", image: "图集", video: "视频" };
const contentTypeColors = { novel: "orange", image: "green", video: "purple" };
function contentTypeLabel(v) { return contentTypeLabels[v] || v; }
function contentTypeColor(v) { return contentTypeColors[v] || "gray"; }
</script>

<style scoped>
.input {
  width: 100%;
}

.http-spider-tabs {
  max-height: 62vh;
  overflow-y: auto;
}

.http-spider-tabs :deep(.arco-tabs-content) {
  padding-top: 8px;
}
</style>
