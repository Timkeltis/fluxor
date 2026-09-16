package download

// Package download 负责把远程订阅拉取为本地节点文件。
//
// 采用「直连优先、临时内核回退」策略：
//  1. direct.go  直接 HTTP 下载，自动识别 Base64 订阅并解析 subscription-userinfo 头
//  2. 失败时回退到 core.StartTempCore 启动临时内核实例完成下载
//
// 本包为叶子依赖，不引用其它 internal 子包。注意 core 通过函数注入方式调用本包
// （见 core/lifecycle.go 的 SetTempCoreDownloader），以避免 core <-> subscription
// 的循环依赖。
