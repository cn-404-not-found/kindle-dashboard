# Continuation: Kindle Dashboard

**Date:** 2026-11-26（Asia/Shanghai）
**Working directory:** `E:\开发项目\Kindle-dashboard`（Windows）
**Archived previous handoff:** `continuation.archive.20261126_180157.md`
**Latest Code Commit (before this doc):** `fa53d689a54a0cade3859670615a2fca9bc7a0f2`（`fa53d68 feat: 调整横屏三页顺序为热榜、月历、OpenCode用量`）

---

## What This Project Is

Go HTTP 服务为越狱 Kindle（支持 PW2 `758×1024` 与 PW3 `1072×1448`）动态生成 8 位灰度 PNG 电子墨水屏看板，逆时针横屏摆放。当前涵盖首页「时钟/两日天气 + 中文科技热榜」、完整月历页、OpenCode Go 用量页，以及首页轻点新闻全屏查看正文的多页阅读视图，并搭配 Kindle KUAL/ash 客户端脚本实现局部刷新、滑动翻页、Wi-Fi 自愈与防烧屏/前光管理。

## Tech Stack

- **Backend**: Go 1.25（module `kindle-dashboard`）；`github.com/fogleman/gg`、`golang.org/x/image/font/{opentype,sfnt}`（严禁使用已移除的 `font/truetype`）、`github.com/6tail/lunar-go/calendar`。
- **Fonts**: 更纱黑体官方 `.ttc`（`Sarasa-Regular.ttc` 必选、`Sarasa-Bold.ttc` 可选），放于运行目录，已被 `.gitignore` 排除且不入库；无字库环境下渲染 PNG 的测试按约定 `t.Skip`。
- **Deployment & CI**: Docker Compose + GitHub Actions（`.github/workflows/container.yml`），自动运行 Linux 全量 `go test ./...`、构建 `linux/amd64` 与 `linux/arm64` 多架构镜像发布至 `ghcr.io/cn-404-not-found/kindle-dashboard:latest` 及提交 SHA 标签，并启动容器抽检 `/screen.png`。
- **Kindle Client**: KUAL、POSIX `sh`/BusyBox `ash`、`eips`、`curl`、LIPC、`evdev`（`/dev/input/event1` 触摸、`/dev/input/event0` 电源键）。当前主力实机为 **Kindle Paperwhite 2（PW2，`758×1024`）**。

## How to Build & Run

- 本机（Windows）未安装 Go 工具链（`where.exe go` 为空，`D:\Personal\Temp\kindle-preview-go\go\bin\go.exe` 不存在），因此完整编译与 `go test ./...` 统一在 GitHub Actions 执行。
- 本机 Git 仓库归属另一个 Windows SID，所有 `git` 命令必须带 `-c safe.directory='E:/开发项目/Kindle-dashboard'`；所有 `gh` 命令带 `--repo cn-404-not-found/kindle-dashboard`。

Linux / CI 标准构建与验证命令：

```sh
go mod download
go test ./...
go build -o kindle-dashboard .
./kindle-dashboard
curl -f -o screen.png http://127.0.0.1:8080/screen.png
curl -f -o home.png 'http://127.0.0.1:8080/screen.png?layout=landscape&page=home'
curl -f -o calendar.png 'http://127.0.0.1:8080/screen.png?layout=landscape&page=calendar'
curl -f -o opencode.png 'http://127.0.0.1:8080/screen.png?layout=landscape&page=opencode'
```

Git / CI 查看常用命令：

```powershell
git -c safe.directory='E:/开发项目/Kindle-dashboard' status --short
gh run list --repo cn-404-not-found/kindle-dashboard --limit 5
gh run watch <run-id> --repo cn-404-not-found/kindle-dashboard --exit-status
gh run view <run-id> --repo cn-404-not-found/kindle-dashboard --log | Select-String -Pattern "Run tests|Smoke test published image"
```

## Key Files

