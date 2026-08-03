# Continuation: Kindle-dashboard（墨水屏看板）

**Date:** 2026-08-03
**Working directory:** /Users/liangyong/Documents/开发项目/Kindle-dashboard

---

## What This Project Is

一个面向 Kindle Paperwhite 3（1072×1448 墨水屏）的看板后端服务。它用 Go 起一个 HTTP 服务，每次请求实时组装数据（时间/日期/农历、今日天气、5 日预报、整月日历），调用 `gg` 库在内存里绘制成 PNG，再由墨水屏设备轮询拉取展示。目标部署平台是 Linux x86_64（Alpine LXC）。当前数据除时间/日期/星期外均为结构严密的 Mock，已在各处标注 `TODO` 待接真实 API。

## Tech Stack

- **语言：** Go（标准库 `net/http`、`image/png`、`time`、`encoding/json`）
- **绘图：** `github.com/fogleman/gg`（2D 画布）
- **字体解析：** `golang.org/x/image/font/sfnt`（`ParseCollection` + `col.Font(i)`）配合 `golang.org/x/image/font/opentype`（`NewFace`），解析 TTC 合集并按简体(SC)子变体选出字形。
- **字体文件（默认 1 个）：** `Sarasa-Regular.ttc`（更纱黑体 Sarasa Gothic 官方 TTC 包内原名，开源 SIL OFL，无需改名；放可执行文件同目录即可，内部自动选简体等宽子变体 `Sarasa Mono SC`）。粗体默认用偏移叠绘模拟；若同目录另有 `Sarasa-Bold.ttc`，则升级为真实 Bold 字重。
- 无 Web 框架、无数据库，纯 HTTP 服务。

## How to Build & Run

> 本机已安装 Go 1.26（macOS），可直接 `go build`。以下为本机/部署通用步骤：

```bash
# 1. 安装依赖（首次）
go mod tidy

# 2. 编译
go build -o kindle-dashboard .

# 3. 只放 1 个常规字体即可（粗体可选）
cp /path/to/Sarasa-Regular.ttc ./Sarasa-Regular.ttc
#   （可选）想要更锐利的真粗体，再放一个粗体文件：
#  cp /path/to/Sarasa-Bold.ttc ./Sarasa-Bold.ttc

# 4. 运行（监听 :8080）
./kindle-dashboard

# 5. 验证渲染
open http://localhost:8080/screen.png     # 返回 PNG 看板图
curl  http://localhost:8080/api/display    # 返回轮询元信息 JSON
```

部署到 Kindle 侧（Alpine LXC，Linux x86_64）时交叉编译：
```bash
GOOS=linux GOARCH=amd64 go build -o kindle-dashboard .
```

## Key Files

| File | Purpose |
|------|---------|
| `main.go` | HTTP 服务骨架（`:8080`）。定义 `DashboardData`/`ForecastItem`/`CalendarCell` 结构体；`/screen.png` 调 `RenderDashboard` 返回 PNG，`/api/display` 返回轮询元信息；数据目前为 Mock（TODO 待接真实 API）。 |
| `render.go` | 核心绘制实现。`func RenderDashboard(data DashboardData) ([]byte, error)`：用 `gg` 在 1072×1448 纯白画布上，按用户提供的 HTML/CSS 绝对坐标绘制三区（顶部时间/天气、5 日预报、整月日历），`image/png` 编码返回字节流。 |
| `go.mod` | 依赖声明：`gg v1.3.0` + `golang.org/x/image v0.18.0`。 |
| `.gitignore` | 排除 `.workbuddy/`、字体二进制（`*.ttf/*.ttc/*.otf`）、编译产物。 |

## What Was Just Done (Most Recent Session)

