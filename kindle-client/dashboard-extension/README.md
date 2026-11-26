# Kindle Dashboard 客户端

将本目录的 `config.xml`、`menu.json`、`start.sh`、`start-landscape.sh`、`landscape-partial.sh`、`landscape-ui.sh`、`network-health.sh`、`stop.sh`、`power.sh`、`touch-light.sh`、`touch-swipe.sh`、`exit-button.sh`、`start-offline.sh`、`offline-dashboard.sh`、`offline-fetch.sh`、`offline-network.sh`、`offline-ui-guard.sh`、`set-server.sh`、`server-settings.sh` 放到 Kindle 的 `/mnt/us/extensions/kindle-dashboard/`，并把 `kindle-client/TRMNL.sh` 复制两份，分别命名为同目录的 `dashboard.sh` 和 `dashboard-landscape.sh`。`Set server address` 依赖设备已安装的 kterm。

在 KUAL 中选择 `Set server address`，输入 `192.168.10.103:8080` 或 `http://192.168.10.103:8080` 一类地址，确认后写入设备本地 `server-url.txt`。也支持 HTTPS 和域名，不支持路径、查询参数或 IPv6 字面地址。保存后客户端下次刷新读取，通常约 60 秒；首次从旧版脚本升级后需停止并重新启动客户端。空输入取消，不更改配置。

升级时保留现有 `server-url.txt`；如果从旧版升级且尚无此文件，先将旧 `dashboard.sh` 中的 `BASE_URL` 保存到文件，否则会回退至旧版默认地址。不要将本地地址文件提交至仓库。

启动看板后阻止自动屏保，并将前光调至 0；触摸屏幕后前光亮度设为 12，30 秒后调回 0。停止看板时恢复启动前的前光和屏保设置。触摸事件不会被拦截，仍会传给 Kindle 原有界面。程序异常退出时会尝试重启；设备重启后仍需从 KUAL 启动看板。触摸设备默认适配 Paperwhite 3 的 `/dev/input/event1`，其他设备可能不同。

在线模式启动时立即刷新，随后按 Kindle 系统时钟对齐刷新周期（当前服务端为每分钟），不修改系统时钟。网络延迟和绘屏会影响实际显示时刻。

横屏在线模式需新版服务端。先停止当前看板，再选择 `Start landscape online`，将设备逆时针转 90 度；如果服务端尚未更新，客户端会保留当前画面并等待重试。下载首张图成功后暂停窗口管理器，防止系统状态栏覆盖；退出时恢复，并由独立守护进程在异常退出时尝试恢复。暂停期间无法依靠普通系统界面操作，使用五秒内短按电源键四次退出；重新启动设备也可恢复。

横屏在线模式支持 1072×1448 的 PW3 和 758×1024 的 PW2，每分钟联网更新时钟图块；每小时更新天气图块，热榜首页在服务端资讯变化时自动局刷右侧区域，失败后下一分钟重试；启动或服务端日期变化时才清屏并完整绘制。PW2 需要支持该分辨率局刷的新版服务器与客户端。联网失败时会检查无线状态：设备关闭 Wi-Fi 时重新打开，掉线时先重新关联、必要时重启无线，并每分钟重申一次防屏保以避免休眠断网；设置 `KINDLE_WIFI_WATCHDOG=0` 可关闭该行为。PW2 的按键、局刷、残影及电量表现需真机验证。

横屏在线模式支持左右滑动翻页：首页向左滑进入下一页，向右滑返回上一页；每次翻页由服务端返回整页图片并完整刷新一次。翻页读取 `/dev/input/event1`，不会拦截设备原有触摸事件。设置 `KINDLE_PAGE_SWIPE=0` 可关闭滑动翻页；`KINDLE_SWIPE_AXIS=y` 是横屏旋转后的默认轴，`KINDLE_SWIPE_THRESHOLD`（默认 180）调整触发距离。当前页面由服务端响应头 `X-Dashboard-Next-Page` / `X-Dashboard-Prev-Page` 决定，新增页面不需要改客户端。

在热榜首页直接轻点右侧任意一条资讯标题可进入全屏正文阅读页（`news_detail`）；阅读页支持左右滑或轻点底栏「上一页 / 下一页」翻页、轻点左上「← 返回看板」回到首页，连续无操作 `KINDLE_ARTICLE_TIMEOUT` 秒（默认 `180` 秒 = 3 分钟）会自动返回首页。设置 `KINDLE_NEWS_TAP=0` 可关闭点击进入正文，`KINDLE_TAP_THRESHOLD`（默认 `60`）调整轻点位移容差；如需校准触摸屏最大值，可设置 `KINDLE_TOUCH_MAX_X` / `KINDLE_TOUCH_MAX_Y`。

看板运行时可在 5 秒内连续短按电源键 4 次，停止看板并返回主页；该操作使用 Paperwhite 3 的 `/dev/input/event0`，其他设备需验证。

## 离线时钟模式

`Start offline dashboard` 启动离线时钟模式，需要服务端支持 `GET /screen.png?clock=local`；`Start online dashboard` 是原在线模式。两种模式互斥，切换前先停止原模式。

离线模式按 Kindle 本机时钟每分钟局部绘制时间，并使用服务端配置的 UTC 时区偏移（首次获取前默认 UTC+8），不修改 Kindle 系统时钟；后台启动时及当地时间 0、2、4、6 点等每两个小时的整点尝试启用 Wi‑Fi 下载留白底图，完成或失败后关闭 Wi‑Fi（Kindle 可能显示为飞行模式）；下次更新会重新打开无线并尝试连接。失败保留旧底图并每 5 分钟重试；更改服务器地址后也会尝试更新；停止时尝试恢复启动前的 Wi‑Fi 状态。运行时暂停系统窗口管理器以避免状态栏重绘；退出后恢复，并由独立守护进程在异常退出时尝试恢复。状态栏隐藏和按键退出已在 Paperwhite 3 上验证；两小时自动重连仍需实机验证。时钟采用服务端字体生成的数字图块，无需在 Kindle 存放字体；当前仅针对 1072×1448 帧缓冲的 Paperwhite 3，长期耗电仍需真机测试。服务器需运行支持离线底图的新版服务端，旧服务端不会被识别为支持离线底图。
