# Kindle Dashboard

为 Kindle 墨水屏制作的天气与日历看板。服务端用 Go 生成灰度 PNG，Kindle 客户端定时下载并显示图片。当前布局面向 Kindle Paperwhite 3，基准分辨率为 **1072 × 1448**。

## 功能

- 显示当前时间、公历和农历日期。
- 显示当天气温、天气与五日预报；天气数据从中国天气网公开页面获取，气温观测与天气预报是不同的数据。
- 月历以周一开头，突出今天，显示农历月份、传统节日和节气。
- 输出 8 位灰度 PNG，适配 Kindle `eips`；可按客户端上报的屏幕尺寸等比缩放。
- Kindle 端通过 KUAL 启动或停止看板，并可用屏幕键盘设置服务器地址。
- 附带实验性的 Wi-Fi IPv4 设置工具，可切换 DHCP 和静态地址，并测试 Ping。

## 工作方式

```text
中国天气网页面 ──> Go 服务端 ──HTTP/PNG──> Kindle 客户端 ──> 墨水屏
```

服务端需要能访问天气来源，Kindle 需要能访问服务端。程序运行时不依赖 AI 或数据库。天气抓取结果缓存 30 分钟；获取失败时会尝试使用缓存或显示不可用，公开网页的结构或访问规则变化可能影响数据获取。

## Docker 部署

GitHub Actions 构建并发布 `linux/amd64` 和 `linux/arm64` 镜像：

```text
ghcr.io/cn-404-not-found/kindle-dashboard:latest
```

从本仓库获取 `compose.yaml` 和 `.env.example`，在安装 Docker Compose 的服务器上执行：

```sh
cp .env.example .env
docker compose pull
docker compose up -d
```

在 `.env` 中配置：

```dotenv
HOST_PORT=8080
CITY_NAME=南宁
CITY_ID=
TIMEZONE_OFFSET=8
```

- `HOST_PORT` 是宿主机端口，容器内固定监听 `8080`。
- `CITY_NAME` 是显示的城市名；内置城市列表见 [`weather.go`](weather.go)。
- `CITY_ID` 可在使用内置城市时留空；其他城市需要填入中国天气网的 9 位城市代码。
- `TIMEZONE_OFFSET` 是相对 UTC 的整小时固定偏移，中国为 `8`；不会自动处理夏令时。

修改 `.env` 后执行 `docker compose up -d` 重新创建容器。打开 `http://服务器IP:8080/screen.png` 查看图片；若更改了 `HOST_PORT`，使用对应端口。配置和完整说明见 [DOCKER.md](DOCKER.md)。如果镜像包设为私有，拉取前需要用有包读取权限的 GitHub 账号登录 `ghcr.io`。

## Kindle 客户端

客户端需要可运行 KUAL 的 Kindle，以及设备上已安装的 kterm。将 [`kindle-client/dashboard-extension/`](kindle-client/dashboard-extension/) 中的扩展文件复制到 Kindle 的 `/mnt/us/extensions/kindle-dashboard/`，并将 [`kindle-client/TRMNL.sh`](kindle-client/TRMNL.sh) 复制到该目录，命名为 `dashboard.sh`。具体步骤见[客户端说明](kindle-client/dashboard-extension/README.md)。

从 KUAL 打开 **Kindle Dashboard**：

- **Start dashboard**：启动定时显示。
- **Stop dashboard**：停止客户端。
- **Set server address**：输入服务器地址，例如 `192.168.1.20:8080`。保存后，运行中的客户端在下次刷新时读取新地址。

服务端提供 `GET /screen.png` 和 `GET /api/display`；访问根路径 `/` 返回 404 属正常情况。

## Kindle 网络设置工具

[`kindle-client/network-settings/`](kindle-client/network-settings/) 包含 KUAL 网络设置工具，可输入静态 IP、子网掩码、网关和 DNS，切换 DHCP，并测试 Ping。静态模式会针对选定 Wi-Fi 尝试在重启或重连后恢复设置；程序会添加一个带标记的 cron 启动项。**无 DHCP 网络中的连接尚未验证**，使用前请阅读[网络工具说明](kindle-client/network-settings/README.md)，并准备好恢复网络的方式。

## 本地开发

需要 Go 1.25 或更新版本。将 `Sarasa-Regular.ttc` 放在程序或工作目录的 `fonts/` 子目录，也可放在同目录；`Sarasa-Bold.ttc` 可选，未提供时程序会模拟粗体。字体文件不存储在本仓库。

```sh
go mod download
go test ./...
cp -n config.example.json config.json
go run .
```

默认监听 `8080`。非 Docker 运行时可通过 `config.json` 配置端口、城市与时区；`CITY_NAME`、`CITY_ID`、`TIMEZONE_OFFSET` 环境变量优先覆盖相应 JSON 设置。配置在进程启动时读取。

## 已知限制

- 天气来自公开网页而非稳定的官方 API，页面变化或临近午夜的数据更新延迟可能导致天气暂时不可用。
- Kindle 真机已验证 Paperwhite 3 的灰度显示；其他型号和固件未全面测试。
- 网络工具尚未在无 DHCP 环境中完成验证；切换网络设置可能暂时断开连接。
- 本项目目前没有声明开源许可证；使用及再分发权限以仓库所有者后续提供的许可为准。