1. **编写 `render.go`**（本会话核心交付）：
   - 严格实现 `RenderDashboard(data DashboardData) ([]byte, error)`，对照用户给的 HTML/CSS 翻译三区布局到 `gg` 绝对坐标。
   - 顶部区：大号时间(115px)/日期(32px)/农历(24px) 左对齐纵向居中；右侧圆角 16px、4px 黑边的天气卡片（内容右对齐）；底部 4px 分割线。
   - 5 日预报：5 列圆角卡片，`IsToday` 黑底白字反色，其余白底黑框；中部因字体无法渲染 emoji，改用 `WeatherText`（中文天气）。
   - 整月日历：7 列 6 行，表头周一为首；非本月浅灰虚线+淡字，今日黑底白字。
2. **修复一处 `text()` 参数顺序笔误**（天气区间行的 `size`/`color` 写反）。
3. **补齐画布背景**：显式白色矩形铺底，不依赖 `Clear()` 语义。
4. **创建 GitHub 私有仓库** `cn-404-not-found/kindle-dashboard`，`git init` + 提交首个 commit，推送 `main`。
5. **本会话关键决策（见下「Key Technical Decisions」）**：TTC 字体解析、粗体模拟、日期推导、月历列映射。

## Current State

**Working:**
- 代码结构完整，`main.go` 与 `render.go` 接口对齐（`RenderDashboard` 签名一致）。
- GitHub 私有仓库已建好，源码（`.gitignore`/`go.mod`/`main.go`/`render.go`）已入库并推送，远端 `main` 与本地同步。
- 本地 git 身份已设（仓库级）：`cn-404-not-found` / `cn-404-not-found@users.noreply.github.com`。

**Broken / Known Issues:**
- **编译验证已通过**：本机 Go 1.26 已安装，依赖已 tidy，`go build` 跑通；以真实更纱字体（`Sarasa Mono SC`/`Sarasa Mono SC Bold`）渲染出有效 1072×1448 PNG。
- **Mock 数据未接真实源**：天气（实时温/最高/最低/天气现象）、5 日预报天气文本、月历农历（`LunarStr` 写死 "六月廿一"、`CalendarCell.Lunar` 为空）均为占位，`main.go` 内多处 `TODO`。
- **字体缺失**：`Sarasa-Regular.ttc` 未入库（更纱官方 TTC 按字重拆文件、且体积 ~80MB，故不入库）。运行前放一个常规字重文件（包内 `Sarasa-Regular.ttc`，原名即可）即可；可选再放 `Sarasa-Bold.ttc` 启用真粗体。`render.go` 的 `selectSC()` 自动选简体等宽子变体，无简体时退回首个字形（兼容任意单字重 TTF）。

**In Progress:**
- （已完成）编译与真实字体渲染验证。

## Pending Tasks

- 肉眼核对 `/screen.png` 真实渲染效果，按需微调字号/间距。
- 将 Mock 天气与农历替换为真实 API（main.go 中 TODO 项）。
- 部署到 Kindle 侧 Alpine LXC（交叉编译 + 字体随包）。

## Key Technical Decisions

- **TTC 字体用 `sfnt.ParseCollection` + `opentype.NewFace`（不是 `gg.LoadFontFace`，也不是 `golang.org/x/image/font/truetype`）**：`golang.org/x/image/font/truetype` 这个包在任意 x/image 版本里都**不存在**（早已移除，归宿是 `github.com/golang/freetype/truetype`，但 freetype 旧版也缺 `ParseCollection`），`gg.LoadFontFace` 只认单字体 TTF。正确链路：`sfnt.ParseCollection` → `col.Font(i)` 取 `*sfnt.Font` → `opentype.NewFace(font, &opentype.FaceOptions{Size,DPI,Hinting})`。按 `(字号,粗体)` 缓存 `font.Face`。
- **单文件即可 + 粗体可选**：默认只需 `Sarasa-Regular.ttc`（常规字重，原名无需改），`selectSC()` 从中选简体等宽子变体（`Sarasa Mono SC`）；无简体时退回首个字形，兼容任意单字重 TTF/TTC。粗体默认「±0.8px 偏移叠绘」模拟；同目录另有 `Sarasa-Bold.ttc` 时改用真·Bold 字重（`Sarasa Mono SC Bold`，墨水屏更锐利）。
- **5 日预报 `MM/DD` 由 `DateStr` 推导**：`ForecastItem` 结构体**没有日期字段**（只有 `Weekday/WeatherText/HighTemp/LowTemp/IsToday`），所以渲染时以 `DashboardData.DateStr`（当天）为基准，`+i` 天算出 `MM/DD`。若以后想直接传日期，需改 `ForecastItem`。
- **月历列映射 `dataCol = (c+1)%7`**：`Calendar` 网格本身是「周日为首」（`gridStart` 是包含 1 号的那个周日，`col0=Sun`），而 HTML 表头是「周一为首」（一…日）。显示列 `c` 须映射到数据列 `(c+1)%7`，否则表头与日期会错位。
- **emoji 天气图标 → 中文 `WeatherText`**：`msyh.ttc` 渲染不出 ☀️/⛅ 等 emoji，预报卡片中部用 `WeatherText` 文本替代。
- **`.gitignore` 排除 `.workbuddy/` 与字体**：WorkBuddy 本地项目数据、版权字体、编译产物都不入库。

