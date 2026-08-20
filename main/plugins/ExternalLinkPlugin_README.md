# 外链插件 (ExternalLinkPlugin)

## 功能说明

外链插件用于模拟浏览器访问外部链接地址，帮助提升网站的外部链接数量。插件会：

1. 定时或手动执行任务
2. 访问配置的外链地址列表
3. 将地址中的 `****` 替换为目标域名
4. 模拟浏览器请求（User-Agent、Referer、Cookies）
5. 记录每个请求的成功/失败状态

## 配置说明

### 基础配置

| 配置项 | 说明 | 默认值 |
|--------|------|--------|
| 启用插件 | 是否启用插件 | true |
| 域名 | 目标域名，用于替换 URL 模板中的 `****` | 必填 |
| 请求超时 | 单个请求的超时时间（秒） | 30 |
| 请求间隔 | 每个请求之间的延迟时间（毫秒） | 1000 |

### URL 模板

在 URL 模板中，每行一个 URL，`****` 会被替换为配置的域名。

**示例：**
```
https://example1.com/search?q=****
https://example2.com/api?url=****
https://example3.com/track?target=****
```

假设域名为 `moss.com`，则生成的实际 URL 为：
```
https://example1.com/search?q=moss.com
https://example2.com/api?url=moss.com
https://example3.com/track?target=moss.com
```

### 浏览器模拟

| 配置项 | 说明 | 默认值 |
|--------|------|--------|
| User-Agent | 浏览器标识 | Chrome 120.0.0.0 |
| Referer | 请求来源页面 | 空（智能生成） |
| Cookies | Cookie 字符串 | 空 |

**Referer 说明：**
- 留空时，插件会根据目标 URL 自动生成 Referer
- 智能生成规则：提取 URL 的协议、域名和端口（如 `https://example.com`）
- 建议留空，使用智能生成功能

**Cookies 格式：**
```
name1=value1; name2=value2; name3=value3
```

## 使用方法

### 1. 配置插件

1. 进入插件管理页面
2. 找到 "ExternalLinkPlugin"
3. 点击配置
4. 填写配置信息：
   - 设置目标域名
   - 添加 URL 模板（每行一个，使用 `****` 占位符）
   - 配置浏览器模拟参数（可选）

### 2. 手动执行

配置完成后，可以手动执行一次测试：

1. 在插件列表中点击 "运行"
2. 查看日志输出，确认请求结果

### 3. 启用定时任务

插件支持系统级定时任务，可在插件管理页面中：

1. 找到 "ExternalLinkPlugin"
2. 启用定时任务开关
3. 设置定时表达式（如 `@every 1h` 表示每小时执行一次）
4. 保存配置

插件将按照设定的时间自动执行。

## 日志说明

插件会记录以下日志信息：

### 启动日志
```
INFO: ExternalLinkPlugin started
  domain: example.com
  timeout: 30
  delay: 1000
  total: 5
```

### 请求日志
```
INFO: Requesting URL
  index: 1
  total: 5
  url: https://example.com/search?q=example.com
```

### 成功日志
```
INFO: Request succeeded
  index: 1
  url: https://example.com/search?q=example.com
```

### 失败日志
```
ERROR: Request failed
  index: 2
  url: https://example.com/api?url=example.com
  error: HTTP status code: 404
```

### 完成日志
```
INFO: ExternalLinkPlugin completed
  success: 4
  failed: 1
  total: 5
```

## 注意事项

### 1. 域名格式

- 域名不需要包含协议（http:// 或 https://）
- 示例：`example.com` 或 `www.example.com`

### 2. URL 格式

- URL 必须是有效的 HTTP/HTTPS 地址
- URL 中必须包含 `****` 作为占位符
- 每行一个 URL，空行会被忽略

### 3. 请求间隔

- 建议设置合理的请求间隔（1000ms 或更长）
- 避免请求过快被目标网站封禁
- 遵守目标网站的访问规则

### 4. 超时设置

- 根据目标网站的响应速度调整超时时间
- 默认 30 秒通常足够
- 如果网络较慢，可以适当增加

### 5. 定时任务

- 定时任务会持续执行，直到插件被禁用
- 建议设置合理的执行频率
- 避免过于频繁的请求

## 常见问题

### 1. 插件执行后没有记录日志

**原因：**
- 插件未启用
- 配置验证失败（域名或 URL 模板为空）

**解决方法：**
- 检查插件是否启用
- 检查域名和 URL 模板是否正确填写

### 2. 所有请求都失败

**原因：**
- 网络连接问题
- 目标网站无法访问
- 域名格式错误

**解决方法：**
- 检查网络连接
- 验证目标网站是否可访问
- 检查域名格式是否正确

### 3. 部分请求失败

**原因：**
- 目标网站拒绝访问
- 请求频率过高被限制
- URL 格式错误

**解决方法：**
- 增加请求间隔
- 检查失败的 URL 是否正确
- 查看具体的错误日志

## 最佳实践

### 1. URL 模板管理

- 将 URL 模板按类别分组（如：搜索引擎、社交媒体、论坛等）
- 定期更新 URL 模板，移除失效的地址
- 测试每个 URL 的有效性

### 2. 请求频率控制

- 设置合理的请求间隔（建议 1000ms 以上）
- 避免在短时间内大量请求同一网站
- 根据目标网站的承受能力调整频率

### 3. 监控和优化

- 定期查看插件日志
- 分析失败原因并优化配置
- 根据效果调整 URL 列表和参数

### 4. 合规性

- 遵守目标网站的使用条款
- 不要进行恶意请求
- 尊重网站的 robots.txt 规则

## 技术细节

### 请求特性

- 使用 HTTP GET 方法
- 自动设置常见浏览器头部
- 不跟随重定向
- 超时后自动终止请求

### 浏览器模拟

插件会自动设置以下头部：

```
User-Agent: 配置的 User-Agent
Accept: text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,*/*;q=0.8
Accept-Language: zh-CN,zh;q=0.9,en;q=0.8
Accept-Encoding: gzip, deflate, br
Connection: keep-alive
Upgrade-Insecure-Requests: 1
Referer: 智能生成或配置的 Referer
Cookie: 配置的 Cookies（如果设置）
```

**智能 Referer 生成：**

如果未配置 Referer，插件会根据目标 URL 自动生成：

- 提取 URL 的协议和域名
- 保留非默认端口
- 例如：
  - `https://example.com/page?q=xxx` → Referer: `https://example.com`
  - `http://example.com:8080/api` → Referer: `http://example.com:8080`

### 错误处理

- 连接失败：记录错误日志
- 超时：记录超时错误
- HTTP 错误（4xx/5xx）：记录状态码
- URL 格式错误：跳过该 URL 并记录警告

## 更新日志

### v1.0.0 (2026-03-19)

- 初始版本
- 支持基础外链请求功能
- 支持浏览器模拟
- 支持定时任务
- 完整的日志记录

## 支持和反馈

如有问题或建议，请通过以下方式联系：

- GitHub Issues: https://github.com/ctwj/moss/issues
- QQ 交流群: 68396947
- TG 交流群: https://t.me/mosscms