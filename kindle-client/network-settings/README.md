# Kindle Wi-Fi IP Settings（测试版）

KUAL 扩展，依赖设备已安装的 `/mnt/us/extensions/kterm`。在 Kindle 屏幕键盘中输入并保存 IPv4、子网掩码、网关和 DNS，查看当前网络，临时应用配置，并可 Ping IP 或域名。

安装：把此目录内容复制到 Kindle 的 `/mnt/us/extensions/kindle-network/`，从 KUAL 选择 `Wi-Fi IP Settings`。配置保存在同目录的 `network.conf`，包含网络地址，不应提交。程序不会收集或修改 Wi-Fi 密码。

静态设置需要在当前连接目标 Wi-Fi 后手动应用：先保存，再选择应用并确认。启动后 90 秒内不选择保留，会尝试恢复之前的地址、路由和 DNS；退出或出现异常时如网络未恢复，请重新连接 Wi-Fi 或重启 Kindle。此功能还没有验证无 DHCP 网络中的关联、断线重连和开机持久化，不应视为最终方案。系统 DHCP 客户端或 Wi-Fi 管理器可能覆盖手动设置。当前没有配置开机自启，也不会修改系统文件来开机自启。

Ping 选项发送 3 个 ICMP 包；失败不一定说明网络不通，目标可能屏蔽 ICMP。域名测试还依赖 DNS。
