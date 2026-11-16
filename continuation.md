# Continuation: Kindle Dashboard

**Date:** 2026-11-16（Asia/Shanghai）
**Working directory:** `/Users/liangyong/Documents/开发项目/Kindle-dashboard`

## What This Project Is

Go HTTP 服务生成 Kindle 灰度仪表盘，包含时间、天气、农历和月历。用户在 Kindle Paperwhite 上通过 KUAL 客户端运行横屏看板，保持 Wi-Fi 联网并局部刷新，减少整屏闪烁。

## Tech Stack

- Go 1.25，module `kindle-dashboard`；gg、`golang.org/x/image/font/{sfnt,opentype}`、lunar-go。
- Docker Compose；GitHub Actions 测试并构建 linux/amd64、linux/arm64 镜像，发布至 `ghcr.io/cn-404-not-found/kindle-dashboard:latest` 和提交 SHA 标签。
- Kindle Paperwhite 3（PW3，1072×1448）与 Paperwhite 2（PW2，第六代，758×1024）；KUAL、shell、eips、LIPC 和 kterm。
- 官方 `Sarasa-Regular.ttc` 为默认字体，`Sarasa-Bold.ttc` 可选；字体文件不提交。

## How to Build & Run

```sh
go mod download
go test ./...
go build -o kindle-dashboard .
./kindle-dashboard
curl -f -o screen.png http://127.0.0.1:8080/screen.png
curl -f -o landscape.png 'http://127.0.0.1:8080/screen.png?layout=landscape'
```

需要在可执行文件同目录或 `fonts/` 放置官方更纱字体。`go test ./...` 在本次 Mac 和 GitHub Linux 构建中均通过。

## Key Files

| 文件 | 用途 |
| --- | --- |
| `AGENTS.md` | 项目规范：仅改需求相关文件、运行测试、中文提交并推送；字体不入库。 |
| `main.go`、`landscape.go`、`landscape_partial.go` | 请求、横屏渲染和按设备裁剪局部图块。 |
| `weather.go` | 中国天气网获取、启动时更新、每小时半点后台更新、失败重试。 |
| `kindle-client/dashboard-extension/landscape-partial.sh` | PW2/PW3 横屏在线局刷客户端。 |
| `kindle-client/dashboard-extension/menu-pw2.json` | PW2 安装时复制为 `menu.json`；只提供横屏在线、停止和服务器地址设置。 |
| `kindle-client/dashboard-extension/server-settings.sh` | 保存服务器地址；`set-server.sh` 通过 kterm 打开。 |
| `kindle-client/network-settings/` | KUAL 静态 IPv4/DHCP 和 Ping 工具；无 DHCP 环境尚未验证。 |
| `landscape_test.go`、`landscape_partial_client_test.go` | 图块坐标、像素及 PW2/PW3 局刷顺序测试。 |
| `.github/workflows/container.yml` | Linux 测试、双架构镜像构建发布和默认竖屏 PNG 冒烟。 |

## What Was Just Done (Most Recent Session)

