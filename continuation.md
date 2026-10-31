# Continuation: Kindle Dashboard

**Date:** 2026-11-01（Asia/Shanghai）
**Working directory:** /Users/liangyong/Documents/开发项目/Kindle-dashboard

## What This Project Is

Go HTTP 服务生成包含时间、天气、五日预报和农历月历的 Kindle 墨水屏 PNG；越狱 Kindle 通过 KUAL 客户端定时下载并用 `eips` 显示。服务端可以部署为 Docker 容器，运行时不需要 AI、大模型、天气 API 密钥或数据库。

## Tech Stack

- Go 1.25；`net/http`、`github.com/fogleman/gg`、`golang.org/x/image`、`github.com/6tail/lunar-go`。
- 中国天气网公开网页和城市列表，30 分钟天气缓存；字体采用更纱 TTC，不提交二进制字体。
- Kindle Paperwhite 3（第七代、固件 5.16.2.1.1），shell、KUAL、kterm、curl、eips、LIPC。
- GitHub Actions 构建公开 GHCR 镜像，支持 linux/amd64、linux/arm64。

## How to Build & Run

```sh
cd '/Users/liangyong/Documents/开发项目/Kindle-dashboard'
go mod download
go test ./...
# 首次运行复制示例，并提供 fonts/Sarasa-Regular.ttc；粗体 TTC 可选。
cp -n config.example.json config.json
go run .
# 另一个终端：
curl -f http://127.0.0.1:8080/api/display
curl -f -o /tmp/kindle-screen.png http://127.0.0.1:8080/screen.png
```

已有 `config.json` 是未跟踪的本地用户配置，不要覆盖、删除或提交。服务器根路径 `/` 返回 404 是正常行为，图片路径是 `/screen.png`。

## Key Files

| File | Purpose |
|------|---------|
| `main.go`, `render.go` | HTTP、数据组装、时间、图像缩放与渲染。 |
| `weather.go`, `cities.go` | 中国天气网天气解析、30 分钟缓存、城市名称查询；跨日时从实际当天开始选择预报。 |
| `kindle-client/TRMNL.sh` | Kindle 图片下载、`eips` 显示、按 Kindle 时钟对齐周期。 |
| `kindle-client/dashboard-extension/` | KUAL 菜单、服务器地址、保活、触摸前光和电源键退出。 |
| `kindle-client/network-settings/` | 测试版 Wi-Fi 静态 IPv4 / DHCP / Ping 设置。 |
| `.env.example`, `compose.yaml`, `Dockerfile`, `DOCKER.md` | Docker 配置和部署。 |
| `README.md`, `AGENTS.md` | 用户文档和项目规范。 |

## What Was Just Done (Most Recent Session)

- 已发布公开仓库及公开镜像 `ghcr.io/cn-404-not-found/kindle-dashboard:latest`；用户只需在 `.env` 填写城市名称，程序查城市列表（`a619af6`），无须城市 ID。
- 保活和前光（`68383e5`）：启动后阻止自动屏保，前光设为 0，触摸后设为 12、30 秒无触摸后设回 0；退出时恢复原状态。用户实机确认前光、静置约 5 分钟持续刷新正常。
- 修复午夜跨日天气（`d13b573`）：源网页仍以昨天开头时，从当天条目起选择连续五天；已用 11 月 1 日南宁真实网页和单元测试验证。
- 增加退出手势（`64447cb`）：5 秒内短按电源键 4 次，停止看板、恢复电源设置并尝试打开主页；用户已实机确认。
- 对齐刷新（`179dcc5`）：启动立即获取，此后以 Kindle 自身系统时钟对齐 60 秒周期；失败也等下一整分钟重试。用户实机确认。之前 `4e9951a` 设计的服务端时间字段已撤销，不要重新引入，用户明确指定使用 Kindle 时钟。
- 设备只读诊断：系统 `/usr/sbin/updatetime` 优先用 SNTP 请求 `ntp-g7g.amazon.com`，失败则用 `kindle-time.amazon.com` 的 HTTP 时间；配置检查窗口 86400 秒。未登录账号情况下存在 2026-11-01 01:50:25 CST 成功校时标记，标记不能区分 SNTP 与 HTTP。

## Current State

