#!/usr/bin/env bash
set -euo pipefail
source "$(dirname "$0")/common.sh"
echo 'MySQL: authenticated query and transactional read/write'
k exec mysql-0 -- sh -ec 'export MYSQL_PWD="$MYSQL_PASSWORD"; mysql -h 127.0.0.1 -ufeitu feitu -e "SELECT VERSION(); CREATE TABLE IF NOT EXISTS infra_smoke (id INT PRIMARY KEY, value VARCHAR(32)); START TRANSACTION; INSERT INTO infra_smoke VALUES (1, '\''ok'\'') ON DUPLICATE KEY UPDATE value='\''ok'\''; SELECT * FROM infra_smoke; COMMIT;"'
echo 'Redis: authenticated SET/GET'
k exec redis-0 -- sh -ec 'redis-cli SET infra:smoke ok EX 60 >/dev/null; test "$(redis-cli --raw GET infra:smoke)" = ok; echo ok'
echo 'Kafka: real produce/consume with unique topic'
topic="infra-smoke-$(date +%s)-$$"
trap 'k exec kafka-0 -- /opt/kafka/bin/kafka-topics.sh --bootstrap-server localhost:9092 --delete --topic "$topic" >/dev/null 2>&1 || true' EXIT
k exec kafka-0 -- /opt/kafka/bin/kafka-topics.sh --bootstrap-server localhost:9092 --create --topic "$topic" --partitions 1 --replication-factor 1 >/dev/null
printf '%s\n' 'feitu-smoke-ok' | k exec -i kafka-0 -- /opt/kafka/bin/kafka-console-producer.sh --bootstrap-server kafka.feitu-dev.svc.cluster.local:9092 --topic "$topic"
result="$(k exec kafka-0 -- /opt/kafka/bin/kafka-console-consumer.sh --bootstrap-server kafka.feitu-dev.svc.cluster.local:9092 --topic "$topic" --from-beginning --max-messages 1 --timeout-ms 20000)"
test "$result" = feitu-smoke-ok
echo "$result"
echo 'All smoke checks passed.'
