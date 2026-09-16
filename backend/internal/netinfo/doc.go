package netinfo

// Package netinfo 提供网络信息查询能力。
//
// 文件划分：
//   - interfaces.go 系统网卡枚举与本地出口 IP 回退
//   - iplookup.go   经代理或直连查询公网 IP（兼容 JSON 与纯文本响应）
//   - geo.go        IP 归属地（国家/地区/ISP）查询，带 10 分钟缓存
//   - proxyport.go  从内核配置读取实际生效的代理端口
//   - addr.go       TCP 监听地址格式校验
//   - handlers.go   /ipinfo/* 与 /interfaces HTTP 接口