**Working:**
- 看板图像尺寸及方向、前光、5 分钟静置刷新、电源键退出、整分钟刷新，均已由用户在 Kindle 真机确认。
- 城市名自动解析、天气跨日处理和客户端逻辑已有测试；上次 `go test -count=1 ./...` 通过。
- 提交 `179dcc5` 的 GitHub Actions 镜像构建和冒烟测试成功。用户的远端服务器是否已拉取最新镜像不可确认。

**Broken / Known Issues:**
- 静态网络工具尚未在无 DHCP 的目标网络验证；曾在重启后看到 DHCP 地址，自动恢复不能视为已确认。
- 保活只实测约 5 分钟，不代表长期稳定；设备重启后需手动从 KUAL 启动看板。
- 前光和电源事件路径适配实机 `/dev/input/event1` 和 `/dev/input/event0`；触摸仍传给系统，其他设备需实测。
- 天气取公开网页而非稳定 API；网络、页面格式或更新延迟可能导致不可用。服务无认证，应限制可信网络访问。

**In Progress:**
- 本次任务为更新 README、归档并更新交接记录，以及提交推送；除此之外无用户明确要求的未完成工作。

## Pending Tasks

- 暂无用户明确要求且尚未完成的功能。后续如用户提供可测试的无 DHCP 网络，再验证静态模式重启、重连及恢复；长期保活可在用户需要时延长真机测试。

## Key Technical Decisions

- 看板时间由服务端渲染，刷新触发以 Kindle 系统时钟的整分钟为准；不通过服务端给客户端传时间，不修改任一系统时钟。传输、渲染和绘屏导致显示略晚。
- 中国天气网是当前数据源；不依赖 AI；城市名自动解析，网络不可用时仅部分常用城市有内置后备。
- Docker 配置写 `.env`，用户不希望为调整端口、城市、时区而修改 Compose 文件。
- 用户不手动改代码；遵守 AGENTS.md，只提交本任务相关文件，字体和本机配置不得入库。

## Gotchas & Traps

**天气跨日页面滞后**
午夜后中国天气网可能继续把昨天放在预报第一项，旧代码因此拒绝全部天气。
**Solution:** 找到真实当天，再验证并读取连续五天，不能把昨天标成今天。
**Lesson:** 测试跨日、跨月、天气缺失和刷新失败。

**Kindle 退出看板**
屏幕被 eips 图像覆盖且阻止自动屏保时，不容易通过 KUAL 菜单返回。
**Solution:** 5 秒内连续短按电源键四次；兜底可长按电源键约 40 秒重启。
**Lesson:** 功能变更必须保留用户可达的退出方式。

**Kindle 系统根分区只读**
静态网络工具最初无法写入 cron。
**Solution:** 查阅 `kindle-client/network-settings/` 现有实现和说明，不要假定 `/etc` 可直接写。
**Lesson:** 修改系统设置前验证挂载状态。

**旧交接文档已过期**
旧文件说 Kindle 未测试、城市必须填 ID、午夜天气未修复；这些结论已经过时。
**Solution:** 以本文件、代码和当前测试为准。
**Lesson:** 历史归档是当时状态，不是当前事实。

**配置和字体是本机数据**
`config.json` 未跟踪，`.env` 和 TTC 文件已忽略。
**Solution:** 不要提交、覆盖或删除；检查 git 状态，仅暂存任务相关文件。
**Lesson:** 交接和源码更新不能夹带用户配置。

## How to Deploy

在已安装 Docker Compose 的服务器：

```sh
mkdir -p kindle-dashboard
cd kindle-dashboard
curl -fsSL -o compose.yaml https://raw.githubusercontent.com/cn-404-not-found/kindle-dashboard/main/compose.yaml
curl -fsSL -o .env https://raw.githubusercontent.com/cn-404-not-found/kindle-dashboard/main/.env.example
# 根据服务器需要设置 .env 中 HOST_PORT、CITY_NAME、TIMEZONE_OFFSET。
docker compose pull
docker compose up -d
curl -f -o /tmp/kindle-screen.png http://127.0.0.1:8080/screen.png
```

已有部署不要用上述下载命令覆盖自己的 `.env`；进入原部署目录执行 `docker compose pull && docker compose up -d`。端口与防火墙需允许 Kindle 访问。客户端安装步骤见 README，升级时保留设备 `server-url.txt`。
