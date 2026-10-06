# feitu

面向金融数据开发岗位的练习项目：通过历史行情回放，练习 Kafka 可靠消费、数据清洗与质量检查、MySQL 调优、Kubernetes 运维，以及 MCP 数据服务。

## 当前范围

- 先使用下载好的真实历史行情 CSV，不实现持续在线采集。
- 数据准备：[股票样本说明](data/stock_samples/README.md)，包含 20 只 A 股的 2024—2025 年不复权日线，共 9,700 条，以及交易日历、来源清单和离线验证脚本。
- 基础环境：[Kubernetes 部署与连接说明](infra/README.md)，用于本地 MySQL、Kafka、Redis 学习环境。
- 后端：Go 1.27.1，标准库 HTTP 服务，提供进程存活、依赖就绪和环境状态接口，见 [后端说明](backend/README.md)。
- 前端：React + TypeScript + Vite，展示真实依赖状态，见 [前端说明](frontend/README.md)。行情业务和 MCP 尚未实现。

原始样本保持不变。后续通过回放器注入缺失、重复和异常数据，完成故障恢复与入库对账练习；压测合成数据另行保存并标注。

## 本机开发

开发工具固定为 Go 1.27.1、Node.js 24.18.0、pnpm 11.9.0。Node 版本见根 `.nvmrc`，Go 版本见 `backend/go.mod`，前端版本与依赖见 `frontend/package.json` 和锁文件。Go/Node 在宿主机运行，MySQL/Kafka/Redis 留在本地 Kubernetes；本阶段不需要 Nginx。

在仓库根目录执行（已准备过工具与依赖可跳过安装步骤）：

```sh
nvm install
nvm use
npm install --global pnpm@11.9.0
pnpm --dir frontend install --frozen-lockfile
infra/scripts/up.sh
scripts/dev.sh
```

Go 尚未安装时可通过 Homebrew 安装，之后用 `go version` 核对项目版本。当前开发机已经安装 Go 1.27.1。后端模块将在首次构建时下载。

打开 <http://127.0.0.1:5173>。启动脚本优先选择 nvm 中固定的 Node，只影响脚本进程，不修改系统默认版本。脚本从 `infra/.local/credentials.env` 加载密码，不把凭据写入前端或版本控制；自动打开三个端口转发并运行前后端。请勿同时运行另一份 `forward.sh`，脚本会检测端口冲突。

Ctrl-C 停止本次启动的应用与转发，Kubernetes 数据服务继续运行。日志位于忽略版本控制的 `.local/logs/`；后端源代码修改后重新启动脚本，前端支持热更新。单独运行/调试见两个子目录说明。

页面检查的是连接可用性，不代表行情已入库。MySQL、Redis 执行真实协议检查，Kafka 检查元数据与返回的 broker 地址；依赖失败时页面显示异常，`/api/health/ready` 返回 503，`/api/health/live` 仍返回 200。

## 本阶段验收（2026-10-06）

- 后端 `go test -race ./...`、`go vet ./...` 与构建通过；前端 Node 24.18.0 下类型检查与生产构建通过。
- 浏览器通过 Vite 代理访问 Go，四项服务显示正常；实际就绪接口返回 200。
- 对本地 Redis 执行短暂的自动到期命令暂停：就绪检查在约 3 秒后返回 503，存活检查仍为 200，页面显示 Redis 异常与 3/4。暂停到期后重新检查恢复 4/4，无需重启后端。
- 停止开发脚本后五个本机端口均释放，三个 Kubernetes Pod 保持 Ready；重新启动脚本后前后端及依赖连接恢复。
- 浏览器最终页面无控制台错误。当前没有行情业务接口、MCP 或应用容器部署。

## 验证历史样本

在仓库根目录执行：

```sh
python3 data/stock_samples/verify.py
```

## 后续链路

```text
CSV → 回放器 → Kafka → 清洗与校验 → MySQL → 查询服务 / MCP
                                  ↘ 质量异常与入库对账
```

未来可增加上游 API 适配器。当前文件对账验证数据处理链路完整性，不代表独立跨源价格核验。

早期全市场在线采集、Docker Compose 和 Python 主后端方案已调整；以本说明及后续讨论为准。
