# Continuation: Kindle Dashboard

**Date:** 2026-11-13（Asia/Shanghai）
**Working directory:** `E:\开发项目\Kindle-dashboard`

---

## What This Project Is

Go HTTP 服务为已越狱 Kindle Paperwhite 3 生成灰度仪表盘图片，展示日期、农历、节气、时间和中国天气网数据。Kindle 通过 KUAL 运行 shell 客户端，在局域网从服务端下载图片并绘制；也保留了独立本地日历模式。

## Tech Stack

- Go 1.25，module `kindle-dashboard`；`github.com/fogleman/gg`、`golang.org/x/image/font/{sfnt,opentype}`、`github.com/6tail/lunar-go`。
- 服务端：Docker Compose；GitHub Actions 构建并发布 GHCR 多架构镜像 `ghcr.io/cn-404-not-found/kindle-dashboard:latest`（linux/amd64、linux/arm64）。
- Kindle：Paperwhite 3（1072×1448，固件 5.16.2.1.1），KUAL、`eips`、LIPC、shell、FBInk。字体为官方 `Sarasa-Regular.ttc`、可选 `Sarasa-Bold.ttc`，不入库。
- 本机 Windows PowerShell；Go 1.25.9 的临时安装为 `D:\Personal\Temp\codex-go1259\go\bin\go.exe`，该路径是临时目录，可能被清除。

## How to Build & Run

Linux/有 Go 的系统：

```sh
go mod download
go test ./...
go build -o kindle-dashboard .
./kindle-dashboard
curl -f -o screen.png http://127.0.0.1:8080/screen.png
curl -f -o landscape.png 'http://127.0.0.1:8080/screen.png?layout=landscape'
```

需将官方更纱 `Sarasa-Regular.ttc` 和可选 `Sarasa-Bold.ttc` 放在可执行文件旁或其 `fonts/` 目录。Docker 构建会自动下载并校验字体。Windows 全量测试有 Linux 专用 `syscall` 测试，不能直接用 Windows Go 跑；GitHub Actions Linux 的 `go test ./...` 已通过。Windows 只验证纯 Go 测试的示例：

```powershell
$goFiles = @(Get-ChildItem -Name *.go | Where-Object { $_ -notlike '*_test.go' })
$goFiles += @('landscape_test.go','weather_test.go','calendar_test.go','local_calendar_test.go','display_test.go')
& "$env:TEMP\codex-go1259\go\bin\go.exe" test @goFiles
```

## Key Files

| 文件 | 用途 |
| --- | --- |
| `AGENTS.md` | 项目约束；用户不手工改代码、测试并提交推送、不要改无关内容或提交字体。部分 Mock 描述过时，当前天气和农历是真实数据。 |
| `main.go` | HTTP 接口、数据组装、五日预报和设备分辨率/旋转适配。 |
| `landscape.go` | 1448×1072 横屏画面；五日卡片字号与自动适配。 |
| `render.go` | 旧竖屏渲染、字体加载、`fitText`。 |
| `weather.go` | 中国天气网数据抓取、解析与缓存，通常一小时更新，跨日重新获取。 |
| `local_calendar.go` | 独立本地日历、农历和节气计算。 |
| `kindle-client/TRMNL.sh` | 在线客户端；安装到设备的横屏副本叫 `dashboard-landscape.sh`。 |
| `kindle-client/dashboard-extension/landscape-ui.sh` | 横屏客户端暂停/恢复窗口管理器；`offline-ui-guard.sh` 为独立异常恢复守护。 |
| `kindle-client/dashboard-extension/start-landscape.sh` | KUAL 横屏启动入口。 |
| `landscape_ui_test.go` | Linux 下验证窗口管理器正常退出、异常退出后的恢复。 |
| `.github/workflows/container.yml` | `main` 推送触发完整测试、构建发布镜像和默认竖屏 PNG 冒烟测试。 |
| `DOCKER.md`, `compose.yaml` | 镜像部署说明和配置。 |

## What Was Just Done (Most Recent Session)

