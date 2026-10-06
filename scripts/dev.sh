#!/usr/bin/env bash
# 本机开发入口：固定 Node，加载本地凭据，启动转发与前后端。
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"
node_version="$(tr -d '\n\r' < .nvmrc)"
node_bin="${NVM_DIR:-$HOME/.nvm}/versions/node/v${node_version}/bin"
if [[ -d "$node_bin" ]]; then export PATH="$node_bin:$PATH"; fi
for tool in go node curl lsof kubectl; do
  command -v "$tool" >/dev/null || { echo "缺少工具：$tool" >&2; exit 1; }
done
[[ "$(node --version)" == "v${node_version}" ]] || { echo '请先运行 nvm install && nvm use。' >&2; exit 1; }
[[ -f frontend/node_modules/vite/bin/vite.js ]] || { echo '请先执行 pnpm --dir frontend install --frozen-lockfile。' >&2; exit 1; }
[[ -f infra/.local/credentials.env ]] || { echo '请先按 infra/README.md 启动基础环境并准备凭据。' >&2; exit 1; }
for port in 8080 5173 13306 16379 19092; do
  if lsof -nP -iTCP:"$port" -sTCP:LISTEN >/dev/null 2>&1; then
    echo "端口 $port 已被占用；请关闭此前的开发进程或端口转发后重试。" >&2
    exit 1
  fi
done
mkdir -p .local/bin .local/logs
chmod 700 .local
(cd backend && go build -o "$ROOT/.local/bin/server" ./cmd/server)
# 此文件由基础设施脚本生成，只在当前进程中使用，不打印凭据。
source infra/.local/credentials.env
export MYSQL_DSN="feitu:${MYSQL_PASSWORD}@tcp(127.0.0.1:13306)/feitu?parseTime=true&timeout=2s&readTimeout=2s&writeTimeout=2s"
export REDIS_ADDR=127.0.0.1:16379 REDIS_PASSWORD
export KAFKA_BROKERS=127.0.0.1:19092 HTTP_ADDR=127.0.0.1:8080
pids=()
cleanup() {
  trap - EXIT
  for pid in "${pids[@]}"; do kill "$pid" 2>/dev/null || true; done
  for pid in "${pids[@]}"; do wait "$pid" 2>/dev/null || true; done
}
trap cleanup EXIT
trap 'exit 0' INT TERM
bash infra/scripts/forward.sh >.local/logs/forward.log 2>&1 & pids+=("$!")
.local/bin/server >.local/logs/backend.log 2>&1 & pids+=("$!")
env -u MYSQL_DSN -u REDIS_PASSWORD -u MYSQL_PASSWORD -u MYSQL_ROOT_PASSWORD \
  node frontend/node_modules/vite/bin/vite.js frontend --host 127.0.0.1 --port 5173 --strictPort >.local/logs/frontend.log 2>&1 & pids+=("$!")
started=false
for attempt in {1..30}; do
  for pid in "${pids[@]}"; do
    kill -0 "$pid" 2>/dev/null || { echo '启动失败，请查看 .local/logs/。' >&2; exit 1; }
  done
  if curl --fail --silent --max-time 2 http://127.0.0.1:8080/api/health/live >/dev/null && \
     curl --fail --silent --max-time 2 http://127.0.0.1:5173 >/dev/null; then started=true; break; fi
  sleep 1
done
[[ "$started" == true ]] || { echo '启动超时，请查看 .local/logs/。' >&2; exit 1; }
echo '前端：http://127.0.0.1:5173'
echo '后端：http://127.0.0.1:8080/api/status'
echo '日志：.local/logs/；Ctrl-C 停止应用和端口转发，保留 Kubernetes 数据服务。'
while true; do
  for pid in "${pids[@]}"; do
    kill -0 "$pid" 2>/dev/null || { echo '开发进程已退出，请查看 .local/logs/。' >&2; exit 1; }
  done
  sleep 2
done
