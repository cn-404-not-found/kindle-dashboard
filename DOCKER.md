# Docker 部署

项目采用容器内固定监听 `8080`，主机端口通过 Compose 的 `HOST_PORT` 修改；城市、时区和城市代码使用容器外的 `config.json`。程序只在启动时读取配置，修改 JSON 后需要重启容器。

## 首次构建并运行

1. 安装 Docker 和 Docker Compose。将 `Sarasa-Regular.ttc` 和 `Sarasa-Bold.ttc` 放在仓库根目录：字体不会提交到 Git，但会在构建时复制进镜像。已有本地字体可以直接使用；其他环境可从 [Sarasa Gothic 官方发布页](https://github.com/be5invis/Sarasa-Gothic/releases)获取对应 TTC 文件。
2. 首次运行前准备配置文件（已有 `config.json` 时无需复制）：

   ```sh
   cp -n config.example.json config.json
   docker compose up -d --build
   ```

3. 查看 `http://主机IP:8080/screen.png`，在 Kindle 的 `Set server address` 中输入 `主机IP:8080`。服务器须能访问中国天气网，且端口需能从 Kindle 访问。

`config.json` 例子：

```json
{
  "port": 8080,
  "city_name": "南宁",
  "timezone_offset": 8
}
```

容器内 `port` 保持 `8080`；如需对外使用 `58188`，在 Compose 同目录建立 `.env` 文件写入 `HOST_PORT=58188`，然后运行 `docker compose up -d`，访问 `http://主机IP:58188/screen.png`。`timezone_offset` 是 UTC 整小时偏移，并非 IANA 时区，不自动处理夏令时。其他城市若未在 `weather.go` 的 `weatherCityIDs` 中列出，需要在 JSON 中增加该城市对应的 9 位 `city_id`。

修改 JSON 后运行 `docker compose restart`。查看日志：`docker compose logs -f`；停止：`docker compose down`。请勿将真实的 `config.json`、`.env` 和字体文件提交到 Git。

## 搬到其他 Docker 机器

在可构建的机器上运行 `docker save -o kindle-dashboard-image.tar kindle-dashboard:local`，将 tar 包、`compose.yaml` 和 `config.json` 传到目标机。目标机运行：

```sh
docker load -i kindle-dashboard-image.tar
docker compose up -d --no-build
```

镜像只包含构建时所选 CPU 架构；目标机的架构需要匹配。此时字体已经包含在镜像里。镜像不会随源码改动自动更新，重新构建并重新传输后才会包含新版本。
