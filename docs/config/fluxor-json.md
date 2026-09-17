# 持久化配置 (fluxor.json)

Fluxor 的设计核心之一是不引入复杂的数据库系统。所有的全局参数、机场订阅链接及拉取元数据，都持久化存储在一个 JSON 文件中。

---

## 配置文件路径

默认保存在 `/var/apps/Fluxor/var/fluxor.json`（真实落点为 `/vol1/@appdata/Fluxor/fluxor.json`）。

| 事项 | 说明 |
|------|------|
| 写入时机 | 点击网页上的「保存并应用」、修改订阅、切换 TProxy 开关等操作时，由程序即时写回 |
| 写入方式 | 「读—改—写」，只更新自己负责的字段，不整文件覆写 |
| 建议 | **面板运行期间不要手动编辑该文件**，避免覆盖掉正在写入的内容 |

> 该文件同时保存**订阅配置**与 **TProxy 状态**（`tproxy_*` 字段）。面板内部对它的读写共用同一把文件锁，因此不存在「保存订阅时把 TProxy 配置清空」的问题；但外部手动编辑无法参与这把锁，故仍建议先停止面板再改。

---

## 配置字段格式参考

以下是 `fluxor.json` 的完整结构示例，用以帮助您理解底层参数的流动和映射：

```json
{
  "proxy_port": 7890,
  "tproxy_port": 7898,
  "panel_port": 9090,
  "panel_secret": "your-panel-controller-secret",
  "rule_group": "base",
  "ui_panel": "metacubexd",
  "meta_backend_url": "",
  "mode": "merge",
  "active_subscription": "",
  "tproxy_enabled": false,
  "tproxy_dst_exceptions": [],
  "tproxy_src_exceptions": [],
  "tproxy_proxy_local": false,
  "subscriptions": [
    {
      "name": "my_airport",
      "url": "https://example.com/sub/link",
      "update_interval": 86400,
      "health_interval": 300,
      "prefix": "香港",
      "updated_at": "2026-07-02T15:00:00Z",
      "subscription_info": {
        "upload": 10737418240,
        "download": 53687091200,
        "total": 536870912000,
        "expire": 1782979200
      }
    }
  ]
}
```

---

## 核心字段详解

### 全局参数

* **`proxy_port`**：混合富强端口（Mixed Port），该端口同时支持 HTTP 和 SOCKS5 富强协议，默认 `7890`。取值为 `0` 表示禁用。
* **`tproxy_port`**：透明富强网关端口（TProxy Port），专门供 nftables 防火墙重定向劫持流量使用，默认 `7898`。
* **`panel_port`**：外部控制器监听端口，外置面板（如 MetaCubeXD）会通过该端口发送 HTTP API/WS 请求，默认 `9090`。
* **`panel_secret`**：面板连接的安全验证密钥。后端在转发内核请求时会自动附加为 `Bearer` 认证头，密钥不会下发给浏览器。
* **`rule_group`**：当前选用的规则分流集模板名称，取值为 `base`（标准）或 `full`（详细）。
* **`ui_panel`**：关联的外部控制面板界面类型，取值为 `metacubexd` 或 `zashboard`。
* **`meta_backend_url`**：外部面板回连面板后端所用的地址，留空表示使用默认推导值。
* **`mode`**：订阅加载方式，取值为 `merge`（融合模式）或 `switch`（切换模式）。详见 [订阅工作模式](./subscription-modes)。
* **`active_subscription`**：当处于切换模式（`switch`）时，当前正在生效并激活的订阅名。

### TProxy 状态字段

这几个字段由 TProxy 功能单独维护，通常不需要手工编辑：

* **`tproxy_enabled`**：透明富强开关的持久化值。**注意**：nftables 规则不跨重启存活，因此面板每次启动都会把该值**归零**并清理残留规则，确保「面板显示的状态」与「系统实际生效的规则」一致。
* **`tproxy_dst_exceptions`** / **`tproxy_src_exceptions`**：目的 / 源例外列表。
* **`tproxy_proxy_local`**：是否将面板主机自身的出站流量也纳入劫持。

> 旧版本曾使用 `tproxy_exceptions` 单一字段，现在会自动迁移到分离的源/目的两个字段。

### 订阅数组

* **`name`**：订阅名，作为节点的来源标识与文件名依据。
* **`url`**：机场订阅链接（`http(s)://`）。
* **`update_interval`**：以秒为单位的自动静默更新间隔（`0` 表示不自动更新）。
* **`health_interval`**：以秒为单位的后台健康测速频率。
* **`prefix`**：**节点名称前缀**（不是过滤正则）。填写后生成的节点名会自动带上该前缀，对应内核 `proxy-provider` 的 `override.additional-prefix`，用于多订阅时区分节点来源。
* **`updated_at`**：最近一次成功更新的时间。
* **`subscription_info`**：机场返回的流量配额，包含已用上传（`upload`）、已用下载（`download`）、总配额（`total`）与过期时间戳（`expire`），会在「订阅配置」页作为流量卡片渲染。

---

## 不会保存在这里的配置

以下内容**不在** `fluxor.json` 中，避免产生误解：

| 内容 | 实际存放位置 |
|------|--------------|
| 主题、语言、默认启动页 | 浏览器 `localStorage`（每台设备各自独立） |
| 代理页的排序方式、延迟阈值、节点过滤正则、自动断开开关 | 浏览器 `localStorage` |
| 内核的运行参数（端口以外的 allow-lan / ipv6 / mode / TUN 等） | 内核自身的 `config.yaml`，由面板通过内核 API 修改 |
| 自定义延迟测试 URL | 浏览器 `localStorage` |
