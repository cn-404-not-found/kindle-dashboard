# Continuation: Kindle-dashboard

**Date:** 2026-10-31
**Working directory:** /Users/liangyong/Documents/开发项目/Kindle-dashboard

---

## What This Project Is

面向 Kindle Paperwhite 3 的墨水屏看板：Go 服务生成 1072×1448 PNG，显示当前时间、中国天气网天气与五日预报、公历和农历月历。Kindle 客户端轮询服务端并用 `eips` 显示，服务器负责天气数据获取、农历计算和绘图；运行不需要 AI 模型。

## Tech Stack

- Go 1.25（本机 Go 1.26.5）；`net/http`、`github.com/fogleman/gg`、`golang.org/x/image`。
- `github.com/6tail/lunar-go`：农历、节气和节日。
- 中国天气网公开网页：Go 解析 HTML 与网页内的观测数据，非官方 API；天气请求缓存 30 分钟。
- 字体：`Sarasa-Regular.ttc` 必需，`Sarasa-Bold.ttc` 可选；不提交字体。
- Kindle 客户端：`kindle-client/TRMNL.sh`，需 `curl`、`eips`。
- 无数据库、无模型服务。

## How to Build & Run

```bash
cd '/Users/liangyong/Documents/开发项目/Kindle-dashboard'
go mod download
go test ./...
go build -o kindle-dashboard .
# 准备 Sarasa-Regular.ttc；Sarasa-Bold.ttc 可选。
# 首次建立配置：cp -n config.example.json config.json
./kindle-dashboard
# 在另一终端：
curl http://127.0.0.1:8080/api/display
curl -o /tmp/kindle-screen.png http://127.0.0.1:8080/screen.png
open /tmp/kindle-screen.png
```

本机字体与 `config.json` 已存在。配置默认 `port=8080`、`city_name=南宁`、`timezone_offset=8`；城市不在 `weatherCityIDs` 时需在配置中指定中国天气网 9 位 `city_id`。本地此前通过 Codex exec 会话启动服务，不能保证会话结束后仍运行或重启后自启。

## Key Files

| File | Purpose |
|------|---------|
| `main.go` | 配置、数据结构、农历/周一开头月历、HTTP 接口和屏幕缩放。 |
| `weather.go` | 中国天气网页面、气温解析、城市代码、30 分钟缓存和失败状态。 |
| `render.go` | 看板绘制；天气卡片使用左右两栏、上温度中温区下预报。 |
| `calendar_test.go`, `weather_test.go` | 日期、节日、解析和过期预报测试。 |
| `config.example.json` | 配置示例。`config.json` 是本机未跟踪配置，不应顺手提交。 |
| `kindle-client/TRMNL.sh` | Kindle 拉取与 eips 刷新脚本。 |
| `AGENTS.md` | 项目约定，修改前阅读。 |

## What Was Just Done (Most Recent Session)

- 拉取 GitHub 后建立 macOS 本地 Go 环境、编译程序，下载并解压更纱 TTC 字体。运行服务并生成 PNG 验证。
- `7b2753b`：用中国天气网网页替换 Open-Meteo；读取逐日预报和观测温度，天气缓存 30 分钟，失败保留旧数据或显示不可用。
- `819e32b`：修复月历以周一开头、首日为周日漏显示的问题，添加农历初一月份、节气和传统节日；增强对比度、天气错误提示。
- `8094559`、`d574761`、`c6a119f`：根据用户截图迭代天气卡片。最终为左侧大号城市，右侧从上到下为当前温度、最高/最低温及“今日 天气”；不显示来源、观测时间、预报时间；左右向中间收紧、右侧整体下移。天气失败仍显示简短提示。
- 最后一次静态预览：`/Users/liangyong/.codex/visualizations/2026/10/30/01a18d3c-2bf5-7961-8b3e-48ded9ae79ad/kindle-card-refined.png`；使用 2026-10-30 的已获取网页数据生成，仅用于排版演示，不是当前实时天气。

## Current State

**Working:**
- 最近一次 `go test ./...` 和 `go build -o kindle-dashboard .` 成功；本地已验证生成 PNG，服务端监听 :8080。
- Git `main` 已推送至 `origin/main`，代码 HEAD 为 `c6a119f`；本次交接文档改动尚未提交。`config.json` 是之前创建的未跟踪文件，必须保留。
- 未连接或验证 Kindle 真机；部署、屏幕效果、网络与客户端还需实测。