## Gotchas & Traps

**字体是 TTC，且 `golang.org/x/image/font/truetype` 这个包已经不存在**
`gg.LoadFontFace` 只认单字体 TTF，对 .ttc 合集无效；而 `golang.org/x/image/font/truetype` 在**任何** x/image 版本里都已被移除（归宿是 `github.com/golang/freetype/truetype`，但 freetype 旧版也没有 `ParseCollection`/`Collection`）。
**Solution:** 用 `golang.org/x/image/font/sfnt` 的 `ParseCollection` + `col.Font(i)`，再用 `golang.org/x/image/font/opentype` 的 `NewFace(font, &opentype.FaceOptions{...})`。
**Lesson:** 凡涉及「合集字体 / 多语种字体文件」，先确认扩展名是 .ttc，且用 sfnt+opentype 而非已弃用的 font/truetype。当前选定字体为更纱黑体 `Sarasa-Regular.ttc`（开源 OFL，原名无需改）。

**zsh 把 URL 里的 `?` 当通配符**
上次用 `curl https://go.dev/dl/?mode=json` 下载 Go 时，zsh 报 "no matches found" 导致 curl 没拿到数据、安装中断。
**Solution:** 给 URL 加引号，或改用 `curl -sSL "https://go.dev/dl/?mode=json"`。
**Lesson:** 在 zsh 里跑带 `?`/`*` 的 curl，务必加双引号。

**本机无 Go 工具链**
`go` 不在 PATH，无法 `go build`/`go mod tidy`，所以 render.go 的编译正确性只能靠人工逐行核对 API。
**Solution:** 装 Go 后第一件事就是 `go mod tidy && go build` 验证。
**Lesson:** 交付 Go 代码前先确认目标机有工具链，否则编译错误要等用户环境才能发现。

**`Calendar` 是周日为首但表头周一为首**
若不做列映射，月历数字会整体偏移一天。
**Solution:** 渲染时显示列 `c` → 数据列 `(c+1)%7`。
**Lesson:** 数据网格的"周起始日"和 UI 表头顺序必须显式对齐，别假设一致。

## How to Deploy

```bash
# 开发机（交叉编译到 Kindle 的 Alpine LXC，Linux x86_64）
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o kindle-dashboard .

# 把可执行文件 + 两个字体一起传到设备
scp kindle-dashboard Sarasa-Regular.ttc user@kindle:/opt/kindle-dashboard/   # 粗体可选：再加 Sarasa-Bold.ttc

# 设备上运行（建议用 supervisor / openrc 守护，监听 :8080）
/opt/kindle-dashboard/kindle-dashboard &

# 墨水屏浏览器/轮询端访问
#   GET http://<device-ip>:8080/screen.png   -> 看板 PNG
#   GET http://<device-ip>:8080/api/display   -> {image_url, filename, refresh_rate}
```
