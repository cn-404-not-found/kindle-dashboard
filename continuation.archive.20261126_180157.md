# Continuation: Kindle Dashboard

**Date:** 2026-11-20（Asia/Shanghai）
**Last update:** 2026-11-23（用户实机验收：无 DHCP 通过、整套看板运行正常）
**Working directory:** `E:\开发项目\Kindle-dashboard`（Windows；本会话在 Windows 上完成）
**Git HEAD:** `770c6ee docs: 记录夜间 Wi-Fi 自愈已实机验证`（已推送 `origin/main`；Wi-Fi 自愈提交为 `e7fb8ee`）

## What This Project Is

Go HTTP 服务为越狱 Kindle 生成灰度仪表盘，显示时间、天气、农历和月历。Kindle 通过 KUAL 客户端经局域网取图；横屏在线模式保持联网并局部刷新时钟/天气，减少整屏闪烁。项目同时支持 Paperwhite 3（1072×1448）和 Paperwhite 2（758×1024）。

## Tech Stack

- Go 1.25，module `kindle-dashboard`；`github.com/fogleman/gg`、`golang.org/x/image/font/{sfnt,opentype}`、`github.com/6tail/lunar-go`。
- Docker Compose；GitHub Actions 运行 Linux 全量测试、构建 linux/amd64 与 linux/arm64 镜像，发布到 `ghcr.io/cn-404-not-found/kindle-dashboard:latest` 及提交 SHA 标签。
- Kindle：KUAL、POSIX shell（ash）、`eips`、LIPC、kterm；PW3 触摸 `/dev/input/event1`、电源键 `/dev/input/event0`，PW2 已确认同样的输入设备路径。
- 字体：官方 `Sarasa-Regular.ttc`（默认）和可选 `Sarasa-Bold.ttc`，均不入库。

## How to Build & Run

Linux / CI：

```sh
go mod download
go test ./...
go build -o kindle-dashboard .
./kindle-dashboard
curl -f -o screen.png http://127.0.0.1:8080/screen.png
curl -f -o landscape.png 'http://127.0.0.1:8080/screen.png?layout=landscape'
```

本机（Windows）没有 `go` 在 PATH，使用临时工具链 `go1.25.9`：

```sh
GO="/d/Personal/Temp/kindle-preview-go/go/bin/go.exe"
```

Windows 只能运行可移植测试；`landscape_partial_client_test.go`（`!windows`）、`dashboard_power_test.go` 等依赖 Linux syscall/FIFO，交给 GitHub Actions。当前可用的 Windows 子集：

```sh
src=$(ls *.go | grep -v '_test\.go$')
"$GO" test -count=1 $src network_health_test.go landscape_test.go weather_test.go \
  calendar_test.go local_calendar_test.go display_test.go
```

Git 目录归属另一 Windows SID，所有 git 命令加：

```sh
git -c safe.directory='E:/开发项目/Kindle-dashboard' ...
```

`gh` 需指定 `--repo cn-404-not-found/kindle-dashboard`。

## Key Files

| 文件 | 用途 |
| --- | --- |
| `AGENTS.md` | 项目规范：只改需求相关文件、补测试、中文提交并推送；字体不入库。 |
| `main.go` | HTTP 路由、数据组装、设备旋转与局刷协议、天气后台更新。 |
| `landscape.go` / `landscape_partial.go` | 横屏渲染；按设备分辨率裁切时钟/天气图块。 |
| `weather.go` | 中国天气网获取、缓存、每小时半点主动更新与失败重试。 |
| `kindle-client/dashboard-extension/landscape-partial.sh` | PW2/PW3 横屏在线局刷客户端主循环。 |
| `kindle-client/dashboard-extension/network-health.sh` | **本会话新增**：链路级 Wi-Fi 健康检查与分级恢复。 |
| `kindle-client/dashboard-extension/power.sh` | 防屏保、前光控制、看板异常重启与退出恢复。 |
| `kindle-client/dashboard-extension/menu-pw2.json` | PW2 菜单（只含横屏在线、停止、服务器地址）；PW2 设备上它被复制为 `menu.json`。 |
| `kindle-client/dashboard-extension/menu.json` | PW3/完整菜单（含离线与在线模式），**不要装到 PW2**。 |
| `kindle-client/network-settings/` | 静态 IPv4/DHCP 与 Ping 工具；无 DHCP 环境仍未验证。 |
| `network_health_test.go` | **本会话新增**：Wi-Fi 恢复阶梯的 shell 单元测试。 |
| `landscape_partial_client_test.go` | PW2/PW3 刷新顺序测试（`!windows`，Linux CI 执行）。 |
| `.github/workflows/container.yml` | Linux 测试、双架构镜像发布、默认竖屏 PNG 冒烟。 |

