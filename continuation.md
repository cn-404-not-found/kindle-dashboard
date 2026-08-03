# Continuation: Kindle-dashboard（墨水屏看板）

**Date:** 2026-08-03
**Working directory:** /Users/liangyong/Documents/Git开发项目/Kindle-dashboard

---

## What This Project Is

一个面向 Kindle Paperwhite 3（1072×1448 墨水屏）的看板后端服务。用 Go 起 HTTP 服务（监听 `:8080`），每次请求实时组装数据（时间/日期/农历、实时天气、5 日预报、整月日历），调用 `gg` 库在内存绘制成 PNG，墨水屏设备每 60s 轮询拉取 `/screen.png` 展示。目标部署平台是 Kindle 侧 Linux x86_64（Alpine LXC）。

**当前数据已全部接入真实源**（不再有 Mock 占位）：天气来自 Open-Meteo 免 key 接口，农历来自 `lunar-go` 库。Mock 仅作为网络失败时的兜底。

## Tech Stack

- **语言：** Go（本机 1.26.5；标准库 `net/http`、`image/png`、`time`、`encoding/json`）
- **绘图：** `github.com/fogleman/gg v1.3.0`（2D 画布）
- **字体解析：** `golang.org/x/image v0.44.0` 的 `font/sfnt`（`ParseCollection` + `col.Font(i)`）+ `font/opentype`（`NewFace`）。**注意 x/image 不锁版本，放开到 latest**（用户明确反对版本 pinned）。
- **字体文件（默认 1 个）：** `Sarasa-Regular.ttc`（更纱黑体 Sarasa Gothic 官方 TTC 缩写包内原名，开源 OFL，无需改名；放可执行文件同目录，内部自动选简体等宽子变体 `Sarasa Mono SC`）。粗体默认偏移叠绘模拟；同目录另有 `Sarasa-Bold.ttc` 则升级为真实 Bold 字重。
- **农历：** `github.com/6tail/lunar-go/calendar v1.4.6`（顶部月+日、月历每格日，均为实时计算）。
- **天气：** Open-Meteo（免 key、免 token）。
- 无 Web 框架、无数据库，纯 HTTP。

## How to Build & Run

```bash
# 1. 依赖（首次 / go.mod 变动后）
go mod tidy

# 2. 编译（macOS 本机）
go build -o kindle-dashboard .

# 3. 放字体（粗体可选）
cp /path/to/Sarasa-Regular.ttc ./Sarasa-Regular.ttc
#   cp /path/to/Sarasa-Bold.ttc  ./Sarasa-Bold.ttc   # 可选真粗体

# 4. 运行（监听 :8080，被动渲染——每次请求实时取数+绘制）
./kindle-dashboard

# 5. 验证
curl -s -o /tmp/s.png -w "HTTP %{http_code} size %{size_download}\n" http://localhost:8080/screen.png
file /tmp/s.png        # 应为 PNG image data, 1072 x 1448
curl http://localhost:8080/api/display   # {"image_url":"/screen.png",...}
```

交叉编译到 Kindle（Linux x86_64）：
```bash
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o kindle-dashboard .
```

## Key Files

| File | Purpose |
|------|---------|
| `main.go` | HTTP 服务骨架。定义 `DashboardData`/`ForecastItem`/`CalendarCell`；`/screen.png` 调 `RenderDashboard` 返回 PNG，`/api/display` 返回轮询元信息。数据现由 `fetchDashboardData()` 实时获取：时间/日期/星期/农历(`lunar-go`)、真实天气(`fetchWeather` via Open-Meteo，失败回退 `mockWeather`)、5 日预报、整月日历(`buildMonthGrid`)。 |
| `render.go` | 核心绘制。`RenderDashboard(data)` 在 1072×1448 白底用 `gg` 画三区：顶部（时间+日期/农历居中、右侧圆角天气卡含温度/天气/区间/南宁）、中部 5 日预报横卡、下部 6 行月历。字体用 `sfnt`+`opentype` 解析 TTC 简体子变体；粗体偏移叠绘或真粗体。 |
| `go.mod` | 依赖：`gg v1.3.0`、`golang.org/x/image v0.44.0`、`6tail/lunar-go v1.4.6`。 |
| `.gitignore` | 排除 `.workbuddy/`、字体二进制（`*.ttf/*.ttc/*.otf`）、编译产物。 |

