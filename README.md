# Casdoor

开源身份与访问管理（IAM）服务，带 Web 管理界面，支持 OAuth/OIDC、SAML、LDAP 等多种协议。

---

把 [我的 PR](https://github.com/casdoor/casdoor/pull/4974) 关掉再 [自己交一个 PR](https://github.com/casdoor/casdoor/pull/5119) 是何意味?

[用昵称当用户名](https://github.com/casdoor/casdoor/pull/4975) , 还有我懒得说的其他平台明明给了用户 UID 还要用用户名当 ID 何意味?

当个 Contributor 以为自己是 CN 体制内领导吗?

---

下文是**快速上手**；完整安装、配置、高可用与集成说明请以官方文档为准。

---

## 本地开发

### 环境要求

| 组件 | 说明 |
|------|------|
| Go | 与 `go.mod` 中版本一致（当前为 Go 1.25 系） |
| Node.js | 建议 LTS（如 20.x），用于前端 |
| Yarn | 前端包管理（勿用 npm 安装，见 `web/package.json`） |
| MySQL | 默认使用 MySQL，连接串见 `conf/app.conf` |

### 安装依赖

在项目根目录：

```bash
go mod download
```

```bash
cd web
yarn
```

### 准备数据库

按 `conf/app.conf` 中的 `dataSourceName`、`dbName` 创建数据库与用户（默认示例为本地 MySQL、`casdoor` 库）。若账号密码与配置文件不一致，请修改 `conf/app.conf` 后再启动。

### 运行

需要**两个终端**：

1. **后端**（默认监听 `8000`）：

   ```bash
   go run ./main.go
   ```

2. **前端开发服务器**（默认 `7001`，并将 API 代理到本机 `8000`）：

   ```bash
   cd web
   yarn start
   ```

浏览器访问：**http://localhost:7001**。开发环境下前端会把请求转发到后端（见 `web/craco.config.js`）。

仅验证后端或构建产物时，可直接访问 **http://localhost:8000**（需已构建前端并嵌入，或使用下方 Docker 镜像）。

### 本地构建（可选）

```bash
# 前端静态资源
cd web && yarn run build && cd ..

# 后端二进制
go build -o bin/manager main.go
```

---

## 生产环境（Docker）

本仓库 CI 在发版时会构建并推送两个镜像（命名空间以 [Docker Hub](https://hub.docker.com/u/tomyjan) 为准，标签含版本号与 `latest`）：

| 镜像 | Dockerfile 目标 | 用途简述 |
|------|-----------------|----------|
| `tomyjan/casdoor` | `STANDARD` | 仅 Casdoor 服务进程，**需外置数据库**（生产常用）。 |
| `tomyjan/casdoor-all-in-one` | `ALLINONE` | 一体化镜像（内置依赖更多，适合快速试用；生产选型见官方文档）。 |

拉取示例：

```bash
docker pull tomyjan/casdoor:latest
docker pull tomyjan/casdoor-all-in-one:latest
```

生产环境务必挂载配置、设置数据库地址与安全参数；具体环境变量与卷挂载见官方文档。

### 使用本仓库的 docker compose

在仓库根目录（会构建镜像并启动 MySQL + Casdoor）：

```bash
docker compose up -d
```

默认将 Casdoor 映射到本机 **8000**，MySQL **3306**。首次启动示例中带有 `--createDatabase=true`（见 `docker-compose.yml`）。配置目录挂载为 `./conf`，可按需修改后重启。

查看日志：

```bash
docker compose logs -f casdoor
```

停止：

```bash
docker compose down
```

### 镜像构建说明

根目录 `Dockerfile` 与上文两个镜像一一对应；本地可分别构建：

```bash
docker build --target STANDARD -t casdoor:local .
docker build --target ALLINONE -t casdoor-all-in-one:local .
```

CI 中已关闭上游「同步 `casdoor-helm` 仓库并推 Helm OCI」步骤；若需自有 Helm 流程，请自行维护 chart 或参考 [官方 Helm 文档](https://casdoor.org/docs/basic/try-with-helm)。

---

## 更多资料

| 主题 | 链接 |
|------|------|
| 源码安装与配置 | https://casdoor.org/docs/basic/server-installation |
| Docker 试用与说明 | https://casdoor.org/docs/basic/try-with-docker |
| Helm / Kubernetes | https://casdoor.org/docs/basic/try-with-helm |
| 如何接入应用 | https://casdoor.org/docs/how-to-connect/overview |
| 公开 API 与 Swagger | https://casdoor.org/docs/basic/public-api |

在线演示与完整文档入口：**https://casdoor.org**

---

## 许可证

[Apache-2.0](LICENSE)
