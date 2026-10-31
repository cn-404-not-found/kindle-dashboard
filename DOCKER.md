# Docker 部署

所有日常配置放在项目目录的 `.env` 文件中，不需要修改 `compose.yaml`。Compose 将这些值传给程序：

```dotenv
HOST_PORT=8080
CITY_NAME=南宁
CITY_ID=
TIMEZONE_OFFSET=8
```

- `HOST_PORT`：Docker 主机对外端口；容器内固定监听 8080。例如填 `58188` 后，通过 `http://主机IP:58188/screen.png` 访问。
- `CITY_NAME`：看板显示的城市名。内置城市见 `weather.go` 的 `weatherCityIDs`。
- `CITY_ID`：中国天气网的 9 位城市代码。内置城市可留空；其他城市需要填写对应代码。
- `TIMEZONE_OFFSET`：UTC 整小时偏移，中国为 `8`。这是固定偏移，不会自动处理夏令时。

## 首次构建并运行

1. 安装 Docker 和 Docker Compose。将 `Sarasa-Regular.ttc` 和 `Sarasa-Bold.ttc` 放在仓库根目录：字体不会提交到 Git，但构建时会被复制进镜像。其他环境可从 [Sarasa Gothic 官方发布页](https://github.com/be5invis/Sarasa-Gothic/releases)获取 TTC 文件。
2. 复制示例配置，修改 `.env`，然后构建并启动：

   ```sh
   cp -n .env.example .env
   docker compose up -d --build
   ```

3. 打开 `http://主机IP:8080/screen.png`。如果修改了 `HOST_PORT`，使用对应端口；在 Kindle 的 `Set server address` 中填入相同的主机地址和端口。

修改 `.env` 后执行 `docker compose up -d`，使 Compose 重新创建配置已变化的容器；单独执行 `docker compose restart` 不会重新读取环境变量。查看日志：`docker compose logs -f`；停止：`docker compose down`。

服务端需要能访问中国天气网，且端口需允许 Kindle 连接。不要提交真实的 `.env` 或字体文件。镜像内部保留默认 `config.json`，环境变量优先覆盖城市和时区；非 Docker 运行方式仍可使用原有 JSON 文件。

## 搬到其他 Docker 机器

在可构建的机器上运行 `docker save -o kindle-dashboard-image.tar kindle-dashboard:local`，将 tar 包、`compose.yaml`、`.env.example` 和 `.env` 传到目标机。目标机运行：

```sh
docker load -i kindle-dashboard-image.tar
docker compose up -d --no-build
```

镜像只包含构建时所选 CPU 架构；目标机架构需要匹配。字体已经包含在镜像里。源码改动后需重新构建并重新传输镜像。
