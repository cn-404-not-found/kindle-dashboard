# Kindle Dashboard 客户端

将本目录的 `config.xml`、`menu.json`、`start.sh`、`stop.sh`、`power.sh`、`touch-light.sh`、`set-server.sh`、`server-settings.sh` 放到 Kindle 的 `/mnt/us/extensions/kindle-dashboard/`，并把 `kindle-client/TRMNL.sh` 复制为同目录的 `dashboard.sh`。`Set server address` 依赖设备已安装的 kterm。

在 KUAL 中选择 `Set server address`，输入 `192.168.10.103:8080` 或 `http://192.168.10.103:8080` 一类地址，确认后写入设备本地 `server-url.txt`。也支持 HTTPS 和域名，不支持路径、查询参数或 IPv6 字面地址。保存后客户端下次刷新读取，通常约 60 秒；首次从旧版脚本升级后需停止并重新启动客户端。空输入取消，不更改配置。

升级时保留现有 `server-url.txt`；如果从旧版升级且尚无此文件，先将旧 `dashboard.sh` 中的 `BASE_URL` 保存到文件，否则会回退至旧版默认地址。不要将本地地址文件提交至仓库。

启动看板后阻止自动屏保，并将前光调至 0；触摸屏幕后前光亮度设为 12，30 秒后调回 0。停止看板时恢复启动前的前光和屏保设置。触摸事件不会被拦截，仍会传给 Kindle 原有界面。程序异常退出时会尝试重启；设备重启后仍需从 KUAL 启动看板。触摸设备默认适配 Paperwhite 3 的 `/dev/input/event1`，其他设备可能不同。

启动时立即刷新，随后根据 `/api/display` 返回的服务端时间对齐刷新周期（当前服务端为每分钟），不修改 Kindle 系统时钟。网络延迟和绘屏会影响实际显示时刻。旧版服务端没有时间字段时使用 Kindle 系统时钟；为获得服务端时间对齐，客户端和服务端都需要更新。
