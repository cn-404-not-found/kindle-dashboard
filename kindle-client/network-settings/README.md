# Kindle Wi-Fi IP Settings（测试版）

适用于装有 KUAL、kterm 的越狱 Kindle。输入并保存 IPv4、子网掩码、网关和 DNS；查看当前网络；Ping IP 或域名；切换静态和 DHCP 模式。

把此目录内容复制至 `/mnt/us/extensions/kindle-network/`（保留已有 `network.conf`），从 KUAL 打开 `Wi-Fi IP Settings`。设置保存在设备该目录，不应提交。程序不读取或修改 Wi-Fi 密码。

- 选择静态模式前需保存参数，并连接目标 Wi-Fi。工具记录当前 SSID，只在设备关联该 SSID 时每 5 秒尝试恢复地址、路由和 DNS；其他 Wi-Fi 不应用这些设置。
- 首次启用静态模式会在 `/etc/crontab/root` 追加带 `kindle-network-watchdog` 标记的一行，用于重启后启动监测，不覆盖其他计划任务。启动后、USB 存储卸载或设备睡眠时生效时间可能延迟。
- 选择 DHCP 模式会停止静态重用，并通过系统 Wi-Fi 开关重新连接；所选模式和参数保留，cron 条目在 DHCP 模式下不执行静态修改。
- 临时测试模式有 90 秒回退；请在独立安全的网络测试，不保证 DHCP 和系统 Wi-Fi 管理器不会干扰。
- 域名 Ping 依赖 DNS；Ping 失败也可能是目标屏蔽 ICMP。

在有 DHCP 的网络中，已验证手动设置静态地址及 Ping 网关、域名。曾观察到重启后设备获取 DHCP 地址，因此重启和 Wi-Fi 重连后的自动恢复仍需进一步验证。无 DHCP 环境仍须真机测试。Kindle 系统可能在 DHCP 失败时断开 Wi-Fi 关联，不能假设监测脚本一定能维持连接。
