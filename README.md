# Moss
moss is a simple and lightweight web content management system

moss是一个简单轻量的内容管理系统

可以使用mysql、postgresql、sqlite数据库。后台支持12种语言，可切换明暗风格

使用中不懂的可以加群问我

QQ交流群：68396947

TG交流群：[https://t.me/mosscms](https://t.me/mosscms)

------

### 配置文件(conf.toml)

| key  | 说明       | 默认      |
| ---- | ---------- | --------- |
| addr | 监听地址   | 随机      |
| db   | 数据库类型 | sqlite    |
| dsn  | 数据源     | ./moss.db?_pragma=journal_mode(WAL) |
      默认sqlite使用WAL方式打开，防止读取阻塞
+ 数据源示例

| Type       | dsn 示例                                                                           |
| ---------- | ---------------------------------------------------------------------------------- |
| sqlite     | ./data.db                                                |
| mysql      | user:password@tcp(127.0.0.1:3306)/moss?charset=utf8mb4&parseTime=True              |
| postgresql | host=127.0.0.1 port=5432 user=postgres password=123456 dbname=moss sslmode=disable |



### 命令行
| key         | 说明             | 示例                                   |
| ----------- | ---------------- | -------------------------------------- |
| --username  | 重置管理员用户名 |                                        |
| --password  | 重置管理员密码   |                                        |
| --adminpath | 重置后台路径     | ./moss --adminpath="admin"             |
| --config    | 指定配置文件路径 | ./moss --config="/home/othername.toml" |
