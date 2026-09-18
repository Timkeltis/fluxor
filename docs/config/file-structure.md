# 目录与路径配置

本页面为您介绍系统的默认目录结构以及在运行/启动服务时可供调配的系统环境变量。

---

## 默认目录结构

如果您是通过安装包安装或处于飞牛 OS 默认环境下运行，底层的程序、库文件、订阅以及日志的默认路径分布如下：

```text
/var/apps/Fluxor/
├── target -> /vol1/@appcenter/Fluxor/      # 程序目录（软链）
│   ├── app.sock              # Fluxor 自身监听的 UNIX Socket
│   ├── core.sock             # mi鸿蒙 内核监听的 UNIX Socket
│   └── bin/
│       ├── mihomo            # mi鸿蒙 内核二进制执行程序
│       └── fluxor            # Fluxor 后端二进制管理程序
├── var -> /vol1/@appdata/Fluxor/           # 运行时数据目录（软链）
│   ├── core.pid              # 内核进程的 PID 运行记录文件
│   ├── fluxor.pid            # 面板自身的 PID 运行记录文件
│   └── fluxor.json           # 面板设置与订阅持久化 JSON 文件
├── etc -> /vol1/@appconf/Fluxor/           # 配置目录（软链）
└── shares/                                 # 数据共享目录
    ├── Fluxor -> /vol1/@appshare/Fluxor/   # 内核工作目录
    │   ├── config.yaml       # 当前在内核中实际生效的配置文件
    │   ├── info.log          # 面板启停、内核运维等操作的日志文件
    │   ├── proxies/          # 各订阅下载的节点源文件存储目录
    │   └── ruleset/          # 规则集模板目录
    └── ui -> /vol1/@appshare/Fluxor/ui/    # 外部面板静态文件
        ├── meta/             # 预置 MetaCubeXD 面板的静态文件目录
        └── zash/             # 预置 Zashboard 面板的静态文件目录
```

> 飞牛 OS 下 `/var/apps/Fluxor/` 内的 `target`、`var`、`etc`、`shares/*` 均为指向 `/vol1/@app*` 的软链接，上表已标注真实落点；两套路径等价，访问哪个都可以。
>
> `info.log` **不是内核的运行日志**，而是面板自身记录启动、停止等运维动作的日志（同时也会写入启动脚本的输出）。内核的运行日志请在面板的「日志」页面查看。

> 内核二进制按惯例命名为 `mihomo`。若您自行重命名或放在其它位置，请通过下方环境变量覆盖。

---

## 启动与运行环境变量参考

若您是在fnOS环境中部署，或者需要调整后台的工作目录，请在启动二进制文件前，配置并导出以下环境变量：

```bash
# 设置管理面板访问的前缀路径（适用于飞牛 OS 等 NAS 网关环境）
export BASE_URL="/app/Fluxor"

# 设置面板自身监听的本地 Unix Socket 路径
export SOCKET_PATH="/var/apps/Fluxor/target/app.sock"

# 设置底层内核运行绑定的 Unix Socket 路径
export CORE_SOCKET="/var/apps/Fluxor/target/core.sock"

# 设置内核二进制程序的主路径
export CORE_BIN="/var/apps/Fluxor/target/bin/mihomo"

# 设置fluxor二进制程序的路径
export FLUXOR_BIN_DIR="/var/apps/Fluxor/target/bin/"

# 设置内核运行时的 PID 进程锁定文件路径
export CORE_PID_FILE="/var/apps/Fluxor/var/core.pid"

# 设置fluxor运行时的 PID 进程锁定文件路径
export FLUXOR_PID_FILE="/var/apps/Fluxor/var/fluxor.pid"

# 设置持久化保存您订阅与全局端口密钥等设置的文件路径
export FLUXOR_CONFIG_FILE="/var/apps/Fluxor/var/fluxor.json"

# 设置内核的主工作目录（用于存放 Geo 数据库、测速缓存、临时文件等）
export CORE_WORK_DIR="/var/apps/Fluxor/shares/Fluxor"

# 设置最终渲染生成的 mi鸿蒙 运行配置文件的目标路径
export CONFIG_TARGET="/var/apps/Fluxor/shares/Fluxor/config.yaml"

# 设置系统日志的保存文件路径
export INFO_LOG_FILE="/var/apps/Fluxor/shares/Fluxor/info.log"

# 可选：metacubexd 外置面板路径
# export META_DIR="/var/apps/Fluxor/shares/ui/meta"

# 可选：zashboard 外置面板路径
# export ZASH_DIR="/var/apps/Fluxor/shares/ui/zash"              

# 可选：如果需要通过传统的端口号（如 http://IP:8080）访问面板，可设置此项
# export FLUXOR_ADDR="0.0.0.0:8080"
```
若您是在openwrt等嵌入式环境中部署，可创建`/etc/fluxor`目录，将 fluxor 与 mi鸿蒙内核放置在此目录，以`./fluxor -w`启动以下预设的openwrt运行设置或自行修改相应环境变量：

