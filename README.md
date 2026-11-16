# XiaYu Proxy Subscriptions

把已有的 Shadowsocks 节点整理成订阅链接，也可以导出 sing-box 服务端配置，自建 Windows、Android 或 Linux 出口。客户端从订阅读取节点配置，代理流量由所选节点所在的系统发往目标站点。已有可用代理时，直接使用订阅功能即可。

## 准备配置

需要 Go 1.22 或更新版本。程序只使用 Go 标准库。

在项目目录运行下面的命令，在仓库外初始化私有配置。程序会生成随机订阅令牌，并为 `windows`、`android` 和 `linux` 三个示例节点各生成一个 UUID。

```sh
mkdir ../xiayu-private
go run . -init -config ../xiayu-private/config.json
```

编辑 `../xiayu-private/config.json`，保留需要的节点，填入客户端能访问的地址、端口、加密方法和密码。节点使用不带插件的 Shadowsocks AEAD-2022。已有的两节点配置可以直接使用，无需迁移。要新增节点，给它一个唯一的 `id` 和 UUID。

| 字段 | 用途 |
| --- | --- |
| `token` | 订阅链接使用的私有令牌。 |
| `nodes` | 节点列表，每个节点填写下列字段。 |
| `id` | 节点在订阅路径中的短名，例如 `windows`、`android` 或 `linux`。`all` 保留给全量订阅。 |
| `uuid` | SIP008 客户端用来识别同一节点的 UUID。修改已有节点时保留原值。 |
| `name` | 在客户端显示的节点名称。 |
| `server`、`port` | 客户端访问节点时使用的地址和端口。 |
| `method`、`password` | 加密方法和密码，与 Shadowsocks 服务端保持一致。 |
| `udp` | 是否为该节点启用 UDP。 |

## 自建 Shadowsocks 出口

Windows、Android、Linux 三条线路使用同一种 Shadowsocks AEAD-2022 协议。要得到相应系统的原生 TCP 出口，就在该系统上运行对应的服务端，由它向目标发起连接。给同一台机器上的几个节点改名或换端口，仍会使用同一个系统内核。外部检测工具也可能把 Android 和 Linux 归为同一类。

1. 在目标系统安装 sing-box。

   - Windows 使用 Windows 原生版 sing-box，并在 Windows 中启动。WSL 中运行的 sing-box 走 Linux 内核。[sing-box 安装文档](https://sing-box.sagernet.org/installation/package-manager/)
   - Android 使用 Termux，在 Termux 内运行 `pkg install sing-box`。[Termux 官方 sing-box 软件包](https://github.com/termux/termux-packages/blob/master/packages/sing-box/build.sh)
   - Linux 按发行版选择安装方式。[sing-box 安装文档](https://sing-box.sagernet.org/installation/package-manager/)

2. 为每条自建线路填写节点地址、端口和独立密码，再导出配置。选用 `2022-blake3-aes-128-gcm` 时，可用 `sing-box generate rand --base64 16` 生成密码。选用 `2022-blake3-aes-256-gcm` 或 `2022-blake3-chacha20-poly1305` 时，使用 32 字节。[sing-box Shadowsocks 入站文档](https://sing-box.sagernet.org/configuration/inbound/shadowsocks/)

   ```sh
   go run . -config ../xiayu-private/config.json -export-node windows -out ../xiayu-private/windows.server.json -node-bind 0.0.0.0
   go run . -config ../xiayu-private/config.json -export-node android -out ../xiayu-private/android.server.json -node-bind 0.0.0.0
   go run . -config ../xiayu-private/config.json -export-node linux -out ../xiayu-private/linux.server.json -node-bind 0.0.0.0
   ```

   按需运行对应系统的命令。`-out` 需要指向一个新文件。`-node-bind` 默认绑定回环地址。要让其他设备连接，显式填入节点的监听地址。示例中的 `0.0.0.0` 表示监听所有 IPv4 网卡。客户端使用的是 `server` 填写的可达地址。确保客户端能访问配置的端口。

3. 将导出的配置放到对应系统的私有目录。在那个系统上用安装好的 sing-box 检查，然后启动。例如在 Windows 的私有目录运行以下命令。Android 与 Linux 分别使用自己对应的文件名。

   ```sh
   sing-box check -c windows.server.json
   sing-box run -c windows.server.json
   ```

   导出的配置使用 sing-box 的 Shadowsocks 入站和直接出站。[sing-box 直接出站文档](https://sing-box.sagernet.org/configuration/outbound/direct/)

## 启动订阅服务

在项目目录校验配置并启动服务。

```sh
go run . -check -config ../xiayu-private/config.json
go run . -config ../xiayu-private/config.json -listen 127.0.0.1:8090
```

本机可访问 `http://127.0.0.1:8090/healthz` 确认订阅服务是否启动。修改配置后重启订阅服务，再到客户端刷新订阅。

## 导入客户端并选择出口

将下列路径接到订阅服务的地址后，再把 `<token>` 换成配置里的令牌。路径里的节点短名对应 `id`。`windows`、`android`、`linux` 指明所选出口，客户端可以运行在另一种系统上。

| 导入方式 | 订阅路径 | 在客户端里的操作 |
| --- | --- | --- |
| Mihomo 完整配置 | `/sub/<token>/all/mihomo.yaml` | 添加远程配置并启用。在 `XiaYu Route` 中手动选择 Windows、Android 或 Linux。该组的流量经所选节点转发。 |
| Mihomo 单个出口 | `/sub/<token>/windows/mihomo.yaml` | 只导入指定节点。用 `android` 或 `linux` 可改选另一个出口。 |
| SIP008 在线配置 | `/sub/<token>/android/sip008.json` | 在支持 SIP008 的客户端添加在线配置。用 `all` 可导入所有节点。 |
| Shadowsocks 分享链接 | `/sub/<token>/all/ss.txt` | 取回文本，逐条复制其中的 `ss://` 链接导入。 |

客户端需同时支持所用的订阅格式和 AEAD-2022 加密方法。本机可用 `http://127.0.0.1:8090/sub/<token>/all/mihomo.yaml`。

## 让其他设备访问订阅

订阅服务默认只在本机监听。需要从手机或其他设备刷新时，在运行订阅服务的同一台机器上配置 HTTPS 反向代理，将请求转发到 `127.0.0.1:8090`。以 `https://subscriptions.example.com/sub/<token>/android/sip008.json` 为例，把域名和令牌替换为自己的。SIP008 线上订阅使用 HTTPS。

把私有 `config.json`、导出的 `*.server.json`、订阅链接和下载的内容保存在仓库外，当作密码保管。反向代理的访问日志应避免记录订阅 URL 中的令牌。

## 验证

```sh
go test ./...
go vet ./...
```

把环境变量 `XIAYU_TEST_MIHOMO` 指向 Mihomo 可执行文件，可同时检验生成的配置能否被 Mihomo 加载。
把 `XIAYU_TEST_SINGBOX` 指向 sing-box 可执行文件，可验证三条节点配置和本机 Shadowsocks TCP 转发。

## 规范与许可

- [Mihomo Shadowsocks 配置](https://wiki.metacubex.one/config/proxies/ss/)
- [Shadowsocks SIP008 在线配置](https://shadowsocks.org/doc/sip008.html)
- [Shadowsocks SIP002 分享链接](https://shadowsocks.org/doc/sip002.html)
- [MIT License](LICENSE)
