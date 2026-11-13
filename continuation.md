# Continuation: Kindle Dashboard

**Date:** 2026-11-13（Asia/Shanghai）
**Working directory:** `E:\开发项目\Kindle-dashboard`

## What This Project Is

Go HTTP 服务为越狱 Kindle Paperwhite 3 输出灰度看板，展示时间、天气、农历和月历；设备通过 KUAL shell 客户端在局域网获取图片。最新横屏版保持联网，只在时钟及天气区域进行局部刷新，减少每分钟清屏闪烁。

## Tech Stack

- Go 1.25、module `kindle-dashboard`、`github.com/fogleman/gg`、`golang.org/x/image/font/{sfnt,opentype}`、`github.com/6tail/lunar-go`。
- Docker Compose；GitHub Actions 在 `main` 更新后发布 `ghcr.io/cn-404-not-found/kindle-dashboard:latest`（linux/amd64、linux/arm64）。
- Kindle Paperwhite 3，1072×1448，固件 5.16.2.1.1；KUAL、shell、`eips`、LIPC、FBInk。
- 默认使用官方字体 `Sarasa-Regular.ttc`，`Sarasa-Bold.ttc` 可选；字体不入库。Windows 本机临时 Go：`D:\Personal\Temp\codex-go1259\go\bin\go.exe`，临时路径可能被清理。

## How to Build & Run

Linux/有 Go 的机器，放置官方更纱 TTC 字体于可执行文件同目录或 `fonts/`：

```sh
go mod download
go test ./...
go build -o kindle-dashboard .
./kindle-dashboard
curl -f -o screen.png http://127.0.0.1:8080/screen.png
curl -f -o landscape.png 'http://127.0.0.1:8080/screen.png?layout=landscape'
```

Windows 全量测试包含 Linux 专用 syscall，无法用 Windows Go 完整运行；最新提交的 GitHub Actions Linux 全量测试已通过。Windows 下运行适用的纯 Go 测试：

```powershell
$goFiles = @(Get-ChildItem -Name *.go | Where-Object { $_ -notlike '*_test.go' })
$goFiles += @('landscape_test.go','weather_test.go','calendar_test.go','local_calendar_test.go','display_test.go')
& "$env:TEMP\codex-go1259\go\bin\go.exe" test @goFiles
```

## Key Files

| 文件 | 用途 |
| --- | --- |
| `AGENTS.md` | 用户项目规范；禁止无关修改、提交字体或让用户手工改代码。有关天气 Mock 的旧描述已过时。 |
| `main.go` | HTTP 路由、日历/天气组装、设备尺寸和旋转、局部刷新协议响应头。 |
| `landscape.go` | 横屏看板设计，未来五日预报从明天开始。 |
| `landscape_partial.go` | 将旋转后的 PW3 整屏图裁剪为时钟和天气局部图块。 |
| `weather.go` | 中国天气网抓取与解析；成功数据缓存一小时，跨日重新获取。 |
| `render.go` | 旧竖屏绘图、字体解析及自动缩字。 |
| `kindle-client/dashboard-extension/landscape-partial.sh` | 最新横屏客户端；首次及跨日全刷，每分钟时钟局刷，每小时天气局刷。 |
| `kindle-client/dashboard-extension/start-landscape.sh` | KUAL 横屏入口，启动上述局刷脚本。 |
| `kindle-client/dashboard-extension/landscape-ui.sh` | 横屏时暂停/恢复 `awesome` 防状态栏覆盖，异常恢复依靠 `offline-ui-guard.sh`。 |
| `landscape_partial_client_test.go` | Linux shell 集成测试，验证首次、每分钟、整点、跨日刷新顺序。 |
| `.github/workflows/container.yml` | 自动运行 Linux 全量测试、构建发布镜像及原竖屏 PNG 冒烟测试。 |
| `DOCKER.md`, `compose.yaml` | 容器部署和服务器配置。 |

## What Was Just Done (Most Recent Session)

