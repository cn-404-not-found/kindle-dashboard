# Kindle Dashboard 客户端

将本目录的 `config.xml`、`menu.json`、`start.sh`、`stop.sh`、`set-server.sh`、`server-settings.sh` 放到 Kindle 的 `/mnt/us/extensions/kindle-dashboard/`，并把 `kindle-client/TRMNL.sh` 复制为同目录的 `dashboard.sh`。`Set server address` 依赖设备已安装的 kterm。

在 KUAL 中选择 `Set server address`，输入 `192.168.10.103:8080` 或 `http://192.168.10.103:8080` 一类地址，确认后写入设备本地 `server-url.txt`。也支持 HTTPS 和域名，不支持路径、查询参数或 IPv6 字面地址。保存后客户端下次刷新读取，通常约 60 秒；首次从旧版脚本升级后需停止并重新启动客户端。空输入取消，不更改配置。

升级时保留现有 `server-url.txt`；如果从旧版升级且尚无此文件，先将旧 `dashboard.sh` 中的 `BASE_URL` 保存到文件，否则会回退至旧版默认地址。不要将本地地址文件提交至仓库。
