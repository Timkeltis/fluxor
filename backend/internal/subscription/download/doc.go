package download

// Package download 负责把远程订阅拉取为本地节点文件。
//
// 采用「直连优先、临时内核回退」策略：
//  1. direct.go  直接 HTTP 下载，仅接受原样即为 Clash 明文 YAML 的响应，
//     并解析 subscription-userinfo 头
//  2. 直连内容不是 Clash YAML（Base64 编码的 URI 列表、裸 URI 列表、age 加密等）
//     时回退到临时内核实例完成下载——格式识别与解码统一交由内核，Fluxor 不自行实现
//
// 本包为叶子依赖，不引用其它 internal 子包。注意 core 通过函数注入方式调用本包
// （见 core/lifecycle.go 的 SetTempCoreDownloader），以避免 core <-> subscription
// 的循环依赖。