| File | Purpose |
| --- | --- |
| `AGENTS.md` | 项目编码红线与行为准则（只改需求相关代码、补测试、中文祈使提交、禁止提交二进制/`.env`）。 |
| `main.go` | HTTP 入口：`/screen.png`、`/api/tap`、`/api/news`、`/api/display`、`/api/opencode/usage`；包含 `fitToDevice`/`rotate90CW` 与 `news_detail` 快速分支。 |
| `pages.go` | 横屏页面轮换顺序（`dashboardPageNames`: `home → calendar → opencode`）与按需页 `news_detail` 路由及局刷能力声明。 |
| `landscape.go` | 横屏左栏（时钟、天气、无标题放大两日天气大字卡 `260×239`、农历/节气）与第 2 页完整月历渲染。 |
| `landscape_partial.go` | 横屏局刷图块裁切与 PW2/PW3 坐标换算：`clock`、`weather`（已扩至 `136,55..701,599`）、`news`、`pageheader`。 |
| `tech_news.go` | 5 源中文科技热榜（`solidot,sspai,ithome,geekpark,v2ex`）定时抓取（30 分钟）、广告过滤、`NewsItem` 正文提取与 50 条历史索引。 |
| `news_page.go` | 首页右侧热榜绘制与 `wrapTwoLines`（ASCII 整词回退 + 中文避头点）。 |
| `news_article.go` | 正文按需抓取（少数派 `wangEditor-txt` 等容器提取与 2h 缓存）、`rawTouchToLandscape`、热榜与阅读页命中判定及 `/api/tap`。 |
| `news_article_page.go` | `1448×1072` 全屏正文阅读页（`← 返回看板`、第 1 页大标题+续页紧凑单行标题、`38px` 正文自动分页、翻页底栏、无二维码）。 |
| `weather.go` / `opencode_usage.go` / `opencode_page.go` | 中国天气网采集缓存与 OpenCode Go 订阅用量页。 |
| `kindle-client/dashboard-extension/landscape-partial.sh` | PW2/PW3 横屏主循环：每分钟时钟、整点天气（`136:55`/`96:38`）、热榜局刷、轻点 `tap` 调用、阅读页快翻与 3 分钟无操作回首页。 |
| `kindle-client/dashboard-extension/touch-swipe.sh` | `/dev/input/event1` 滑动翻页与短点（`tap <x> <y>`，`KINDLE_TAP_THRESHOLD=60`）识别，带同轴坐标缓存与 `BTN_TOUCH` 去重。 |
| `kindle-client/dashboard-extension/landscape-ui.sh` | 暂停/恢复系统窗口管理器 `awesome`，以及可被 `SIGUSR1` 安全终止清理的 `landscape_sleep`。 |
| `kindle-client/dashboard-extension/network-health.sh` | Wi-Fi 链路检查与分级恢复。 |
| `kindle-client/dashboard-extension/menu-pw2.json` | PW2 精简菜单；在 PW2 设备上作为 `menu.json` 使用（**切勿用 PW3 完整 `menu.json` 覆盖**）。 |
| `tech_news_test.go` / `news_article_test.go` | 热榜、正文解析抓取、坐标映射、命中测试、分页、handler 单元测试。 |
| `landscape_test.go` / `landscape_partial_client_test.go` / `pages_test.go` | 局刷坐标、PW2 适配、FIFO 触摸模拟、点击阅读与超时回首页 Shell 测试（`!windows` 在 CI 执行）。 |
| `.github/workflows/container.yml` | CI 测试 + 多架构镜像构建发布 + 冒烟测试。 |

## What Was Just Done (Most Recent Session)

1. **左栏两日天气卡片放大去标题 + 热榜折行优化（`044656c`）**：
   - 按预览确认方案，去掉 `landscape.go` 的 `"未来两日天气"` 小标题，将双卡放大为 `260×239`（`y=692..931`）、内部分隔线 + `"明天 · 周五"` / `"后天 · 周六"`，`46px` 天气与 `36px` 温度。
   - 同步将 `part=weather` 局刷矩形下扩为 `136,55..701,599`（PW3 `weather_x=136`，PW2 `weather_x=96`）。
   - 优化 `news_page.go` `wrapTwoLines`：修复 `runes[i] == ' '` 会越过中文误断句的问题，改为沿连续 ASCII/数字/`.`/`-` 词元回退，并增加中文句末/分隔标点避头点（`isClosingPunctRune`）。
