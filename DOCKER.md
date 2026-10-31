# Docker 部署

GitHub Actions 在 `main` 更新后自动构建并发布 `ghcr.io/cn-404-not-found/kindle-dashboard:latest`（linux/amd64 和 linux/arm64）。仓库已公开，但镜像包目前为私有，需要有包读取权限的 GitHub 账号登录 GHCR。

镜像包含字体和程序，部署机器不需要 Go 或字体文件。仅需下载本仓库中的 `compose.yaml` 与 `.env.example`，把示例复制成 `.env`：

```sh
cp .env.example .env
# 私有镜像：使用具有 read:packages 权限的 GitHub classic PAT 登录。
# 安全输入：按提示输入 PAT；不要把令牌写入 .env 或命令历史。
docker login ghcr.io -u 你的GitHub用户名
docker compose pull
docker compose up -d
```

`.env` 包含：

```dotenv
HOST_PORT=8080
CITY_NAME=南宁
CITY_ID=
TIMEZONE_OFFSET=8
```

`HOST_PORT` 是主机对外端口，容器内固定 8080；`CITY_NAME` 是城市名；内置城市可不填 `CITY_ID`，其他城市需填中国天气网 9 位城市代码；`TIMEZONE_OFFSET` 为相对 UTC 的整小时固定偏移，中国为 8，不处理夏令时。修改 `.env` 后执行 `docker compose up -d` 重新创建容器；单独 `restart` 不会重新读取环境变量。

访问 `http://主机IP:8080/screen.png`（如更改端口，请使用相应端口），并在 Kindle 的 `Set server address` 中填写主机地址和端口。服务端需能访问中国天气网，主机端口需能从 Kindle 访问。查看日志 `docker compose logs -f`；停止 `docker compose down`。服务非 Docker 运行仍支持原有 `config.json`。

更新镜像：`docker compose pull && docker compose up -d`。离线转移可在已登录且能访问镜像的机器上执行 `docker pull ghcr.io/cn-404-not-found/kindle-dashboard:latest` 后，再使用 `docker save` / `docker load`。不要提交真实 `.env`、访问令牌或字体。

## 本地构建

```sh
docker build -t ghcr.io/cn-404-not-found/kindle-dashboard:local .
```

构建时 Dockerfile 自动从官方固定版本下载更纱 TTC 字体并校验 SHA-256；需要网络访问 GitHub。若想本地运行这个 `:local` 标签，可使用 `docker run` 并传入 `CITY_NAME`、`CITY_ID`、`TIMEZONE_OFFSET`；默认 Compose 指向远端 `:latest`。