## What Was Just Done (Most Recent Session)

1. 用户报告 Kindle **每天晚上自动断开 Wi-Fi**、之后一直连不回来；确认设备设置里没有 Power Saver 选项，要求在**代码层面**保证 Wi-Fi 持续连接。
2. 定位原因：Kindle 空闲/休眠时会关闭射频；原客户端每分钟只重试 `curl`，**没有任何 Wi-Fi 自愈**，所以夜里掉线后永远不会恢复。原 `power.sh` 只设置了一次 `preventScreenSaver 1`。
3. 新增 `kindle-client/dashboard-extension/network-health.sh`：
   - `network_online`：`wirelessEnable=1` + `wpa_state=COMPLETED` + `wlan0` 有 IPv4 才算在线，健康时只读。
   - `network_ensure_online`：电台被关 → 重新打开；掉线 → `wpa_cli -i wlan0 reassociate`；仍未恢复 → 无线开关 `0→1`；每级等待 20 秒，失败下轮重试。
   - `network_keep_awake`：每轮重申 `preventScreenSaver 1`，避免 powerd 重置后休眠关射频。
   - `KINDLE_WIFI_WATCHDOG=0` 可关闭整套行为。
4. 修改 `landscape-partial.sh`：source `network-health.sh`，在每轮 `curl` 前调用 `network_keep_awake` + `network_ensure_online`。
5. 测试与文档：新增 `network_health_test.go`（健康链路、reassociate 恢复、电台重启恢复 3 个用例）；`landscape_partial_client_test.go` 增加 health 模块 stub；更新根 `README.md` 和客户端 `README.md`。
6. 提交 `e7fb8ee fix: 横屏在线模式自动恢复断开的 Wi-Fi` 并推送。推送前远端已有 `b110917 feat: 适配 PW2` 和 `6725bc2 docs: 更新并归档交接文档`，已 rebase；客户端 README 冲突合并为「保留 PW2 支持 + 新增 WiFi 自愈说明」。
7. [GitHub Actions 44437722106](https://github.com/cn-404-not-found/kindle-dashboard/actions/runs/44437722106) **success**：Linux 全量测试、双架构镜像、PNG 冒烟均通过。
8. 已把更新写入用户接上的 Kindle（Windows 盘符 `G:`）：从 `git show HEAD:…` 写入 `network-health.sh`、`landscape-partial.sh`、客户端 `README.md`，校验 CR=0、与 HEAD 字节一致；保留 `server-url.txt` = `http://192.168.1.250:26168`。设备原 `landscape-partial.sh` 与 `HEAD~1` 完全一致，说明只差本次改动。
9. 设备身份判断：其 `menu.json` 与仓库 `menu-pw2.json` 内容一致（无离线/在线模式项），`menu-pw2.json` 也在，服务器地址与 2026-11-16 的 PW2 会话记录一致；因此本次更新的是 **PW2**，不是 PW3。未在会话中直接读取 `eips` 分辨率。
10. 复制完成并 `sync` 后，设备在后续检查期间从 `G:` 卸载（可能被用户安全弹出或 USB 断开）。文件已在卸载前校验通过，但不能假设设备当前仍挂载。
11. 回答用户：**服务器端不需要更新**——本次只改客户端脚本，服务端接口/局刷协议/渲染均未变。
12. 2026-11-20 用户反馈：更新后**夜间已不再掉 Wi-Fi**。此前"待观察"的自愈修复通过实机过夜验证。
13. 2026-11-23 用户反馈：**无 DHCP 环境测试通过**，设备不再掉线，整套看板运行正常；此前遗留的 PW2 实机验证项一并关闭。

## Current State

**Working:**
- Wi-Fi 自愈代码已提交、推送，CI 全绿。
- **夜间掉线已验证修复**（2026-11-20 用户反馈过夜不再断网）：每分钟重申 `preventScreenSaver 1` 阻止休眠关射频；掉线时按「开电台 → reassociate → 电台重启」自愈。
- **无 DHCP 静态 IP 环境已验证通过**（2026-11-23 用户反馈）。
- **整机运行正常**（2026-11-23 用户反馈）：不再掉线，日常看板作业均正常。
- PW2 设备上的 `network-health.sh` / `landscape-partial.sh` / `README.md` 已写入并校验（LF、匹配 HEAD）；`server-url.txt` 未动。
- 健康链路下行为不变：检查只读，不写 Wi-Fi 设置；刷新坐标与 PW2/PW3 布局未改。

**Broken / Known Issues:**
- 自愈是链路级的；路由器夜间重启、上游故障不在客户端能力范围内。若整机被 powerd 强制挂起，进程无法运行，只能靠每分钟重申防屏保缓解。
- 未核实具体由哪一级恢复生效：用户未提供 `/tmp/kindle-dashboard.log`，日志中是否出现 `Wi-Fi offline; attempting recovery.` 不明。最可能的主因是 `network_keep_awake`（防休眠关射频）。
- 设备当前未挂载在本机。
- PW2 真机每分钟局刷、整点天气、闪烁/残影、四次短按退出：2026-11-23 用户确认整体运行正常。
- KUAL 曾偶发打不开（原因不明，近期未复现）；GHCR/TLS 超时问题不要假设已根除。
- PW2 的 `menu.json` 是**有意**的最小菜单，不要用仓库的完整 `menu.json` 覆盖。

**In Progress:**
- 无。夜间掉线、无 DHCP 环境、PW2 整机运行均已由用户实机确认正常（2026-11-23）；若以后再次掉线，再收集 `/tmp/kindle-dashboard.log` 断网前后日志，区分省电（`wirelessEnable=0`）与关联/路由器问题。

## Pending Tasks

1. 已完成：PW2 文件已更新，用户已在 KUAL 启动 `Start landscape online` 并过夜观察。
2. 已完成：用户反馈夜间不再掉 Wi-Fi（2026-11-20），暂无需收集日志。若复发，再看 `/tmp/kindle-dashboard.log` 中是否有 `Wi-Fi offline; attempting recovery.` / `Reassociate failed; toggling the radio.`。
3. 服务器端由用户自行更新；本次无需更新。除非用户明确要求，不代替部署。
4. 已完成：无 DHCP 网络静态 IP 验证通过（2026-11-23）；PW2 实机整体运行正常，此前局刷/残影/退出验证项一并关闭。KUAL 偶发打不开保留为低优先级观察项。
5. 不要同步 PW2 的 `menu.json` 为完整菜单。

## Key Technical Decisions

- **Wi-Fi 自愈放在客户端主循环**，不新增独立守护进程：每轮刷新前先做轻量检查，健康时无写操作；恢复按「开电台 → reassociate → 电台重启」逐级升级，避免每分钟无意义地切射频。
- 每轮重申 `preventScreenSaver 1`：`power.sh` 只在启动/重启看板时设置一次，若被 powerd 重置，设备休眠会直接关射频。
- 该组合已由用户过夜实机验证有效（2026-11-20）；`KINDLE_WIFI_WATCHDOG=0` 可整体关闭，排查时优先读日志判断触发到哪一级。
- 使用 `com.lab126.cmd wirelessEnable` 作为电台开关、`wpa_cli -i wlan0 status` + `ifconfig wlan0` 判定连通；这些与离线模式 `offline-network.sh` 已验证的做法一致。
- 本次**不改服务器**：协议和渲染未变；PW2 所需的动态 `png-width`/`png-height` 适配已在 `b110917` 完成。
- PW2 使用 `menu-pw2.json` 作为设备 `menu.json`；PW3 使用完整 `menu.json`，两者不要混装。
- 保留用户认可的横屏天气排版（最高/最低温单行居中），不要重排。
- 部署客户端脚本时从 `git show HEAD:path` 取内容，而不是直接复制工作区文件（见 Gotchas 的 CRLF 问题）。

## Gotchas & Traps

**工作区 CRLF 与提交 LF 不一致**
Windows 工作区里许多 `kindle-client/dashboard-extension/*` 文件实际含 CRLF（例如 `stop.sh` 有 7 个 CR），尽管 `.gitattributes` 声明 `eol=lf`；但 `git show HEAD:path` 的提交内容全部是 LF（已逐个验证 CR=0）。直接把工作区文件复制到 Kindle 可能带入 CRLF，导致 ash 脚本异常。
**Solution:** 部署时用 `git show HEAD:path > /g/...`，复制后执行 `tr -cd '\r' | wc -c` 确认 CR=0，再 `sync`。
**Lesson:** `.gitattributes` 只约束 Git 检出/提交，不保证当前工作区字节。

**Git 目录所有权**
本机仓库归属另一个 Windows SID，普通 `git` 报 `dubious ownership`。
**Solution:** 每条命令加 `-c safe.directory='E:/开发项目/Kindle-dashboard'`；`gh` 加 `--repo`。
**Lesson:** 不要改全局 Git 信任设置。

**设备身份与菜单**
本次挂载的 `G:` 是 **PW2**（`menu.json` 与 `menu-pw2.json` 一致、服务器地址与 PW2 会话一致）。若误把仓库完整 `menu.json` 写入，会暴露 PW3 专用的离线/在线模式。
**Solution:** PW2 始终使用 `menu-pw2.json`；更新时不要覆盖 `menu.json`。
**Lesson:** 先核对 `menu.json` 内容再判断设备，不要凭想象。

**Windows 无法跑全部测试**
`landscape_partial_client_test.go` 是 `!windows`；`dashboard_power_test.go` 用 `syscall.Mkfifo`。本机只能跑可移植子集。
**Solution:** 本机跑上面的 Windows 子集；用带 stub 的手工 shell 冒烟验证 PW2/PW3 坐标；全量交给 GitHub Actions。
**Lesson:** 区分平台限制和真实回归。

**远端分叉**
推送时远端已有 PW2 与文档提交，直接 push 被拒。
**Solution:** `git fetch` → `git rebase origin/main` → 解决 `kindle-client/dashboard-extension/README.md` 冲突（保留 PW2 文本 + WiFi 自愈文本）→ `git push`。
**Lesson:** 多会话/多机器并行时先 fetch 再推送。

**设备会在检查中途卸载**
USB 模式下 `G:` 可能在无操作后断开/被弹出，`ls /g` 突然报不存在。
**Solution:** 写完后立即校验并 `sync`；不要假设挂载一直存在；重新挂载后再检查。
**Lesson:** 不要把“路径突然消失”误判为文件被删除。

**自愈的边界**
Wi-Fi watchdog 只恢复链路（电台、关联、IPv4），不修复路由器夜间重启、上游故障或服务端不可达。
**Solution:** 先看日志与 `wirelessEnable`/`wpa_state`；再决定是否查路由器。
**Lesson:** 客户端自愈不等于网络环境稳定。

## How to Deploy

服务器（用户自行操作，本次不需要）：

```sh
docker compose pull
docker compose up -d
docker compose ps
```

Kindle 客户端更新（本次已对 PW2 执行；重做时）：

```sh
G="git -c safe.directory=E:/开发项目/Kindle-dashboard"
$G show HEAD:kindle-client/dashboard-extension/network-health.sh \
  > /g/extensions/kindle-dashboard/network-health.sh
$G show HEAD:kindle-client/dashboard-extension/landscape-partial.sh \
  > /g/extensions/kindle-dashboard/landscape-partial.sh
$G show HEAD:kindle-client/dashboard-extension/README.md \
  > /g/extensions/kindle-dashboard/README.md
sync
```

然后安全弹出 `G:`，在 KUAL 选择 `Kindle Dashboard` → `Start landscape online`。PW2 设备当前服务器地址为 `http://192.168.1.250:26168`，不要覆盖 `server-url.txt`。PW2 重装时用 `menu-pw2.json` 作为 `menu.json`；PW3 才用完整 `menu.json`。

日志：`/tmp/kindle-dashboard.log`。正常恢复会打印 `Wi-Fi offline; attempting recovery.`、`Reassociate failed; toggling the radio.` 等。