## What Was Just Done (2026-08-03 全天会话)

> 本会话从断点续做（原代码从未真正编译过），完整修复编译 + 接入真实数据 + 多轮布局微调。

1. **首次成功编译 + 渲染冒烟**：`golang.org/x/image/font/truetype` 包在任意版本都不存在 → 改为 `sfnt.ParseCollection`+`col.Font(0)`+`opentype.NewFace`。gg v1.3.0 无 `StrokeStringAnchored` → 粗体改为 ±0.8px 偏移叠绘模拟。
2. **字体定为更纱黑体（Sarasa Gothic，OFL）**：双文件设计（`Sarasa-Regular.ttc` 必放、简体 Mono 子变体；`Sarasa-Bold.ttc` 可选真粗体），后简化为单文件即可。`selectSC()` 遍历子字体选含 "SC" 者、优先含 "mono"。
3. **农历接入真实**：新增 `lunar-go` 依赖，`lunarDay()`（日）+`lunarMonthDay()`（月+日）；顶部 `LunarStr`、月历每格 `Lunar` 均实时计算。
4. **天气接入 Open-Meteo**：`cityName/cityLat/cityLon` 常量（南宁 22.82/108.37）；`fetchWeather()` 拉 `current`(实时 temp+code) + `daily`(7 日 weather_code/max/min)；`wmoToText()` 映射 WMO→中文(晴/多云/阴/小雨/雷阵雨…)；失败回退 `mockWeather()`。`buildForecast()` 用真实天气。
5. **日历列映射 bug 修复**：原 `data.Calendar[row][(c+1)%7]` 让 Sunday 列取成"本周起始周日"→8 号后错显 2 号。改为 c<6 取同行 c+1、c==6 取下一行 col 0。
6. **顶部布局多轮微调（用户逐条要求）**：
   - 时间放大到 128px 左对齐纵居中；
   - 日期块(2026年 / 8月3日 / 农历 六月廿一)三行，居于"时间与天气卡之间区域"**居中**（用 `dc.MeasureString` 精确测时间宽度确定左缘，避免与时间重叠）；
   - "南宁" 34px 画在天气卡左侧空白区、纵向居中；
   - 温度/天气/区间三行在卡内垂直居中，温度 90px、天气与区间均 34px；
   - 月历数字 60px、农历 31px、间距 7px。
7. **日历末格"0"修复（关键）**：原 `[6][7]`=42 天，但周一为首排版最后一行周日(行5,列6)需取数据行6col0 → 越界 → 零值 Day=0。最终方案：**保持 `[6][7]`+6 行渲染**，对越界格在 render 层从 `gridStart.AddDate(0,0,42)` 推算正确日期(如 9月6日)与农历（**未用扩成 7 行方案**，用户明确要求保持 6 行）。

## Current State

**Working:**
- 全部代码已 `go build` 通过，本机实跑 `:8080` 产出有效 1072×1448 PNG（HTTP 200）。
- 真实数据全部生效：Open-Meteo 南宁实时天气 + 7 日预报、lunar-go 农历、顶部居中三行日期、天气卡右上(温度/阴/区间)、"南宁"在卡左侧、6 行月历末格正常(9月6日+农历)。
- 字体：同目录放 `Sarasa-Regular.ttc` 即正常（已在本机验证）；`Sarasa-Bold.ttc` 可选。

**Broken / Known Issues:**
- 无已知 bug。最近一轮（23:06 渲染）日期不重叠、日历 6 行、末格正常。
- 字体二进制 `*.ttc` 被 `.gitignore` 排除，不入库——部署/换机需手动放置。

**In Progress:**
- 无。所有用户提出的可视化微调均已落地。

## Pending Tasks

- 部署到 Kindle 侧 Alpine LXC（交叉编译 + 字体随包 + 守护进程）。
- （可选）顶部天气卡显示"实时此刻"还是"今日全天预报"口径未统一：当前顶部=实时 current(如 26°C 阴)，下方今日预报卡=全天 daily(如雷阵雨 32/25°)，二者均为真实数据、维度不同，用户已知晓非 bug。