2. **新增「首页点击资讯全屏看正文」及 PW2 触控适配（`279513e` → `adda325` → `4cd2534` → `908906d`）**：
   - 验证了 5 个默认资讯源的全文可得性：Solidot/IT之家/极客公园/V2EX 的 RSS/Atom 直接带完整正文（其中 `geekpark` ~4800–7100 字）；少数派 RSS 仅带 `查看全文` 导语，直抓 `sspai.com/post/...` 的 `wangEditor-txt` 容器可稳定拿到全文（实测 6746 字，HTTP 200）。
   - 实现 `tech_news.go` 正文提取、`news_article.go` 按需抓取与 2 小时内存缓存、`/api/tap` 与 `rawTouchToLandscape`（适配 PW2 `758×1024` / PW3 `1072×1448`）、`news_article_page.go` 全屏阅读分页渲染（无二维码）。
   - 顺带修掉了导致 `IT之家`/`少数派`/`Solidot` 在线上报错 `unexpected end element </link>` 的根因（Go `xml.HTMLAutoClose` 内含 `"link"`），修复后 CI 容器冒烟日志不再报任何源抓取错误。
   - 升级 `touch-swipe.sh`、`landscape-partial.sh`、`landscape-ui.sh`：支持轻点识别、`noop` 点击仅亮背光不闪屏、页内翻页无全黑闪烁、左上返回按钮与 `KINDLE_ARTICLE_TIMEOUT=180` 秒（3 分钟）自动回首页，并修复 `SIGUSR1` 唤醒竞态与 `landscape_partial_client_test.go` FIFO/客户端端到端回归测试。
3. **调整横屏三页滑动顺序（`fa53d68`）**：
   - 将 `pages.go` 的 `dashboardPageNames` 改为 `[home, calendar, opencode]`，对应：第 1 页带资讯的时间面板 → 第 2 页带日历的面板 → 第 3 页 OpenCode 用量；同步更新 `pages_test.go` 与 `README.md`。
