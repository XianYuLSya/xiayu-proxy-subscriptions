# XiaYu Proxy Subscriptions

把已有的 Shadowsocks 节点整理成订阅链接。更新节点后，可在支持订阅的客户端里刷新。服务只提供配置，设备上的代理流量由客户端连接到 Shadowsocks 节点传输。

## 开始使用

需要 Go 1.22 或更新版本。程序只使用 Go 标准库。

在项目目录运行以下命令，生成本地配置。程序会为这份配置生成随机订阅令牌，并为每个示例节点生成 UUID。

```sh
go run . -init -config config.json
```

编辑 `config.json`，将示例地址、端口、加密方法和密码替换为自己的节点信息。节点须使用 Shadowsocks AEAD-2022，且无需插件。

| 字段 | 用途 |
| --- | --- |
| `token` | 订阅链接使用的私有令牌。 |
| `nodes` | 节点列表，每个节点填写下列字段。 |
| `id` | 节点在订阅路径中的短名，例如 `windows` 或 `android`。保留名 `all` 表示全部节点。 |
| `uuid` | SIP008 客户端用来识别同一节点的 UUID。修改已有节点时保留原值。 |
| `name` | 在客户端显示的节点名称。 |
| `server`、`port` | 节点地址和端口。 |
| `method`、`password` | 节点的加密方法和密码，填写与服务端一致的值。 |
| `udp` | 生成 Mihomo 配置时是否为该节点启用 UDP，填 `true` 或 `false`。 |

校验并启动服务。

```sh
go run . -check -config config.json
go run . -config config.json -listen 127.0.0.1:8090
```

本机访问 `http://127.0.0.1:8090/healthz` 可以确认服务是否启动。修改 `config.json` 后重启订阅服务，再到客户端刷新订阅。

## 导入客户端

下面的 `<token>` 换成配置里的实际令牌。Windows 与 Android 是示例节点的短名，路径按配置里的 `id` 选节点。要同时导入所有节点，把短名换成 `all`。本机的完整链接示例为 `http://127.0.0.1:8090/sub/<token>/windows/mihomo.yaml`，手机可使用下节的 HTTPS 地址。

| 使用情况 | 订阅路径 | 在客户端里的操作 |
| --- | --- | --- |
| Windows 上使用 Mihomo 客户端 | `/sub/<token>/windows/mihomo.yaml` | 添加远程配置并启用。配置里的 `XiaYu Route` 组将进入该配置的流量发往选中的节点，组里没有直连选项。 |
| Android 上使用 Mihomo 客户端 | `/sub/<token>/android/mihomo.yaml` | 添加远程配置并启用，列表里会显示安卓线路。 |
| Android 上使用支持 SIP008 的客户端 | `/sub/<token>/android/sip008.json` | 添加在线配置或 SIP008 订阅。 |
| 客户端支持导入 Shadowsocks 分享链接 | `/sub/<token>/all/ss.txt` | 取回文本，逐条复制其中的 `ss://` 链接导入。 |

选择客户端时，确认它同时支持相应的导入格式和节点使用的 AEAD-2022 加密方法。

## 让手机访问订阅

服务默认只在本机监听。需要从手机或其他设备刷新时，在运行订阅服务的同一台机器上配置 HTTPS 反向代理，将请求转发到 `127.0.0.1:8090`。以 `https://subscriptions.example.com/sub/<token>/android/sip008.json` 为例，把域名和令牌分别替换为自己的。SIP008 线上订阅使用 HTTPS。

把 `config.json`、订阅链接和导出的内容当作密码保管。仓库只提交占位配置。反向代理的访问日志应避免记录订阅 URL 中的令牌。

## 验证

```sh
go test ./...
go vet ./...
```

把环境变量 `XIAYU_TEST_MIHOMO` 指向 Mihomo 可执行文件，可同时检验生成的配置能否被 Mihomo 加载。

## 规范与许可

- [Mihomo Shadowsocks 配置](https://wiki.metacubex.one/config/proxies/ss/)
- [Shadowsocks SIP008 在线配置](https://shadowsocks.org/doc/sip008.html)
- [Shadowsocks SIP002 分享链接](https://shadowsocks.org/doc/sip002.html)
- [MIT License](LICENSE)
