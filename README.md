# Kindle Dashboard

为 Kindle 墨水屏制作的天气与日历看板。服务端用 Go 生成灰度 PNG，Kindle 客户端定时下载并显示图片。当前布局面向 Kindle Paperwhite 3，基准分辨率为 **1072 × 1448**。

## 功能

- 显示当前时间、公历和农历日期。
- 显示当天气温、天气与五日预报；天气数据从中国天气网公开页面获取，气温观测与天气预报是不同的数据。
- 月历以周一开头，突出今天，显示农历月份、传统节日和节气。
- 输出 8 位灰度 PNG，适配 Kindle `eips`；可按客户端上报的屏幕尺寸等比缩放。
- Kindle 端通过 KUAL 启动或停止看板，并可用屏幕键盘设置服务器地址。
- 看板运行时请求阻止自动屏保，意外退出后尝试重启；触摸屏幕点亮前光，30 秒后调回最低亮度。
- 附带实验性的 Wi-Fi IPv4 设置工具，可切换 DHCP 和静态地址，并测试 Ping。

## 工作方式

```text
中国天气网页面 ──> Go 服务端 ──HTTP/PNG──> Kindle 客户端 ──> 墨水屏
```

服务端需要能访问天气来源，Kindle 需要能访问服务端。程序运行时不依赖 AI、天气 API 密钥或数据库。城市名通过中国天气网城市列表自动匹配；天气抓取结果通常缓存 30 分钟，获取失败时会尝试使用缓存或显示不可用。公开网页的结构或访问规则变化可能影响数据获取。

## 安装与部署（Docker）

GitHub Actions 构建并发布 `linux/amd64` 和 `linux/arm64` 镜像：

```text
ghcr.io/cn-404-not-found/kindle-dashboard:latest
```

