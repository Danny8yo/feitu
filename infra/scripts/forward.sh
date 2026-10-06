#!/usr/bin/env bash
set -euo pipefail
source "$(dirname "$0")/common.sh"
pids=()
cleanup() { for pid in "${pids[@]}"; do kill "$pid" 2>/dev/null || true; done; }
trap cleanup EXIT
trap 'exit 0' INT TERM
kubectl --context "$FEITU_CONTEXT" -n "$NAMESPACE" port-forward --address 127.0.0.1 svc/mysql 13306:3306 & pids+=("$!")
kubectl --context "$FEITU_CONTEXT" -n "$NAMESPACE" port-forward --address 127.0.0.1 svc/redis 16379:6379 & pids+=("$!")
kubectl --context "$FEITU_CONTEXT" -n "$NAMESPACE" port-forward --address 127.0.0.1 svc/kafka 19092:19092 & pids+=("$!")
echo 'MySQL 127.0.0.1:13306 | Redis 127.0.0.1:16379 | Kafka 127.0.0.1:19092 (Ctrl-C stops forwards)'
while true; do
  for pid in "${pids[@]}"; do
    if ! kill -0 "$pid" 2>/dev/null; then echo 'A port-forward exited; stopping the others.' >&2; exit 1; fi
  done
  sleep 2
done
