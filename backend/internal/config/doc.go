package config

// Package config 集中定义 Fluxor 的全局运行配置与持久化状态。
//
// 该包是整个后端的“叶子”依赖：只做路径/模式解析、订阅配置的读写与全局状态
// 的持有，绝不反向依赖 core、tproxy、subscription 等上层包，从而避免循环依赖。
//
// 主要职责：
//   - paths.go  运行路径（Socket、PID、内核二进制、配置目标等）的默认值
//   - modes.go  按运行模式（fnos / openwrt）套用默认路径
//   - env.go    环境变量读取小工具
//   - model.go  SubscribeConfig / Subscription 数据结构定义
//   - state.go  进程内共享的当前配置与读写锁
//   - load.go   配置的加载、默认值补齐与持久化
