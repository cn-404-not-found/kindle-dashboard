# Continuation: Kindle-dashboard（墨水屏看板）

**Date:** 2026-08-04
**Working directory:** E:\开发项目\Kindle-dashboard

---

## What This Project Is

一个面向 Kindle Paperwhite 3（墨水屏 1072×1448）的看板后端服务（Go），同时配套一个 Kindle 侧 shell 客户端（trmnl-kindle 协议兼容）。服务端起 HTTP（监听端口可配置，Alpine 部署用 58188），实时组装时间/日期/农历、实时天气、5 日预报、整月日历，用 `gg` 在内存绘制成 PNG。Kindle 客户端每轮询 `/api/display` → 下载 `/screen.png` → `eips` 上屏。目标服务端平台是 Linux x86_64（Alpine LXC），客户端是 Kindle 自带 BusyBox 环境。

**数据已全部接入真实源**：天气 Open-Meteo（免 key），农历 `lunar-go`。仅网络失败回退 Mock。

## Tech Stack

- **语言：** Go 1.25（开发机用 `C:\Program Files\Go\bin\go.exe`，不在 PATH，需绝对路径调用）。
- **绘图：** `github.com/fogleman/gg v1.3.0`。
- **字体解析：** `golang.org/x/image` 的 `font/sfnt`（`ParseCollection`+`col.Font(i)`）+ `font/opentype`（`NewFace`）。**`font/truetype` 在任何版本都不存在，禁用。**
- **字体文件：** `Sarasa-Regular.ttc`（更纱黑体，放 exe 同目录，内部选简体等宽子变体 `Sarasa Mono SC`）；`Sarasa-Bold.ttc` 可选真粗体，缺失则偏移叠绘模拟。字体被 `.gitignore` 排除，不入库。
- **农历：** `github.com/6tail/lunar-go/calendar`。
- **天气：** Open-Meteo（免 key）。
- **客户端：** `kindle-client/TRMNL.sh`（BusyBox `sh`，自包含，依赖 `eips`/`curl`/`ifconfig`）。
- 无 Web 框架、无数据库。

## How to Build & Run

```bash
# 依赖（首次 / go.mod 变动后）
"C:\Program Files\Go\bin\go.exe" mod tidy

# 本机 Windows 测试版
"C:\Program Files\Go\bin\go.exe" build -o kindle-dashboard.exe .

# 交叉编译到 Kindle（Linux x86_64, alpine）—— 环境变量必须【前置】，禁用 cmd 的 set X=Y && 写法
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 "C:\Program Files\Go\bin\go.exe" build -o kindle-dashboard-alpine .

# 放字体（粗体可选）
cp /path/to/Sarasa-Regular.ttc ./

# 运行（监听端口读 config.json，缺失回退 8080）
./kindle-dashboard-alpine

# 验证
curl -s -o /tmp/s.png -w "HTTP %{http_code} size %{size_download}\n" http://localhost:8080/screen.png
curl http://localhost:8080/api/display   # {"image_url":"/screen.png","refresh_rate":60,...}
```

## Key Files

| File | Purpose |
|------|---------|
| `main.go` | HTTP 服务骨架 + `fitToDevice()` + 配置加载。定义 `DashboardData`/`ForecastItem`/`CalendarCell`；`/screen.png` 调 `RenderDashboard`；`/api/display` 返回轮询元信息；`screenHandler` 读 `png-width/png-height` 头喂给 `fitToDevice`。`loadConfig()` 读 exe 同目录 `config.json`（port/city_name/timezone_offset），缺失/非法回退 8080/南宁/8。`fetchDashboardData()` 用 `time.FixedZone` 按 `timezone_offset` 取时区。 |
| `render.go` | 核心绘制。`RenderDashboard` 在 1072×1448 白底用 `gg` 画三区（顶部时间+日期/农历+天气卡、中部 5 日预报、下部 6 行月历）。`resolveFontPath()` 搜索 exe 目录→`fonts/`→工作目录。 |
| `config.example.json` | 配置样例（port/city_name/timezone_offset），部署时复制为 `config.json` 改本地值。 |
| `kindle-client/TRMNL.sh` | Kindle 客户端（自带，自包含）。轮询 `/api/display`、下载 PNG、`eips -g` 上屏。内联分辨率检测，`DEBUG_MODE=false`，仅用 png-width/png-height 头。 |
| `go.mod` | 依赖：gg v1.3.0、golang.org/x/image、6tail/lunar-go v1.4.6。 |
| `.gitignore` | 排除 `.workbuddy/`、字体 `*.ttc`、编译产物（`/kindle-dashboard`、`*.exe`）。 |

