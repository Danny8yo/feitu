# Go 后端

Go 1.27.1，标准库 `net/http`，依赖版本与校验和由 `go.mod` / `go.sum` 固定。当前只提供健康检查；没有行情采集、业务接口或 MCP。

## 启动

先在仓库根目录另一终端运行 `infra/scripts/forward.sh`。在仓库根目录执行以下命令，从已有本地凭据加载环境（不要开启 shell tracing）：

```sh
set -a
. infra/.local/credentials.env
set +a
export MYSQL_DSN="feitu:${MYSQL_PASSWORD}@tcp(127.0.0.1:13306)/feitu?parseTime=true"
cd backend
go run ./cmd/server
```

服务默认监听 `127.0.0.1:8080`。`.env.example` 仅说明变量，程序不会自动读取 `.env`。`HTTP_ADDR`、`MYSQL_DSN`、`REDIS_ADDR`、`REDIS_PASSWORD`、`KAFKA_BROKERS` 均可通过环境覆盖。未设置 MySQL DSN 时使用无密码的 `feitu@tcp(127.0.0.1:13306)/feitu`，认证服务将如实显示失败。Redis 默认地址 `127.0.0.1:16379`，Kafka 默认 `127.0.0.1:19092`，多个 bootstrap broker 以逗号分隔。请保持监听本地地址；接口没有认证，不用于公网部署。

## 接口语义

| GET 路径 | 行为 |
| --- | --- |
| `/api/health/live` | 只检查 HTTP 进程，返回 200；不调用依赖 |
| `/api/health/ready` | MySQL Ping、Redis PING、Kafka 元数据与 advertised broker 元数据检查；任一失败返回 503 |
| `/api/status` | 同样的真实检查，始终返回 200，让页面展示失败详情 |

后两个接口的 JSON 格式为 `{status: "ok"或"degraded", checked_at: ISO时间, services: [{name, status: "up"或"down", latency_ms, message}]}`。四项服务顺序为 backend、mysql、kafka、redis。每次请求并行检查三依赖，共用 3 秒截止时间，不缓存成功结果。消息经过固定文案处理，不返回驱动错误、地址或凭据。检查不创建 topic、不写业务数据；Kafka 元数据可达不代表生产消费业务已经验收。服务在依赖不可用时仍能启动，恢复连接后下一次检查自动恢复。

HTTP header/read/write/idle 超时分别为 5/10/10/60 秒；SIGINT 或 SIGTERM 触发最长 5 秒优雅退出并关闭客户端。开发前端通过代理访问这些接口。

## 验证

```sh
go test -race ./...
go vet ./...
go build ./cmd/server
curl -i http://127.0.0.1:8080/api/health/live
curl -i http://127.0.0.1:8080/api/health/ready
curl -s http://127.0.0.1:8080/api/status
```

单元测试覆盖依赖成功/失败、503 与 200 的区别、错误信息脱敏、liveness 隔离、并行检查与截止时间。实际依赖连通性必须在端口转发和真实凭据加载后通过接口验证。