1. 用户的 PW2 第六代、固件 `5.12.2.2 (3791510038)` 通过 WinterBreak2 越狱。设备生成 `documents/JAILBROKEN.txt`（`jb.sh v2.6.0`），`;kpm` 运行后生成 KPM 数据库并记录 KPM 0.2.4。官方 WinterBreak2 包来自 GitHub；同型号有组件安装不完整的报告，不仅凭成功文件判断。
2. 通过官方 PEKI 包安装 KUAL；用户确认 KUAL 菜单打开。注意 PEKI 作者对新式越狱环境支持有限，后续排错不要盲目混装旧版 Hotfix/MRPI。
3. `b110917 feat: 适配 PW2 横屏局部刷新` 已推送 `origin/main`：服务端支持 758×1024 局部图块，基于 PW3 坐标按比例向外取整；客户端依据 `eips -i` 选择 PW2/PW3 坐标和尺寸，保留原 PW3 行为。新增 PW2 图块像素及客户端刷新顺序测试。
4. 已将 PW2 版客户端、`kindle-client/network-settings` 和官方 kterm 2.6 的旧架构版本安装至 Kindle；`menu-pw2.json` 复制为设备的 `menu.json`。`server-url.txt` 设置为 `http://192.168.1.250:26168`。用户确认静态 IP 设置界面可打开，但尚未实际配置、测试无 DHCP 网络。
5. 设备检查显示 PW2 `eips -i` 为 758×1024（虚拟宽度 768），电源键 `/dev/input/event0`、触摸屏 `/dev/input/event1`，`awesome`、`wpa_cli`、`ifconfig`、`ip`、`route`、`crond` 可用。测试用临时设备检查工具已删除。
6. [GitHub Actions 43865438563](https://github.com/cn-404-not-found/kindle-dashboard/actions/runs/43865438563) Linux 全量测试、双架构镜像构建发布与默认竖屏 PNG 冒烟通过。用户自行更新服务器后，本机验证 PW2 时钟与天气局部请求返回 200、正确坐标及灰度 PNG。用户确认重启 Kindle 后 KUAL 正常，横屏看板页面正常显示。

## Current State

**Working:**
- `main`、`origin/main` 本次功能提交为 `b110917`，交接文档提交可能使 HEAD 前进；服务器已由用户更新。
- PW2 越狱、KUAL 菜单、网络设置界面及横屏完整画面均有用户现场确认。时钟及天气图块服务端响应正确。
- 横屏在线：启动及跨日全刷，每分钟时钟局刷，整点天气局刷，失败保留画面并后续重试；长期联网，不显示电量/Wi-Fi。服务端启动时获取天气，之后每小时半点主动更新。

**Broken / Known Issues:**
- 用户曾报告 KUAL 一时打不开，重启后恢复；原因不明，勿宣称已根治。
- PW2 真实每分钟局刷、整点天气、闪烁/残影、四次短按退出及异常恢复尚无用户明确反馈。自动化测试和 HTTP 响应不能替代实机确认。
- 静态 IP 工具目前要求先关联目标 SSID；SSID 匹配时每 5 秒重设地址、路由、DNS，并用 cron 保障重启。**无 DHCP 时系统是否维持 Wi-Fi 关联未验证**，不要声称已实现可靠无 DHCP 连接。网络环境实际参数由用户提供。
- 服务器此前访问 GHCR 出现 TLS 超时；本次用户报告容器更新完成，服务器响应已验证。不要假设该网络问题已彻底解决。

**In Progress:**
- 无未提交的功能代码。等待用户观察 PW2 局刷和退出，并在可用的无 DHCP 网络中验证静态 IP。

## Pending Tasks

1. 用户原请求：“我现在手上的 Kindle Paper 2……还要安装那个手动设置IP地址的工具……未来我的 Kindle 会在一个没有 DHCP 的环境当中使用，因此我需要手动设置 IP 地址。” 工具已安装且界面可打开；无 DHCP 环境中的关联、设置、重启/重连、路由和可达性仍需现场验证，必要时只改相关代码。
2. 用户确认“重启之后正常了，现在可以打开正常的看板页面了”。如用户继续测试，确认 PW2 每分钟局刷、整点天气、退出/恢复和残影；异常时检查 `/tmp/kindle-dashboard.log`，不要假设实机已全部通过。
3. 用户自行更新服务器容器；除非另有明确要求，不代替用户部署。

## Key Technical Decisions

- PW2 使用和 PW3 相同的横屏在线刷新策略；不显示电量或 Wi-Fi，保持联网。时钟图块 PW2 `(602,38)`，120×386；天气 `(128,38)`，368×386。PW3 时钟 `(852,55)`、168×544，天气 `(182,55)`、519×544。图块来自设备方向的完整图。
- PW2 的 KUAL 菜单不显示当前仅支持 PW3 的离线模式，避免误启动。PW3 现有菜单和坐标不改变。
- 保留用户认可的横屏天气排版，最高/最低温位于一行居中；不要擅自改回两行。
- 客户端首次完整图绘制前暂停 `awesome`，退出恢复并使用守护进程；五秒内短按电源键四次退出曾在 PW3 验证，PW2 仍需验证。
- 服务器生成完整图再裁图；局刷减少设备绘制，并非停止无线或服务端局部渲染。

## Gotchas & Traps

**PW2 屏幕及裁图**
实际 `eips -i`：`xres=758`、`yres=1024`、`xres_virtual=768`。使用可见尺寸，不要把虚拟宽度当成请求宽度。
**Solution:** 按 `landscape_partial.go` 和 `landscape-partial.sh` 的 PW2 映射请求和验证图块；关注缩放边缘向外取整。
**Lesson:** 服务端新版本和客户端必须同时支持该分辨率。

**旧客户端/服务端不同步**
旧服务端对 PW2 图块返回 HTTP 400；新版返回 200 并附坐标。
**Solution:** 用户更新容器，再测试 `/screen.png?part=clock`，带 `png-width: 758`、`png-height: 1024`、`X-Dashboard-Layout: landscape-v1`。
**Lesson:** GHCR 发布不等于用户服务器已运行新版。

**临时占位文件很慢**
为防 OTA 曾尝试填充 PW2 可用空间；约 3 GB 经 USB 以约 2 MB/s 写入。用户要求停止，进程在 I/O 中延迟退出；最终临时文件已删除、空间恢复，用户选择不填充。越狱期间成功后不要再自动创建大文件。
**Solution:** 尊重用户选择；如有卡住先等待安全结束再弹出，不要直接断开正在写入的设备。
**Lesson:** 长时间占满设备并非用户同意默认做法。

**新版越狱与旧 KUAL**
WinterBreak2 成功标记不保证旧组件齐全；PEKI 安装后用户确实打开 KUAL，但该工具作者提示不适用于所有现代环境。
**Solution:** 按实际症状查看组件/日志；不要盲目安装 Universal Hotfix 或重复越狱。
**Lesson:** 旧教程和新式 KPM 组件不可随意混用。

**无 DHCP 网络限制**
当前 `network.sh` 和 `auto.sh` 依赖 `wpa_state=COMPLETED` 与目标 SSID，一些 Kindle 可能在 DHCP 失败后断开。
**Solution:** 在用户可控的目标网络做现场测试；在确定行为前明确不保证自动重连。
**Lesson:** 界面可打开不等于无 DHCP 连接可用。

**本地交接与配置文件**
有未跟踪的 `config.json`、`continuation.archive.20261115_000954.md`、`continuation.archive.20261116_193301.md`，不能随意提交或删除。此前 Git stash 保存了 `continuation.md` 的本地删除状态；不要随意应用或丢弃。
**Solution:** 提交时只添加本次明确涉及文件；字体和本地配置不入库。
**Lesson:** stash 和未跟踪归档是用户工作状态的一部分。

## How to Deploy

用户自行在服务器现有 Compose 目录运行：

```sh
docker compose pull
docker compose up -d
docker compose ps
```

Kindle 已安装本次客户端，服务器地址为 `http://192.168.1.250:26168`。KUAL → `Kindle Dashboard` → `Start landscape online` 启动，逆时针旋转 90°；`Set server address` 修改服务端地址，独立的 `Wi-Fi IP Settings` 设置设备自身地址。用户希望未来用于没有 DHCP 的网络，设置时需其提供 IP、掩码、网关、DNS 和 Wi-Fi 连接条件，并先确保可恢复。

若重新安装 PW2 客户端：把 `kindle-client/dashboard-extension/` 内容放到 `/mnt/us/extensions/kindle-dashboard/`，用 `menu-pw2.json` 覆盖设备中的 `menu.json`，复制 `kindle-client/TRMNL.sh` 为 `dashboard.sh` 和 `dashboard-landscape.sh`，保留已有 `server-url.txt`。`kindle-client/network-settings/` 放到 `/mnt/us/extensions/kindle-network/`，`/mnt/us/extensions/kterm/` 需匹配 PW2 旧架构。不要将 PW2 菜单误装到 PW3。