**Broken / Known Issues:**
- 午夜跨日后，2026-10-31 00:00 实测中国天气网网页仍以 10 月 30 日为首项，严格日期校验导致 `预报日期与当前日期不匹配`；重启后的内存缓存为空，因此显示“天气暂不可用”、预报区留空。根因是网页逐日预报尚未切换而解析器要求首项为当天；网页何时刷新未知。此问题未修复。历史缓存只存内存，重启后失去。
- `TRMNL.sh` 仅在请求 `/api/display` 时附加 `png-width` / `png-height`，下载 `/screen.png` 时未携带，故服务端分辨率自适应不生效。历史交接提及真机画面拉伸和截断，根因尚未在设备端确认。
- 抓取公开网页并非官方稳定 API；HTML 结构/网页访问规则变化会影响运行；无长期可靠性保证。
- 城市映射只有南宁、北京、上海、广州、深圳、杭州、成都、武汉、西安、重庆；其余需显式 `city_id`。

**In Progress:**
- 无用户明确要求的未完成改版。用户尚未对最后的天气卡片版式反馈。

## Pending Tasks

1. 用户有意先本地开发、后续部署服务端；当前没有部署目标或服务器凭据，尚未执行部署。
2. 如用户继续要求修复：优先处理午夜天气网页日期滞后问题，确保不会把昨天预报错误标为今天；为跨日/首次启动及恢复编写测试。
3. 后续连接 Kindle 后，排查并验证分辨率参数传递、真实屏幕方向、刷新与截断。设备尚未连接。

## Key Technical Decisions

- 只由服务端获取和渲染数据，Kindle 客户端只拉取 PNG 并显示；服务器不需要大模型。
- 中国天气网公开网页而非 Open-Meteo 为当前天气源；不展示来源与观测/预报时间是用户明确要求。
- 数据有效性优先于伪造天气：解析失败时显示不可用，已有缓存标注未更新；旧预报跨日不重标成新日期。
- 今日天气文案表示预报，当前温度来自网页最近观测；并非同一时点。
- 用户要求天气卡片“左边是比较大的‘南宁’两个字；右边上半部分是 26°，23~31°，下半部分是‘今日 多云’”；最近还要求两栏靠中间、右侧上下留白均匀。

## Gotchas & Traps

**午夜天气源滞后导致整块天气失效**
2026-10-31 00:00 日志显示日期不匹配；重启后没有缓存。
**Solution:** 尚未实现。研究网页的次日条目，按真实日期筛选，并单独处理当天温度和网页更新时间；不可把旧数据冒充当天。
**Lesson:** 夜间、跨月、网页未更新及重启都要用固定网页样本测试。

**Mac 系统字体不能替代指定字体**
将 `STHeiti Medium.ttc` 链接成 `Sarasa-Regular.ttc` 造成“未找到简体(SC)子变体”。
**Solution:** 使用更纱官方 TTC，文件不入仓库。已在本地安装常规和粗体。
**Lesson:** 字体合集兼容性应实际渲染验证。

**Kindle 图像尺寸头没有传到图片请求**
脚本把分辨率头发送给元信息接口而不是 `/screen.png`。
**Solution:** 待设备端任务时修正和验证，不要宣称适配已经生效。
**Lesson:** 检查完整客户端请求链和真机表现。

**Git 中本地配置未跟踪**
`config.json` 是既有本地配置；不要删除、覆盖或意外提交。
**Solution:** 配置变更前先检查；源代码提交只挑本次相关文件。
**Lesson:** 把本地配置、字体和用户数据与源码改动区分。

## How to Deploy

服务端部署到未来的 Linux x86_64 Alpine 服务器，实际主机及地址尚未知：

```bash
cd '/Users/liangyong/Documents/开发项目/Kindle-dashboard'
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /tmp/kindle-dashboard-linux .
# 确认目标服务器路径及凭据后，将 /tmp/kindle-dashboard-linux、
# Sarasa-Regular.ttc、可选 Sarasa-Bold.ttc、config.json 上传至同一目录。
# 在服务器启动：./kindle-dashboard-linux
# 客户端配置 BASE_URL 为服务端可达地址；需在 Kindle 上安装/启动 TRMNL.sh。
```

具体复制命令和开机自启方式依服务器环境确定；目前没有部署，也没有真机验证。