```bash
# 通过传统的端口号（如 http://IP:8080）访问面板
# 也可通过`-a 0.0.0.0:18080`运行参数传入
export FLUXOR_ADDR="0.0.0.0:18080"

# 设置管理面板访问的前缀路径
export BASE_URL="/"

# 可选：设置面板自身监听的本地 Unix Socket 路径
# export SOCKET_PATH=""

# 设置底层内核运行绑定的 Unix Socket 路径
export CORE_SOCKET="/etc/fluxor/core.sock"

# 设置内核二进制程序的主路径
export CORE_BIN="/etc/fluxor/mihomo"

# 设置fluxor二进制程序的路径
export FLUXOR_BIN_DIR="/etc/fluxor/"

# 设置内核运行时的 PID 进程锁定文件路径
export CORE_PID_FILE="/var/run/core.pid"

# 设置fluxor运行时的 PID 进程锁定文件路径
export FLUXOR_PID_FILE="/var/run/fluxor.pid"

# 设置持久化保存您订阅与全局端口密钥等设置的文件路径
export FLUXOR_CONFIG_FILE="/etc/fluxor/fluxor.json"

# 设置内核的主工作目录（用于存放 Geo 数据库、测速缓存、临时文件等）
export CORE_WORK_DIR="/etc/fluxor"

# 设置最终渲染生成的 mi鸿蒙 运行配置文件的目标路径
export CONFIG_TARGET="/etc/fluxor/config.yaml"

# 设置系统日志的保存文件路径
export INFO_LOG_FILE="/etc/fluxor/info.log"

# 可选：metacubexd 外置面板路径
# export META_DIR="/etc/fluxor/ui/meta"

# 可选：zashboard 外置面板路径
# export ZASH_DIR="/etc/fluxor/ui/zash"              
```

---

## 关于环境变量

* **优先级**：环境变量 > 运行模式（`-f` / `-w`）的预设默认值。启动参数 `-a` 指定的监听地址优先级最高。
* **路径不必全部手动设置**：只要选择了运行模式，未设置的路径会自动套用该模式的默认值。上表之所以逐个列出，是为了便于您按需覆盖。
* **监听入口至少需要一个**：面板可以通过 Unix Socket（`SOCKET_PATH`）或 TCP 端口（`FLUXOR_ADDR`）对外提供服务，两者至少有一个非空，否则面板无法访问。
  * 飞牛 OS 默认模式：仅监听 Unix Socket，由系统反向代理暴露到 `/app/Fluxor`。
  * OpenWrt 默认模式：监听 `0.0.0.0:18080`，直接以端口访问。
* **`BASE_URL`**：面板对外访问的统一路径前缀。设为 `/` 会被归一化为空串（即根路径部署）。
* **修改 `FLUXOR_CONFIG_FILE` 前请先停止面板**：该文件同时承载订阅配置与 TProxy 状态，运行时被面板读写。