## Key Technical Decisions

- **TTC 用 `sfnt.ParseCollection`+`opentype.NewFace`**：`golang.org/x/image/font/truetype` 在任何版本都不存在（已移除），`gg.LoadFontFace` 只认单 TTF。正确链路：`sfnt.ParseCollection`→`col.Font(i)`→`opentype.NewFace(font, &opentype.FaceOptions{Size,DPI:72,Hinting:font.HintingFull})`，按 (字号,粗体) 缓存 `font.Face`。
- **单文件即可 + 粗体可选**：默认只 `Sarasa-Regular.ttc`（原名不改），`selectSC()` 选简体等宽；无 SC 时退回首个字形。粗体默认 ±0.8px 偏移叠绘；有 `Sarasa-Bold.ttc` 用真 Bold。
- **x/image 不锁版本**：用户反对 pinned，放开到 latest（当前 v0.44.0）。
- **月历列映射**：数据网格周日为首(col0=Sun)，UI 表头周一为首。c<6→数据列 c+1；c==6(周日)→下一行 col0。
- **月历 6 行 + 末格越界推算**：保持 `[6][7]`(42天)，render 层对 (行5,列6) 用 `gridStart.AddDate(0,0,42)` 填正确日期/农历——**不要扩成 7 行**（用户明确要求 6 行）。
- **日期块居中位置**：用 `r.dc.MeasureString(data.TimeStr)` 精确测时间宽度，左缘=时间右缘+50px，右缘=天气卡左缘-40px，三行在此区居中（hAlign=0.5）。不要用 `timeSize*0.65` 之类粗略估算，会和时间重叠。
- **用户偏好**：保留更纱官方 TTC 原文件名不改；git 提交动作每次由用户主动触发（本次终于要提交）。

## Gotchas & Traps

**`golang.org/x/image/font/truetype` 包已不存在**
gg 默认示例和老教程都用它，但任意 x/image 版本都无此包。
**Solution:** `sfnt.ParseCollection` + `opentype.NewFace`。
**Lesson:** 涉及 .ttc 合集字体先确认用 sfnt+opentype。

**日历末格显示 0**
[6][7] 仅 42 天，周一为首排版最后一行周日需取第 43 天 → 越界零值。
**Solution:** render 层对越界格用 `gridStart.AddDate(0,0,42)` 推算（保持 6 行，不扩 7 行）。
**Lesson:** 周一为首排版比周日为首多占 1 天，边缘格需特殊处理而非盲目扩行。

**日期与时间文字重叠**
用 `timeSize*0.65` 估算时间宽度太窄，居中后左缘压到时间上。
**Solution:** `dc.MeasureString` 实测宽度 + 50px 间距。
**Lesson:** 文字宽度必须用 MeasureString，不能拍脑袋。

**渲染验证时旧进程霸占 :8080**
上次起的服务没杀，curl 拿到的是旧图。
**Solution:** 改前先 `pkill -f kindle-dashboard`。
**Lesson:** 重测前 kill 旧进程。

**zsh 把 URL `?` 当通配符**
`curl https://...?key=1` 报 no matches found。
**Solution:** URL 加双引号。
**Lesson:** zsh 里带 `?`/`*` 的 curl 必加引号。

## How to Deploy

```bash
# 开发机交叉编译到 Kindle（Linux x86_64, alpine）
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o kindle-dashboard .

# 传可执行文件 + 字体到设备（粗体可选）
scp kindle-dashboard Sarasa-Regular.ttc user@kindle:/opt/kindle-dashboard/
#   scp Sarasa-Bold.ttc user@kindle:/opt/kindle-dashboard/   # 可选

# 设备运行（建议 supervisor / openrc 守护，监听 :8080）
/opt/kindle-dashboard/kindle-dashboard &

# 墨水屏轮询端
#   GET http://<device-ip>:8080/screen.png   -> 看板 PNG（每 60s）
#   GET http://<device-ip>:8080/api/display   -> {image_url, filename, refresh_rate}
```