4. **更新已连接的 Kindle PW2（`G:\extensions\kindle-dashboard`）**：
   - 已核对 `G:` 为 PW2 设备（原 `menu.json` 277B 与 `menu-pw2.json` 一致，保留 `server-url.txt` 27B 未动）。
   - 将纯 LF（`CR=0`）并做过 SHA256 一致性校验的 4 个文件写入覆盖到 `G:\extensions\kindle-dashboard\`：`landscape-partial.sh`（11761 B）、`touch-swipe.sh`（3765 B）、`landscape-ui.sh`（1543 B）、`README.md`（5631 B）。

## Current State

**Working:**
- Commit `fa53d68` 已推送至 `origin/main`，[GitHub Actions Run 45794459998](https://github.com/cn-404-not-found/kindle-dashboard/actions/runs/45794459998)（`1m51s`）全绿：
  - `go test ./...` 通过（`ok kindle-dashboard 14.306s`；含字体 PNG 渲染的用例在无 `.ttc` 字体环境按设计 `t.Skip`，其余单测、handler 与 Linux shell/FIFO 客户端测试全部实际跑通）。
  - Docker 多架构镜像发布与冒烟验证通过，且 `IT之家`/`少数派`/`Solidot` 在 RSS `<link>` 修复后容器冒烟日志零报错。
- PW2 客户端 `G:\extensions\kindle-dashboard\` 4 个更新文件已写盘并校验 `CR=0` 与 SHA256 一致；`server-url.txt` 与 PW2 `menu.json` 完整保留。

**Broken / Known Issues:**
- 少数派个别会员付费专栏文章在网页端只有试读段，无法读取付费全文（免费文章均可完整抓取）。
- GitHub Actions 上的部分国内源若遇到特定海外 CDN/WAF 返回 HTML（而非 XML），会走 `返回了 HTML 页面而非 RSS/Atom` 容错并保留旧缓存；国内 NAS 网络实测 5 源均返回 HTTP 200。

**In Progress (Awaiting User / Device Verification):**
- 新版服务尚未在用户 NAS Docker 上拉取重启；Kindle PW2 文件已更新在设备存储内，需用户安全弹出设备后重启 `Start landscape online` 进行真机手感验收。

## Pending Tasks

1. 用户在电脑安全弹出 Kindle（`G:`）并拔线。
2. 服务端执行 `docker compose pull && docker compose up -d` 拉取最新镜像并重启容器。
3. 在 PW2 的 KUAL 菜单停止旧看板进程后重新启动 `Start landscape online`，真机验证：
   - 第 1 页「带资讯的时间面板」→ 第 2 页「带日历的面板」→ 第 3 页「OpenCode 用量」切换是否顺畅；
   - 点击右侧任意新闻全屏看正文（命中准度、翻页、左上返回与 3 分钟自动回首页）；若发现轻点落点存在系统偏差，根据日志微调坐标或 `KINDLE_TOUCH_MAX_X`/`KINDLE_TOUCH_MAX_Y`。
4. 后续如继续调整 UI 布局或样式，按用户偏好先用渲染预览（`html_render`）给用户看效果，确认满意后再开发提交 GitHub。

## Key Technical Decisions

- **`news_detail` 独立于三页轮换**：不放入 `dashboardPageNames`，确保日常左右滑动仍是 `home → calendar → opencode` 3 页；仅通过首页轻点进入，在阅读页左上点击或第 1 页右滑返回 `home`。
- **不添加原文二维码**：按用户明确确认保持阅读页清爽，不占正文空间，正文 38px + 长文分页。
- **PW2 坐标映射方案**：结合 KOReader `KindlePaperWhite2` 驱动与 `rotate90CW` 实现，PW2 `cyttsp4_mt` 直接按竖屏分辨率 `758×1024`（`rawTouchToLandscape` 支持环境变量 `KINDLE_TOUCH_MAX_X`/`Y` 覆盖）逆算回 `1448×1072` 横屏坐标。
- **混合正文抓取与缓存隔离**：RSS 自带长文直接从 `techNews` 内存直出；截断条目点击时按需抓取并缓存 `2h`，抓取失败仅临时降级返回导语但不写 `2h` 缓存以便重试。`screenHandler` 对 `page=news_detail` 跳过 `fetchDashboardData()`，避免看新闻时误触发阻塞天气抓取。
- **轻点左栏不刷新屏幕**：`tap` 若命中 `/api/tap` 返回 `noop` 且未到下分钟刷新时刻，客户端跳过全页/时钟刷新，只由 `touch-light.sh` 点亮背光。
- **PW2 菜单保留策略**：`G:\extensions\kindle-dashboard\menu.json` 保持 `menu-pw2.json` 内容（277 B），绝不用仓库完整 `menu.json` 覆盖。

## Gotchas & Traps

**`xml.HTMLAutoClose` 会破坏标准 RSS 的 `<link>...</link>` 标签**
Go 标准库 `xml.HTMLAutoClose` 将 `link` 视为 HTML 自闭合空标签，导致 RSS 2.0 中 `<link>https://...</link>` 在开标签后立刻合成关闭、`</link>` 报错 `unexpected end element </link>` 或从 `stack` 错弹出外层 `<item>` 导致日期/正文全丢。
**Solution:** 在 `parseNewsFeedItems` 中改用显式排版容错列表 `dec.AutoClose = []string{"br", "hr", "img"}`，并在 `EndElement` 时按标签名逆序查找栈位置后弹出（未匹配则忽略游离结束标签）。
**Lesson:** 解析 RSS/Atom XML 绝不要直接套用 `xml.HTMLAutoClose`。

**Go `regexp`（RE2）不支持反向引用 `\1`**
`(?is)<(script|...)[^>]*>.*?</\1>` 可通过编译，但在包 `init()` 阶段直接 `panic: invalid escape sequence: \1`。
**Solution:** 新增无反向引用的 `compileTagBlockPatterns(tags ...string)` 与 `stripRegexBlocks`（复用在 `tech_news.go` 与 `news_article.go`）。
**Lesson:** 本机无 Go 工具链时，写法上严格避免 PCRE 特性（`\1`、前瞻/后瞻）并在推送前 grep `MustCompile`。

**`strings.ToLower` 扫描 HTML 可能导致字节偏移错位**
某些网页含特殊 Unicode 字符，直接用 `strings.ToLower(htmlStr)` 索引并切原串 `htmlStr[...]` 可能在边界产生偏移偏差。
**Solution:** 在 `news_article.go` 中编写仅转换 ASCII `'A'..'Z'` 的 `asciiLower`，保证输入输出字节长度严格 1:1 相等。
**Lesson:** 混用索引文本与原文切片时必须保证大小写转换保持字节等长。