1. 用户之前确认横屏版可在 Kindle 显示，但原生时钟覆盖画面且天气预报字体偏小。`3f8aca2` 放大预报字体并在横屏时暂停 `awesome`，正常退出和异常退出有恢复保护；该版本镜像已发布，真机效果尚无用户反馈。
2. 用户表示“不需要显示电量，也不需要显示 WiFi”，接受长期联网，主要希望避免每分钟 `eips -c` 整屏清屏。`b970b08 feat: 支持横屏时钟和天气局部刷新` 已推送 `origin/main`。
3. 新横屏客户端启动时全刷一次；之后每分钟联网获取时钟图块，使用 `eips -g ... -x ... -y ...` 局刷；服务器小时变化时再获取天气图块；服务器日期变化时全刷。失败保留已有画面并在后续重试。按用户要求继续保持联网，不增加电量或 Wi-Fi 显示。
4. 用户 USB 连接 Kindle（当时 `G:`）；已安装并字节校验 `landscape-partial.sh` 与更新后的 `start-landscape.sh`，保留地址和其他菜单。旧启动脚本备份在设备的 `start-landscape.before-partial.sh.bak`。
5. 本机 HTTP 冒烟：全屏 1072×1448，时钟与天气局部图块与对应全图区域逐像素一致。Windows 适用 Go 测试、shell 语法检查通过；[GitHub Actions 43398102043](https://github.com/cn-404-not-found/kindle-dashboard/actions/runs/43398102043) 的 Linux 全量测试、构建和镜像冒烟检查通过。

## Current State

**Working:**
- `main` 和 `origin/main` 在本交接前均指向 `b970b08`，Actions 已成功发布对应 GHCR 镜像。
- 用户曾确认基础横屏版在 Kindle 上显示；当前局刷客户端文件已复制并验证 LF/字节内容。
- 协议局限于 PW3 的 1072×1448 帧缓冲。服务端返回 `X-Dashboard-Partial-Version: 1` 和区域坐标：clock `(852,55)` 尺寸 168×544；weather `(182,55)` 尺寸 519×544。

**Broken / Known Issues:**
- 用户未确认 `b970b08` 的 Kindle 真机局刷是否正常、是否仍闪烁或出现残影、每小时天气/跨日全刷是否成功。
- 用户先前明确选择“我自己更新服务器”；没有证据表明已经拉取并运行 `b970b08` 镜像。上次已知地址 `http://192.168.1.250:26168`，实际地址可能变化。
- `awesome` 状态栏屏蔽、四次短按电源键退出及恢复机制在之前基础模式实测过，但最新局刷版尚无用户实测反馈。全量 Linux 自动化测试包含正常和异常恢复。
- 当前模式阻止自动屏保，没有深度休眠或 RTC 唤醒；无法保证具体耗电或硬件刷新波形。较长天气描述会自动缩小。

**In Progress:**
- 等用户自行更新服务器并重新启动 Kindle 横屏模式，观察一分钟局刷、整点天气、跨日切换、原生状态栏和退出恢复。设备真实电池消耗尚未量化。

## Pending Tasks

1. 用户最新要求：“我我可以支持长期联网的，这个没问题只是我希望能够实现局部刷新而已”；并明确“不需要显示电量，也不需要显示 WiFi”。代码、设备脚本及 GitHub 镜像已完成，但服务器部署与真机局刷效果待用户确认。
2. 如有问题先检查 Kindle 的 `/tmp/kindle-dashboard.log`、服务器版本和请求头，再检查图块坐标、残影及设备当前屏幕方向；只改与反馈相关的文件。

## Key Technical Decisions

- 当前横屏依然长期联网；每分钟服务器仍生成整张图再裁剪时钟，局刷减少设备绘制/闪烁，不代表服务器局部渲染或无线关闭。
- `/screen.png` 默认竖屏，`/screen.png?layout=landscape` 为电脑横屏预览；Kindle 声明 `X-Dashboard-Layout: landscape-v1` 并上报 1072×1448，由服务端预旋转。局刷请求使用 `?part=clock` 或 `?part=weather`，必须校验局刷协议、坐标、日期和小时。
- 启动全刷、日期改变全刷；整点天气局刷；每分钟时钟局刷。失败不清空画面，天气局刷失败会在下一分钟重试。
- `awesome` 在成功取得首张可用图片之后暂停；守护脚本运行在 `/tmp`，异常退出后尝试恢复。期间普通原生 UI 可能不可用；PW3 曾验证五秒内短按电源键四次退出，必要时重启。
- 原竖屏和本地日历入口保留；长天气文案自动缩字，天气数据获取失败保留缓存并标注。

## Gotchas & Traps

**局部刷新与屏幕旋转坐标**  
图块需要从已旋转的 1072×1448 全图裁切，不能直接使用预览图横屏坐标。  
**Solution:** 对照 `landscape_partial.go` 坐标和 `landscape_test.go`；本机已逐像素比对。  
**Lesson:** 设备可能仍用全刷波形，闪烁和残影需要真机测量。

**新客户端与旧服务器不兼容**  
局刷客户端要求协议头，服务器仍旧版时不会误绘或清屏。  
**Solution:** 用户更新 Compose 镜像后重启 Kindle 横屏入口。  
**Lesson:** GitHub 发布镜像不代表服务器容器已更新。

**原生时钟覆盖及界面恢复**  
旧在线版的原生状态栏可重绘到看板上；暂停窗口管理器也影响普通原生 UI。  
**Solution:** 使用 `landscape-ui.sh`、`offline-ui-guard.sh`；实机测试四次短按退出，必要时重启。  
**Lesson:** 异常恢复守护和可中断等待不能省略。

**Windows 测试与设备脚本换行**  
Linux 专用测试无法直接在 Windows Go 下编译；CRLF 可能导致 Kindle shell 执行失败。  
**Solution:** 本机跑适用测试、GitHub Actions 跑 Linux 全量；部署 `.sh` 使用 LF。  
**Lesson:** 区分平台限制和应用错误，复制到 Kindle 后验证内容。

**GitHub 工作流权限**  
当前 OAuth token 曾拒绝修改 `.github/workflows/container.yml`，因为没有 `workflow` scope。  
**Solution:** 沿用现有工作流，本地额外验证横屏及局部图块。  
**Lesson:** CI 的镜像冒烟仅检查默认竖屏，不应声称它检查了局刷图块。

## How to Deploy

用户自行在已有 Compose 目录更新服务端：

```sh
docker compose pull
docker compose up -d
docker compose ps
```

横屏浏览器预览：`http://192.168.1.250:26168/screen.png?layout=landscape`，以实际地址为准。GitHub 构建记录见 [Actions](https://github.com/cn-404-not-found/kindle-dashboard/actions)。

Kindle USB 连接时保持原菜单、`server-url.txt` 和其他扩展；将 `kindle-client/dashboard-extension/landscape-partial.sh`、`start-landscape.sh` 以 LF 复制到 `/mnt/us/extensions/kindle-dashboard/`，并确认 `landscape-ui.sh`、`offline-ui-guard.sh` 和 `server-settings.sh` 存在。`b970b08` 已复制这两个新脚本。安全弹出后停止旧看板，从 KUAL 启动 `Kindle Dashboard → Start landscape online`。若服务器未更新，新客户端保留当前画面并重试。

交接文件及其历史归档未纳入功能提交。`continuation.md` 是已跟踪文件，本次修改及未跟踪的 `continuation.archive.*.md` 均为用户的交接产物；不要顺带提交、覆盖或删除。