镜像包含程序和字体，目标机器不需要安装 Go。以下命令适用于已经安装 Docker、Docker Compose 和 `curl` 的 Linux 服务器。Docker 安装方法请参考 [Docker 官方安装指南](https://docs.docker.com/engine/install/)。

### 1. 准备配置文件

```sh
mkdir -p kindle-dashboard
cd kindle-dashboard
curl -fsSL -o compose.yaml https://raw.githubusercontent.com/cn-404-not-found/kindle-dashboard/main/compose.yaml
curl -fsSL -o .env https://raw.githubusercontent.com/cn-404-not-found/kindle-dashboard/main/.env.example
```

根据需要修改 `.env`：

```dotenv
HOST_PORT=8080
CITY_NAME=南宁
TIMEZONE_OFFSET=8
```

- `HOST_PORT`：服务器对外端口，容器内固定为 `8080`。
- `CITY_NAME`：填写城市名，如 `南宁` 或 `苏州`；程序会自动查询对应的天气数据。同名地区可使用 `省/城市`，例如 `广西/南宁`。
- `TIMEZONE_OFFSET`：相对 UTC 的整小时固定偏移，中国为 `8`；不自动处理夏令时。

### 2. 下载并启动

镜像已公开，无需登录 GitHub 或配置令牌：

```sh
docker compose pull
docker compose up -d
```

用浏览器打开 `http://服务器IP:8080/screen.png` 查看。如果修改了 `HOST_PORT`，请使用对应端口；服务器防火墙需允许 Kindle 访问该端口，服务端也需要能够访问中国天气网。

### 3. 查看、更新和停止

```sh
docker compose ps           # 查看状态
docker compose logs -f      # 查看日志，按 Ctrl+C 退出
docker compose pull         # 下载最新版镜像
docker compose up -d        # 使用最新镜像或应用新的 .env
docker compose down         # 停止并移除容器
```

修改 `.env` 后执行 `docker compose up -d`，单独执行 `docker compose restart` 不会重新读取环境变量。更多说明见 [DOCKER.md](DOCKER.md)。在 Kindle 的 **Set server address** 中填写 `服务器IP:端口`。

## Kindle 客户端

### 准备条件

- 已越狱、可运行 KUAL 的 Kindle；设置服务器地址还需要安装 kterm。越狱、KUAL 和 kterm 的安装不由本项目提供。
- Kindle 可以通过 Wi-Fi 访问运行中的服务端。输入的必须是 Kindle 能访问的服务器地址，不能填写 Kindle 自身的 `localhost`。
- 已在 Kindle Paperwhite 3（第 7 代、固件 5.16.2.1.1）验证图片显示、触摸前光和静置约 5 分钟持续刷新；其他设备和固件尚未全面测试。

### 安装

1. 从 GitHub 仓库下载项目（页面上选择 **Code → Download ZIP**）并解压。
2. 用 USB 将 Kindle 连接电脑，打开 Kindle 存储盘，在 `extensions` 下新建 `kindle-dashboard` 文件夹。
3. 将项目中 [`kindle-client/dashboard-extension/`](kindle-client/dashboard-extension/) 目录下的 `config.xml`、`menu.json`、`start.sh`、`stop.sh`、`power.sh`、`touch-light.sh`、`exit-button.sh`、`start-offline.sh`、`offline-dashboard.sh`、`offline-fetch.sh`、`offline-network.sh`、`offline-ui-guard.sh`、`set-server.sh`、`server-settings.sh` 复制到该文件夹。
4. 将 [`kindle-client/TRMNL.sh`](kindle-client/TRMNL.sh) 复制到同一文件夹，并重命名为 `dashboard.sh`。
5. 安全推出 Kindle，打开 KUAL → **Kindle Dashboard** → **Set server address**，输入例如 `192.168.1.20:8080` 的服务器地址并确认，再选择 **Start dashboard**。

也可输入 `http://` 或 `https://` 开头的地址；不支持 URL 路径、查询参数或 IPv6 字面地址。设置保存在设备本地 `server-url.txt`，运行中修改后通常在下一次刷新生效。升级时保留该文件；升级客户端脚本后先停止再启动看板。详细说明见[客户端说明](kindle-client/dashboard-extension/README.md)。

### 运行和前光

- **Start dashboard**：启动看板，启动时立即获取图片，随后按 Kindle 系统时钟在每个整分钟附近请求新图片；网络和绘屏需要额外时间。下载失败时会稍后重试。看板进程意外退出后会尝试重启。
- **Start offline dashboard**（开发测试版）：每分钟在设备本地重绘时间，约每两小时联网更新天气底图；依赖支持数字图块的新版服务端。详见[客户端说明](kindle-client/dashboard-extension/README.md)。
- **Stop dashboard**：停止看板，恢复启动前的前光亮度和屏保设置。也可以在 5 秒内连续短按电源键 4 次退出并返回主页（已在 Paperwhite 3 真机验证）。
- 运行期间会请求阻止自动屏保，并将前光亮度设为 0；触摸屏幕后设为 12，约 30 秒无触摸后恢复为 0。长时间展示建议连接电源。
- 触摸不会被看板拦截，仍可能触发 Kindle 原有界面。当前触摸设备路径针对上述 Paperwhite 3，其他型号需自行验证。设备重启后需要从 KUAL 再次启动看板。
- 整分钟刷新以 Kindle 的系统时钟为准；画面中的时间来自服务端。两台设备的时钟应保持准确，时区配置也应符合使用地区。已检查的 Paperwhite 3 系统带网络校时功能；实际能否校时取决于网络及设备配置。

服务端提供 `GET /screen.png` 和 `GET /api/display`；访问根路径 `/` 返回 404 属正常情况。浏览器预览地址为 `http://服务器IP:端口/screen.png`。服务本身没有访问认证，请限制在可信网络内访问。

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

默认监听 `8080`。非 Docker 运行时可通过 `config.json` 配置端口、城市与时区；`CITY_NAME`、`TIMEZONE_OFFSET` 环境变量优先覆盖相应 JSON 设置。配置在进程启动时读取。

## 已知限制

- 天气来自公开网页而非稳定的官方 API。程序会在午夜后从实际当天开始选择预报，但页面结构变化、预报不足或网络故障仍可能使天气暂时不可用。
- 已在 Paperwhite 3 上验证灰度显示、触摸前光和约 5 分钟静置刷新；尚未完成更长时间的连续运行测试，其他型号和固件未全面测试。
- 网络工具尚未在无 DHCP 环境中完成验证；切换网络设置可能暂时断开连接。
- 本项目目前没有声明开源许可证；使用及再分发权限以仓库所有者后续提供的许可为准。