1. 用户反馈本地日历模式耗电严重，决定回到在线模式并改为横屏。先通过 HTML/PNG 预览迭代确定布局：左侧居中时间、公历/农历、当天气温与天气、明天起五日预报、生肖/节气；右侧放大月历，今天黑底。
2. `06a917c feat: 添加横屏在线仪表盘`：服务端支持 `?layout=landscape` 和请求头 `X-Dashboard-Layout: landscape-v1`，横屏 1448×1072；当设备上报 1072×1448 时服务端将图片旋转后输出，设备逆时针转 90°观看。原 `/screen.png` 保持竖屏兼容。时间每分钟刷新，天气缓存一小时，五日从明天开始，缺失数据用 `--`。GitHub Actions 成功构建、发布并冒烟测试。
3. Kindle 已通过 USB 安装独立 `Start landscape online` 入口、`dashboard-landscape.sh`，保留旧菜单和服务器地址；用户已确认原横屏版在 Kindle 正常显示，但原生系统时钟仍覆盖画面，五日预报字体太小。
4. 用户接受先出的调整预览并要求“服务器和Kindle端都帮更新”；用户随后明确选择“我自己更新服务器”。`3f8aca2 fix: 屏蔽横屏状态栏并放大天气预报` 已推送 `origin/main`：五日星期/天气/温度放大加粗，长内容 `fitText` 自动缩小；横屏客户端首张图下载成功且服务端确认支持布局后暂停 `awesome`，`eips -c` 再画全屏；正常退出恢复，守护脚本在进程异常死亡后尝试恢复。为避免 `sleep` 令退出信号长时间延后，横屏等待使用可中断的后台 sleep。
5. USB Kindle 当时为 `G:`，已经写入新版 `dashboard-landscape.sh` 和 `landscape-ui.sh`，经字节比较和 LF 换行校验；原客户端备份在设备 `G:\extensions\kindle-dashboard\dashboard-landscape.before-statusbar.sh.bak`。服务端本地生成图已检查；GitHub Actions [43393457541](https://github.com/cn-404-not-found/kindle-dashboard/actions/runs/43393457541) 全量测试（包括正常/异常恢复）及镜像发布/冒烟测试通过。

## Current State

**Working:**
- `main` 与 `origin/main` 在本交接编写前均为 `3f8aca2`；GitHub Actions 已成功发布该提交镜像。
- 用户曾确认 `06a917c` 横屏版在 Kindle 显示正常；本机 HTTP 渲染的横屏图是 1448×1072 8 位灰度，Kindle 帧缓冲输出为 1072×1448。
- 截至安装时 Kindle 新版客户端文件已写入并校验；旧菜单及设备 `server-url.txt` 保留。

**Broken / Known Issues:**
- 上一版横屏原生时钟覆盖和五日文字偏小已提交修复，但 **`3f8aca2` 在 Kindle 上是否成功隐藏时钟、字体大小与退出恢复均尚无用户实测反馈**。
- 用户自己更新服务器；**没有确认其已拉取或运行 `3f8aca2` 镜像**。上次已知访问地址 `http://192.168.1.250:26168`；不要假设该地址或端口永远不变。
- 在线与本地模式均请求阻止自动屏保，尚未实现、验证深度休眠和 RTC 定时唤醒；在线模式并不保证比本地模式省电。用户反映本地模式耗电严重，未有量化数据。
- 实际天气现象较长时会自动缩小字号；最低/最高预报与实况取自不同来源，可能出现实况温度高于预报最高值。

**In Progress:**
- 等用户自行更新服务器并重新启动 Kindle 横屏模式，然后观察原生时钟遮盖是否消失、预报是否清晰、四次短按退出及退出后原生界面是否恢复。跨时更新和长期耗电仍需实测。

## Pending Tasks

1. 用户的原请求：“可可以接受这个方案，服务器和Kindle端都帮更新”。代码、Kindle 文件和 GitHub 镜像已完成；服务器端用户明确答复“我自己更新服务器”，实际更新及 Kindle 上修复效果尚待用户确认。
2. 若用户反馈问题，优先检查 `/tmp/kindle-dashboard.log` 和进程/窗口管理器状态，复现后只修改本次问题相关代码；不要声称真机已通过尚未进行的测试。

## Key Technical Decisions

- 在线横屏为独立 KUAL 入口，保留旧竖屏与本地日历。Kindle 请求头声明 `landscape-v1`；若服务器不支持则保留当前画面并重试，不误显示旧版。
- `/screen.png` 默认仍为竖屏，电脑横屏预览使用 `/screen.png?layout=landscape`。客户端上报设备尺寸，服务端旋转 PNG，**不依赖 Kindle 自身旋转功能**。
- 系统窗口管理器只在横屏服务端图片下载并确认成功后暂停；守护进程由 `/tmp` 副本运行以支持 USB 状态变化。退出时恢复；显示期间普通原生 UI 可能不可用，已知 PW3 可 5 秒内短按电源键 4 次退出，必要时重启。
- 服务端按成功获取的时刻缓存天气一小时，跨日优先重新获取；失败保留旧数据并标注，未来五日从次日开始，数据不足不伪造。

## Gotchas & Traps

**屏幕上出现原生时钟**  
旧横屏在线客户端未暂停 `awesome`，系统状态栏可能重画覆盖图像。  
**Solution:** 新横屏客户端引入 `landscape-ui.sh` 和恢复守护；先验证实际设备。  
**Lesson:** 局部清屏不能假定能阻止系统重绘；暂停窗口管理器需确保退出和异常恢复。

**USB 文件换行及旧配置**  
Windows 的 CRLF 脚本可能在 Kindle shell 出错，设备中还存在用户定制菜单项。  
**Solution:** 部署 `.sh` 时写 LF；增量更新目标脚本，不覆盖 `server-url.txt`、旧菜单或其他扩展。  
**Lesson:** 在设备上比对脚本内容并保留旧版备份。

**GitHub 工作流权限**  
曾尝试修改 `.github/workflows/container.yml` 增加横屏冒烟检查，push 被拒：当前 OAuth token 没有 `workflow` scope。  
**Solution:** 保持工作流原样，使用本地横屏 HTTP 冒烟测试，GitHub 保留原全量测试和竖屏镜像冒烟。  
**Lesson:** 不要假定可修改工作流或声称 CI 已包含横屏 PNG 检查。

**Windows 上无法全量 Go 测试**  
已有测试使用 Linux 专用 `syscall.Mkfifo`、`SIGSTOP`/`SIGCONT`。  
**Solution:** Windows 跑适用的测试及 shell 语法检查；以 GitHub Actions Linux 的 `go test ./...` 作为全量结果。  
**Lesson:** 区分平台限制和产品失败。

**横屏路径和设备旋转**  
默认 `/screen.png` 仍是竖屏；浏览器横屏 URL 要加 `?layout=landscape`。  
**Solution:** Kindle 客户端通过请求头选横屏并上传设备尺寸，服务端预旋转。  
**Lesson:** 不应把浏览器预览尺寸与 Kindle 输出的纵向帧缓冲尺寸混为一谈。

**系统与服务器端更新独立**  
GitHub 自动发布镜像并不会自动更新用户服务器上的容器。用户选择自己更新服务器。  
**Solution:** 镜像成功后请用户在已有 Compose 目录运行下面的命令；不要假定已部署。  
**Lesson:** 记录构建、设备复制、服务器部署、真机确认四种不同状态。

## How to Deploy

推送相关文件到 `main` 后，检查 [Actions](https://github.com/cn-404-not-found/kindle-dashboard/actions) 对应提交完整通过。用户自行登录已部署 Compose 的服务器并运行：

```sh
docker compose pull
docker compose up -d
docker compose ps
```

横屏预览：`http://192.168.1.250:26168/screen.png?layout=landscape`（服务器地址以实际设备配置为准）。

Kindle 更新时 USB 连接并确认设备盘符；保留地址配置和用户其他菜单。将 `kindle-client/TRMNL.sh` 以 LF 内容复制到 `/mnt/us/extensions/kindle-dashboard/dashboard-landscape.sh`，并复制 `kindle-client/dashboard-extension/landscape-ui.sh`、确认现有 `offline-ui-guard.sh`；需更新其他入口时参阅客户端 README。安全弹出后先停旧看板，再在 KUAL 选 `Kindle Dashboard → Start landscape online`。当前版本复制操作已完成，但运行效果待真机确认。

---

交接文件和历史归档按项目规范不应顺带纳入功能提交；本次 handoff 先归档旧文件再写入，检查 Git 状态时注意 `continuation.md` 为已跟踪文件，归档文件可能未跟踪，均属本次交接产物。