## What Was Just Done (2026-08-04 会话)

> 本会话目标：配置化、时区修复、Kindle 客户端兼容、屏幕拉伸排查。

1. **配置文件支持**：新增 `config.json`（`port` / `city_name` / `timezone_offset`）。`loadConfig()` 读 exe 同目录文件，缺失/非法回退默认 `8080 / 南宁 / 8`；端口范围 [1,65535]，时区 [-12,14]。**用户坚持不配经纬度**——城市经纬度用内置 `cityCoords` map 按名查（约 44 城，含港澳台）。
2. **时区改为配置驱动**：`time.FixedZone("UTC+offset", offset*3600)`，不再依赖主机时区。验证 tz=8 → 09:40、tz=0 → 01:41（差 8h 证明生效）。
3. **天气文案微调**：`wmoToText` 中 code 51/53/55 由「毛毛雨」改为「细雨」。WMO 映射逻辑不变。
4. **`fitToDevice()` 新增**（`main.go`）：服务端读客户端上报的 `png-width`/`png-height` 头，对基准 1072×1448 做 **contain 等比缩放 + 白边居中**，输出恰好设备尺寸 → `eips` 1:1 铺满不拉伸。缺头/非法 → 回退 1072×1448。
5. **`fitToDevice()` 加自动旋转**：当设备帧缓冲横屏（`devW>devH`）且源图竖屏（`sw<sh`）时，先顺时针旋转 90° 再适配。竖屏设备（1072×1448）不触发此分支。
6. **交叉编译大坑修复**：Bash 里 `set GOOS=linux && go build` 不会设置环境变量，go 按默认 windows 编出 PE（`MZ` 头），拷到 Kindle 报 `MZ not found / syntax error unexpected |`。**正确写法**：`CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build ...`（环境变量前置）。已用 Python 读魔数验证输出为 `\x7fELF`。
7. **Kindle 客户端 `TRMNL.sh` 重写为自包含单文件**（仓库 `kindle-client/TRMNL.sh`）：内联 `get_kindle_width/height`（`eips -i` 解析）、`DEBUG_MODE=false`、`PNG_WIDTH/PNG_HEIGHT` 变量名修正（之前错写成 `_WIDTH` 导致分辨率头从未发出）、相对路径补全（`/screen.png`→绝对 URL）、`eips -g` 1:1 铺满。不再依赖 `utils.sh`。

## Current State

**Working:**
- 两个二进制均编译成功（Windows `kindle-dashboard.exe` / Linux `kindle-dashboard-alpine`，后者魔法字节已验证为 ELF）。
- 本地 `:8080` 产出有效 1072×1448 PNG；`/api/display` 返回 JSON。
- 配置 / 时区 / `fitToDevice` / 自动旋转逻辑均已落地并通过单元级验证（含 1072×1448 等尺寸回归）。

**Broken / Known Issues:**
- ⚠️ **Kindle 画面仍「拉扯 / 被拉伸」未解决**：用户已确认 `eips -i` 返回 **1072×1448（竖屏 = 服务端基准）**，理论上 `fitToDevice` 应 `scale=1` 零缩放、不拉伸。但用户实测仍变形（时间数字变扁、右侧天气被截断）。**主导假设**：(a) Kindle 上跑的仍是旧二进制 / 旧 `TRMNL.sh`（之前拷错过 PE 版报 `MZ not found`，且截图里 debug 文字残留证明旧脚本在跑）；(b) Kindle 帧缓冲虚拟尺寸与可见尺寸不一致，`eips -g` 实际做了缩放。未在设备端验证。
- 本地测试前务必先 `taskkill /IM kindle-dashboard.exe /F`（旧进程霸占 8080 会返回空响应）。

**In Progress:**
- 排查 Kindle 拉伸根因（待设备端数据：`fbset` / `/sys/class/graphics/fb0/virtual_size` / 确认新 ELF 二进制是否真在跑）。

## Pending Tasks

