# 本地 Kubernetes 数据基础设施

用于金融行情练习的单机环境：MySQL 8.4.11、Apache Kafka 4.1.2（KRaft，broker/controller 合并）、Redis 7.4.11。应用框架尚未选择。

当前复用 Docker Desktop 中的 `kind-study-k8s`，独立命名空间 `feitu-dev`。不修改已有 `default` 命名空间应用。所有脚本显式指定 context，不依赖当前 kubectl context。仅支持本地 kind；不是生产高可用部署。

## 启动与验证

在仓库根目录执行：

```bash
# macOS 首先启动 Docker Desktop
open -a Docker
infra/scripts/up.sh
infra/scripts/smoke.sh
```

`up.sh` 自动生成随机凭据、创建 Kubernetes Secret、部署 StatefulSet/PVC、等待服务就绪并创建三分区 `market.raw` topic。重复执行会复用 Secret 和数据。首次需要网络拉取官方镜像。MySQL 开启 performance_schema、0.5 秒慢查询记录（mysql.slow_log），便于后续调优。

本地凭据在 `infra/.local/credentials.env`，目录权限 700、文件 600，已由 `infra/.gitignore` 排除。不要提交或复制到日志。若本地文件丢失而 Secret 尚在，请从 Secret 恢复；不要重新生成密码覆盖已有 MySQL 数据目录。修改 Secret 也不会自动修改已有数据库账户密码。

如果在另一台机器没有集群：

```bash
kind create cluster --name feitu --image kindest/node:v1.37.0 --config infra/kind.yaml
FEITU_CONTEXT=kind-feitu infra/scripts/up.sh
FEITU_CONTEXT=kind-feitu infra/scripts/smoke.sh
```

要求默认 `standard` StorageClass（kind 自带 local-path provisioner）。实际已在 Kubernetes v1.37.0 / Linux arm64 验证。建议 Docker 分配至少 4 CPU、6 GB 内存、15 GB 可用空间；当前机器为 10 CPU / 约 8 GB。三服务 requests 共 600m CPU / 1408 MiB 内存，limits 共 5 CPU / 2816 MiB；另需 Kubernetes 和其他应用资源。PVC 共申请 11 GiB，local-path 不保证严格磁盘配额。

## 本机与集群内连接

先保持一个终端运行：

```bash
infra/scripts/forward.sh
```

| 服务 | 本机地址 | 集群内地址 | 账户 |
|---|---|---|---|
| MySQL | 127.0.0.1:13306 | mysql.feitu-dev.svc.cluster.local:3306 | 用户 feitu，库 feitu，密码 MYSQL_PASSWORD |
| Kafka | 127.0.0.1:19092 | kafka.feitu-dev.svc.cluster.local:9092 | 本地练习 PLAINTEXT，无认证 |
| Redis | 127.0.0.1:16379 | redis.feitu-dev.svc.cluster.local:6379 | 默认用户，密码 REDIS_PASSWORD |

端口转发只绑定 loopback；没有 NodePort / LoadBalancer。Kafka 有独立内部和外部 listener，外部元数据发布 `127.0.0.1:19092`，不能随意换成本机其他端口。Pod 重建后重新启动端口转发。集群内没有额外 NetworkPolicy，适用于可信本地学习集群。

宿主机端到端验证（测试依赖仅用于检查，**不代表后端选用 Python**）：

```bash
python3 -m venv infra/.local/client-venv
infra/.local/client-venv/bin/pip install -r infra/scripts/client-requirements.txt
infra/.local/client-venv/bin/python infra/scripts/smoke-host.py
```

`smoke.sh` 在集群内部验证 MySQL 事务读写、Redis SET/GET、Kafka 生产消费；`smoke-host.py` 从 macOS 经端口转发验证 MySQL 查询、Redis SET/GET、Kafka 创建 topic、生产和消费，能发现 advertised.listeners 配错的问题。测试 Kafka topic 会删除，Redis 测试键有 TTL；MySQL 留下一张独立 `infra_smoke` 表。

## 停止、恢复与排查

```bash
# 只停止本项目三个服务，保留 PVC 和 Secret
infra/scripts/stop.sh
# 恢复并等待 ready
infra/scripts/up.sh
# 状态和日志
kubectl --context kind-study-k8s -n feitu-dev get pods,pvc
kubectl --context kind-study-k8s -n feitu-dev logs kafka-0 --tail=100
kubectl --context kind-study-k8s -n feitu-dev describe pod mysql-0
```

前台 `forward.sh` 用 Ctrl-C 停止转发。退出 Docker Desktop 会暂停整套本地容器，包含其他项目；通常只用本项目 stop 脚本。

PVC 能跨 Pod 重建和缩容保留数据，但 kind 节点容器删除、集群删除或 PVC 删除会造成数据丢失，不能作为备份。Kafka 单 broker / replication factor=1，不具备容灾；Redis AOF everysec 适合缓存学习，可能丢失最后约一秒写入。不要通过增加 Kafka StatefulSet 副本实现扩容：当前 node id 和 controller quorum 专为单节点配置。

如 Docker Desktop 的系统凭据助手卡住，Kubernetes 节点仍能直接拉公开镜像；无需修改全局 Docker 配置。`kind load docker-image` 在某些 Docker/containerd 多架构组合可能报 digest 缺失，正常节点拉取不依赖该命令。

## 官方参考

- [MySQL 官方镜像与初始化变量](https://hub.docker.com/_/mysql)
- [Apache Kafka 4.1 Docker 镜像](https://kafka.apache.org/41/getting-started/docker/)
- [Redis 官方镜像与持久化](https://hub.docker.com/_/redis)
- [Kubernetes StatefulSet](https://kubernetes.io/docs/concepts/workloads/controllers/statefulset/)

已固定具体镜像版本；升级前自行验证，不使用 latest。

## 本次验收（2026-10-06）

- 三个 StatefulSet 均 1/1 Ready，三份 PVC 均 Bound。
- 集群内 MySQL 事务写入/查询、Redis SET/GET、Kafka 生产/消费通过。
- macOS 经 loopback 转发的三服务真实客户端验证通过，Kafka 外部 listener 正常。
- 三服务缩容到 0 后重新启动，MySQL 测试行、Redis AOF 测试键、Kafka 消息均仍可读取；恢复后清理 Redis/Kafka 测试内容。
- YAML 服务端 dry-run 和 shell 语法检查通过。
- 验收结束保留三个服务运行，关闭临时端口转发。使用时运行 `infra/scripts/forward.sh`。
