#!/usr/bin/env bash
set -euo pipefail
source "$(dirname "$0")/common.sh"
k get nodes >/dev/null
k get storageclass standard >/dev/null
k apply -f "$INFRA_DIR/k8s/namespace.yaml"
mkdir -p "$INFRA_DIR/.local"
chmod 700 "$INFRA_DIR/.local"
if ! k get secret feitu-credentials >/dev/null 2>&1; then
  if [[ ! -f "$INFRA_DIR/.local/credentials.env" ]]; then
    (umask 077
      printf 'MYSQL_ROOT_PASSWORD=%s\nMYSQL_PASSWORD=%s\nREDIS_PASSWORD=%s\n' \
        "$(openssl rand -hex 24)" "$(openssl rand -hex 24)" "$(openssl rand -hex 24)" > "$INFRA_DIR/.local/credentials.env")
  fi
  k create secret generic feitu-credentials --from-env-file="$INFRA_DIR/.local/credentials.env"
fi
k apply -f "$INFRA_DIR/k8s/mysql.yaml" -f "$INFRA_DIR/k8s/kafka.yaml" -f "$INFRA_DIR/k8s/redis.yaml"
for app in mysql kafka redis; do k rollout status "statefulset/$app" --timeout=600s; done
k exec kafka-0 -- /opt/kafka/bin/kafka-topics.sh --bootstrap-server localhost:9092 --create --if-not-exists --topic market.raw --partitions 3 --replication-factor 1
k get pods,pvc
