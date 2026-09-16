package subscription

// Package subscription 实现订阅中心：配置 CRUD、节点文件下载、定时更新与健康检查。
//
// 子目录 download 是独立的下载层（直接 HTTP 下载 + Base64 解析 + 元数据解析），
// 被本包复用；它不反向依赖本包，因此不会形成循环。
//
// 文件划分：
//   - api*.go        各类订阅 HTTP 接口（配置、生成、更新、元信息）
//   - patch.go       往订阅节点文件注入 Fluxor 必需的端口/密钥/DNS 字段
//   - generator.go   调用 configgen 生成 config.yaml
//   - ensure.go      切换模式下确保所有订阅文件就绪
//   - timers.go      定时更新与健康检查定时器的生命周期
//   - metadata.go    订阅流量/有效期等元数据的抓取与规范化
//   - healthcheck.go switch 模式下对激活订阅的周期性测速
