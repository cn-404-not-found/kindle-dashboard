# Continuation: Kindle Dashboard

**Date:** 2026-11-16（Asia/Shanghai）
**Working directory:** `E:\开发项目\Kindle-dashboard`

## What This Project Is

Go HTTP 服务为越狱 Kindle Paperwhite 3 生成灰度仪表盘，显示时间、天气、农历和月历。Kindle 使用 KUAL 脚本通过局域网获取图片；横屏在线模式保持联网，时钟和天气区域支持局部刷新，以减少整屏闪烁。

## Tech Stack

- Go 1.25，module `kindle-dashboard`；`github.com/fogleman/gg`、`golang.org/x/image/font/{sfnt,opentype}`、`github.com/6tail/lunar-go`。
- Docker Compose；GitHub Actions 将 linux/amd64 和 linux/arm64 镜像发布到 `ghcr.io/cn-404-not-found/kindle-dashboard:latest`，也发布提交 SHA 标签。
- Kindle Paperwhite 3：1072×1448，KUAL、shell 和 `eips`；官方 `Sarasa-Regular.ttc` 为默认字体，`Sarasa-Bold.ttc` 可选，字体不入库。

## How to Build & Run

在安装 Go 1.25 的 Linux 机器上，将官方更纱 TTC 字体放到可执行文件同目录或 `fonts/`：

```sh
go mod download
go test ./...
go build -o kindle-dashboard .
./kindle-dashboard
curl -f -o screen.png http://127.0.0.1:8080/screen.png
curl -f -o landscape.png 'http://127.0.0.1:8080/screen.png?layout=landscape'
```

Windows 的全量测试包含 Linux 专用 syscall，无法原生完整运行。本次临时下载的 Go 位于 `$env:TEMP\kindle-preview-go\go\bin\go.exe`，临时目录可能被清理。Windows 适用测试：

```powershell
$files = @(Get-ChildItem -Name *.go | Where-Object { $_ -notlike '*_test.go' })
$files += @('landscape_test.go','weather_test.go','calendar_test.go','local_calendar_test.go','display_test.go')
& (Join-Path $env:TEMP 'kindle-preview-go\go\bin\go.exe') test @files
```

## Key Files

| 文件 | 用途 |
| --- | --- |
| `AGENTS.md` | 项目规范：只改需求相关代码、测试、中文提交、推送，不提交字体。有关天气 Mock 的旧描述已过时。 |
| `landscape.go` | 横屏布局，包括本次调整的天气区域。 |
| `landscape_partial.go` | 在旋转后的 PW3 全图上裁切时钟及天气局部图块。 |
| `main.go` | HTTP 路由、数据组装、设备旋转和局刷协议；启动天气后台更新。 |
| `weather.go` | 中国天气网数据获取、缓存、每小时半点主动更新及失败重试。 |
| `render.go` | 原竖屏绘图、字体加载与自动缩字。 |
| `kindle-client/dashboard-extension/landscape-partial.sh` | 横屏在线局刷客户端。 |
| `landscape_test.go`、`landscape_partial_client_test.go` | 横屏布局相关和 Linux shell 局刷集成测试。 |
| `.github/workflows/container.yml` | Linux 全量测试、双架构构建发布和默认竖屏 PNG 冒烟测试。 |
| `compose.yaml`、`DOCKER.md` | 容器配置和部署说明。 |

## What Was Just Done (Most Recent Session)

