# 本地日历测试版

此入口在 Kindle 本机计算日期、农历和节气，生成 1072×1448 灰度图；不连接服务器或请求网络。使用中国标准时间 UTC+8，每天零点更新。运行期间仍阻止自动屏保；目前没有实现深度休眠和 RTC 唤醒，续航需要实测。

将本目录的 `config.xml`、`menu.json`、`start.sh`、`stop.sh`、`run.sh`，以及交叉编译的 `calendar` 程序和 `Sarasa-Regular.ttc` 放到 `/mnt/us/extensions/kindle-local-calendar/`。可选放入 `Sarasa-Bold.ttc`。字体不纳入 Git。此入口复用 `/mnt/us/extensions/kindle-dashboard/` 的电源管理和退出脚本，安装时需保留原扩展。

使用 Go 1.25 和 `GOOS=linux GOARCH=arm GOARM=7 CGO_ENABLED=0` 编译项目，产物放在上述目录并命名为 `calendar`。从 KUAL 启动 `Local Calendar`；切换前先停止正在运行的旧看板。停止时在 KUAL 选择 `Stop local calendar`，或五秒内短按电源键四次。原看板的操作入口不受影响。

该程序不切换 Wi-Fi；需要关闭无线时可使用设备的飞行模式。长期离线运行需确保设备时钟准确。首次启动会立即生成图片，失败会记录到 `/tmp/kindle-dashboard.log` 并在五分钟后重试。