**Go 无类型浮点常量表达式不能直接转 `int(...)`**
例如 `int((1071.0 - 95.0) * 758.0 / 1072.0)` 含非整数常量会直接编译失败。
**Solution:** 表达式至少一侧用运行时浮点变量或 `float64(...)`（如 `lx, ly := 980.0, 95.0`）。
**Lesson:** 像素换算常量写测试时先落到 `float64` 变量。

**`touch-swipe.sh` 的 `BTN_TOUCH=1` 后到与 evdev 同轴去重**
Linux Type B 多点触控常先发 `ABS_MT_TRACKING_ID` 与 `X/Y` 再发 `BTN_TOUCH=1`，原 `begin()` 无条件清零起点会擦除首触坐标；连点同一水平位时内核也会省略未变的 `ABS_MT_POSITION_X`。
**Solution:** `begin()` 增加 `[ "$touching" -eq 1 ] && return` 守卫，并用 `last_x`/`last_y` + `seen_x`/`seen_y` 延续上次坐标。
**Lesson:** 同时兼容多事件顺序和同轴事件省略。

**`landscape-partial.sh` `SIGUSR1` 唤醒与主循环竞态**
若 `SIGUSR1` 在 `eips`/`curl` 收尾阶段提前到来（尚未进入 `landscape_sleep`），trap 仅置 `page_requested=1` 仍会被后续无条件 `landscape_sleep` 睡满 30–60 秒；且不同 shell（Ubuntu CI `dash` / Kindle `ash`）对 `wait` 中断后残余 `sleep` 子进程行为有差异。
**Solution:** 在 `USR1` trap 内直接 `[ -n "${landscape_sleep_pid:-}" ] && kill "$landscape_sleep_pid" 2>/dev/null`；`apply_page_request` 开头清零 `page_requested=0`；调用 `landscape_sleep` 前判断 `[ "${page_requested:-0}" -eq 0 ] && [ ! -f "$page_request" ]`；在 `landscape-ui.sh` 的 `landscape_sleep` 结束清理子进程。
**Lesson:** 信号驱动的 shell 守卫必须在循环与睡眠入口双重防竞态，并用 FIFO/信号端到端测试验证。

**Windows CRLF、Git SID 与无本地 Go 编译器**
工作区默认检出可能带 CRLF，直接复制 `.sh` 到 Kindle 会导致 `ash` 异常；本机需 `-c safe.directory='E:/开发项目/Kindle-dashboard'` 与 `gh --repo`。
**Solution:** 向 Kindle 写文件时过滤掉 CR 字节（`$_ -ne 13`）并校验 `CR=0` 与 SHA256；提交前同样校验仓内 `.sh` 为 LF，CI 负责 `go test ./...`。
**Lesson:** 脚本文件永远以二进制字节验证无 `0x0D`。

## How to Deploy

### 1. 服务端（Docker Compose）

在服务端（NAS）项目目录运行：

```sh
docker compose pull
docker compose up -d
docker compose ps
```

### 2. Kindle PW2 客户端（本次已更新并校验完毕，重装/重做参考）

保持 `G:\extensions\kindle-dashboard\server-url.txt`（`27 B`）与 PW2 专用 `menu.json`（等同 `menu-pw2.json`，`277 B`）不动，以纯 LF 写入以下 4 个文件并核对 `CR=0`：

```powershell
$repoDir = 'E:\开发项目\Kindle-dashboard\kindle-client\dashboard-extension'
$kindleDir = 'G:\extensions\kindle-dashboard'
foreach ($name in @('landscape-partial.sh', 'touch-swipe.sh', 'landscape-ui.sh', 'README.md')) {
  $lf = [byte[]]([System.IO.File]::ReadAllBytes((Join-Path $repoDir $name)) | Where-Object { $_ -ne 13 })
  [System.IO.File]::WriteAllBytes((Join-Path $kindleDir $name), $lf)
}
```

安全弹出 Kindle 后，在 KUAL 进入 `Kindle Dashboard` → 先执行 `Stop`（如原进程仍在跑）→ 再选择 `Start landscape online`。运行日志在 `/tmp/kindle-dashboard.log`。
