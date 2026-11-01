# Kindle Dashboard 客户端

将本目录的 `config.xml`、`menu.json`、`start.sh`、`stop.sh`、`power.sh`、`touch-light.sh`、`exit-button.sh`、`set-server.sh`、`server-settings.sh` 放到 Kindle 的 `/mnt/us/extensions/kindle-dashboard/`，并把 `kindle-client/TRMNL.sh` 复制为同目录的 `dashboard.sh`。`Set server address` 依赖设备已安装的 kterm。

在 KUAL 中选择 `Set server address`，输入 `192.168.10.103:8080` 或 `http://192.168.10.103:8080` 一类地址，确认后写入设备本地 `server-url.txt`。也支持 HTTPS 和域名，不支持路径、查询参数或 IPv6 字面地址。保存后客户端下次刷新读取，通常约 60 秒；首次从旧版脚本升级后需停止并重新启动客户端。空输入取消，不更改配置。

升级时保留现有 `server-url.txt`；如果从旧版升级且尚无此文件，先将旧 `dashboard.sh` 中的 `BASE_URL` 保存到文件，否则会回退至旧版默认地址。不要将本地地址文件提交至仓库。

启动看板后阻止自动屏保，并将前光调至 0；触摸屏幕后前光亮度设为 12，30 秒后调回 0。停止看板时恢复启动前的前光和屏保设置。触摸事件不会被拦截，仍会传给 Kindle 原有界面。程序异常退出时会尝试重启；设备重启后仍需从 KUAL 启动看板。触摸设备默认适配 Paperwhite 3 的 `/dev/input/event1`，其他设备可能不同。

启动时立即刷新，随后按 Kindle 系统时钟对齐刷新周期（当前服务端为每分钟），不修改系统时钟。网络延迟和绘屏会影响实际显示时刻。

看板运行时可在 5 秒内连续短按电源键 4 次，停止看板并返回主页；该操作使用 Paperwhite 3 的 `/dev/input/event0`，其他设备需验证。

## 离线时钟模式（开发分支测试版）

`Start offline dashboard` 需要服务端支持 `GET /screen.png?clock=local`。原 `Start dashboard` 仍是在线模式。两种模式互斥，切换前先停止原模式。

离线模式按 Kindle 本机时钟每分钟局部绘制时间，并使用服务端配置的 UTC 时区偏移（首次获取前默认 UTC+8），不修改 Kindle 系统时钟；后台启动时、每两小时及跨天后尝试启用 Wi‑Fi 下载留白底图，完成或失败后关闭 Wi‑Fi（Kindle 可能显示为飞行模式）；下次更新会重新打开无线并尝试连接。失败保留旧底图并每 5 分钟重试；停止时尝试恢复启动前的 Wi‑Fi 状态。运行时暂停系统窗口管理器以避免状态栏重绘；退出后恢复，并由独立守护进程在异常退出时尝试恢复。此行为及退出、重连需真机验证。时钟采用服务端字体生成的数字图块，无需在 Kindle 存放字体；当前仅针对 1072×1448 帧缓冲的 Paperwhite 3，数字局部显示和长期耗电仍需真机测试。用户服务器需运行此分支对应的服务端，旧服务端不会被识别为支持离线底图。