1. 从 GitHub 拉取到 `5cdc145 feat: 在每小时半点主动更新天气`；该版本启动时获取天气，之后每分钟检查半点更新与失败重试。
2. 用户要求放大横屏左侧中部天气描述、最高／最低温度及更新时间，先预览。使用临时副本和官方更纱字体生成多版图，用户选择最高／最低温同一行、位于当前温度和天气描述下方，并要求天气内容稍向上移动以平衡留白。
3. 用户接受最终预览并明确要求更新代码、上传 GitHub、构建容器，服务器由用户自行更新。`02b91e7 fix: 调整横屏天气区域排版` 仅修改 `landscape.go`：天气描述 48 号字、最高／最低温居中一行 34 号字、更新时间 28 号字，并调整纵向位置。长文本仍由 `fitText` 按宽度缩小。
4. Windows 适用测试通过；[GitHub Actions 43821283184](https://github.com/cn-404-not-found/kindle-dashboard/actions/runs/43821283184) 的 Linux `go test ./...`、双架构镜像发布和容器 PNG 冒烟检查均通过。`main` 已推送至 `02b91e7`，后续交接文档提交可能使 HEAD 前进。
5. 最终预览位于 `C:\Users\leung\.codex\visualizations\2026\11\16\01a1e318-f4f5-72b2-8919-ed0b30fe69f2\weather-preview-v4.png`；其中天气是示例数据，不是实况。

## Current State

**Working:**
- 新横屏排版已由用户接受、提交、推送；GHCR 镜像已构建发布。
- 本次 Windows 适用测试与 GitHub Linux 全量测试通过；CI 冒烟检查默认竖屏 PNG，并未验证横屏实机效果。
- 横屏客户端启动全刷；之后每分钟时钟局刷、整点天气局刷、跨日全刷。设备长期联网，不显示电量或 Wi-Fi。

**Broken / Known Issues:**
- 没有证据表明用户已在服务器拉取新版镜像，也没有新版天气布局的 Kindle 真机反馈。
- 上次交接时，Kindle 局刷闪烁／残影和最新局刷版 `awesome` 恢复机制仍待真机反馈；不可声称已确认。
- Windows 本地无法直接运行 Linux 专用测试。字体文件不应提交。

**In Progress:**
- 无未完成代码修改。用户自行更新服务器容器，之后可观察 Kindle 显示效果。

## Pending Tasks

1. 用户明确表示：“后续我自己在服务器更新容器，你只需要告诉我如何更新就好了。” 命令已提供；无需替用户部署，除非用户另行要求。
2. 等待用户反馈实机效果；如有问题先检查服务器镜像版本、Kindle `/tmp/kindle-dashboard.log`、局刷协议响应头、图块坐标和屏幕方向。

## Key Technical Decisions

- 横屏天气使用用户认可的单行最高／最低温布局，并在足够宽的区域居中；天气文本与更新时间放大。不要恢复成两行或擅自重排。
- 默认 `/screen.png` 为竖屏，`?layout=landscape` 为横屏预览；Kindle 以 `X-Dashboard-Layout: landscape-v1` 请求由服务器预旋转的图像。
- 局刷图块从旋转后的 1072×1448 全图裁切；clock `(852,55)`、168×544，weather `(182,55)`、519×544，协议版本为 1。客户端必须校验响应头。
- 天气后台在启动时获取，此后每分钟检查，在每小时半点更新；失败保留缓存并限频重试。
- 服务器端每分钟仍生成整图再裁切；局刷减少设备绘制，不表示关闭 Wi-Fi 或服务端局部渲染。
- 原生界面通过暂停 `awesome` 避免状态栏覆盖；守护脚本负责异常恢复。不要删除保护机制。

## Gotchas & Traps

**Git 目录所有权**  
本机仓库归属另一个 Windows SID，Git 默认报 `dubious ownership`。  
**Solution:** 单次调用使用 `git -c safe.directory='E:/开发项目/Kindle-dashboard' ...`；`gh` 指定 `--repo cn-404-not-found/kindle-dashboard`。  
**Lesson:** 不必修改全局 Git 信任设置。

**天气排版宽度和视觉居中**  
在右侧窄列直接放同一行最高／最低温会触发自动缩字；拆成两行的预览被用户认为拥挤、垂直不居中。  
**Solution:** 在当前温度及天气描述下方用整行宽度居中显示最高／最低温，并适度上移内容。  
**Lesson:** 保留用户认可的最终布局；示例预览不代表实机效果。

**局刷坐标与服务器版本**  
图块必须从已旋转整图裁切；新客户端要求协议头，旧服务器不会被误绘。  
**Solution:** 对照 `landscape_partial.go` 与响应头，更新镜像后重启 Kindle 横屏入口。  
**Lesson:** 镜像发布不代表服务器已部署。

**Windows 测试和 shell 换行**  
Linux 专用测试不能在 Windows 原生执行；Kindle shell 脚本需要 LF。  
**Solution:** 本机运行适用测试，以 GitHub Actions 执行 Linux 全量；复制设备脚本后检查字节和换行。  
**Lesson:** 区分平台限制与程序错误。

**GitHub workflow 权限**  
此前 OAuth token 不具备 `workflow` scope，不能随意修改工作流。  
**Solution:** 沿用现有工作流；该工作流的 PNG 冒烟测试只验证默认竖屏。  
**Lesson:** 不要声称 CI 已验证横屏局刷实机效果。

## How to Deploy

用户在服务器现有 Compose 目录自行运行：

```sh
docker compose pull
docker compose up -d
docker compose ps
```

然后重新启动 Kindle 横屏看板。可用 `http://服务器地址:端口/screen.png?layout=landscape` 在浏览器预览；上次已知服务器地址 `192.168.1.250:26168` 可能已变化。不要假设部署已完成。