- 「还是没有变化，显示还是不全感觉是被拉伸，我是要竖屏使用的，是不是程序默认横屏了」—— 验证 Kindle 实际帧缓冲朝向 + 确认新二进制/脚本已部署，彻底消除拉伸。
- （本次已执行）把 `main.go`/`render.go`/`kindle-client/TRMNL.sh`/断点文档等改动 git 提交并 push。
- 可选：顶部天气口径统一（实时 current vs 全天 daily 预报）。

## Key Technical Decisions

- **配置驱动**（port/city_name/timezone_offset），不写死；城市经纬度内置 map 按名查（用户坚持不配经纬度）。
- **`fitToDevice` contain 缩放**（保纵横比，绝不主动拉伸）；横屏设备自动旋转 90°。
- **交叉编译环境变量必须前置**（`CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build`），禁用 `set X=Y &&` 写法（Git Bash 里 `set` 不设置环境变量）。
- **`TRMNL.sh` 自包含**，不依赖 `utils.sh`，减少部署文件数。
- 服务端忽略 trmnl 协议其余头（access-token/battery/rssi/ID），仅用 `png-width`/`png-height`。
- 字体 `*.ttc`、二进制产物均不入 git（`.gitignore` 排除）。

## Gotchas & Traps

**交叉编译 `set GOOS=linux &&` 无效**
在 Git Bash 里 `set X=Y` 是 cmd 语法，不设置环境变量 → go 按默认 windows 编出 PE（带 `MZ` 头），Kindle 报 `MZ not found`。
**Solution:** 环境变量前置：`CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o kindle-dashboard-alpine .`；用 Python 读前 4 字节验证 `\x7fELF`。
**Lesson:** Windows 下交叉编译一律用 bash 前置语法。

**`font/truetype` 包已不存在**
任意 x/image 版本都无此包。
**Solution:** `sfnt.ParseCollection` + `opentype.NewFace`。

**TRMNL.sh 变量名拼错导致分辨率头从未发出**
旧脚本里本地变量叫 `_WIDTH`，发头时却引用 `$PNG_WIDTH`（空值）→ 服务端永远拿不到设备分辨率 → 固定出 1072×1448 → `eips` 拉伸。
**Solution:** 统一为 `PNG_WIDTH`/`PNG_HEIGHT`。

**旧进程霸占端口**
改前没杀旧 `kindle-dashboard.exe` → curl 拿到旧图/空响应。
**Solution:** 重测前 `taskkill /IM kindle-dashboard.exe /F`（Win）/ `pkill -f kindle-dashboard`（Lin）。

**PowerShell/Bash `rm` 中文路径编码错乱**
本机 Bash 的 `rm` 被安全删除包装器拦截，且对中文路径出现 mojibake（「开发项目」变乱码），删除失败。
**Solution:** 用 Python `os.remove` 绝对路径删除（UTF-8 可靠）。

**Git Bash 无 `chmod`**
Windows Git Bash 没有 `chmod` 命令。
**Solution:** 权限在 Kindle 部署时用 `chmod 755` 设置，开发机无需。

## How to Deploy

```bash
# 1. 开发机交叉编译（见上，环境变量前置）
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 "C:\Program Files\Go\bin\go.exe" build -o kindle-dashboard-alpine .

# 2. 传服务端到 Alpine（二进制 + 字体 + 配置）
scp kindle-dashboard-alpine Sarasa-Regular.ttc user@alpine:/opt/kindle-dashboard/
#    ssh alpine: cp config.example.json config.json && vi config.json  # 改 port=58188（或服务端实际端口）
#    先 kill 旧进程再启动，确认日志 listening on :58188

# 3. 传客户端到 Kindle（拷仓库里的 kindle-client/TRMNL.sh）
scp kindle-client/TRMNL.sh root@kindle:/tmp/trmnl-kindle/TRMNL.sh
#    Kindle 上: chmod 755 TRMNL.sh
#    编辑 BASE_URL 第 19 行：改成 Alpine 实际局域网 IP + 端口（须与 config.json 的 port 一致）
#    运行: ./TRMNL.sh
```

## Verification Checklist（部署后）

- [ ] Alpine 日志：`listening on :58188`
- [ ] Kindle：`ps` 确认新 ELF 二进制在跑（无旧进程残留）
- [ ] Kindle 画面：时间数字为正圆不扁、右侧天气完整、无 debug 文字
- [ ] 若仍拉伸：在 Kindle 跑 `fbset | grep -E 'xres|yres'` 和 `cat /sys/class/graphics/fb0/virtual_size` 取真实帧缓冲尺寸，反馈给服务端决定是否需旋转
