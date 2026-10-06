"""Exercise all services from macOS through the loopback port forwards."""
from pathlib import Path
from uuid import uuid4
import json
import pymysql
import redis
from kafka import KafkaAdminClient, KafkaConsumer, KafkaProducer
from kafka.admin import NewTopic

credentials = dict(line.split('=', 1) for line in
    (Path(__file__).resolve().parents[1] / '.local/credentials.env').read_text().splitlines())
with pymysql.connect(host='127.0.0.1', port=13306, user='feitu',
                     password=credentials['MYSQL_PASSWORD'], database='feitu') as conn:
    with conn.cursor() as cur:
        cur.execute('SELECT VERSION(), DATABASE()')
        print('MySQL host query:', cur.fetchone())
r = redis.Redis(host='127.0.0.1', port=16379, password=credentials['REDIS_PASSWORD'])
key = f'infra:host:{uuid4().hex}'
r.set(key, 'ok', ex=60)
assert r.get(key) == b'ok'
r.delete(key)
print('Redis host SET/GET: ok')
bootstrap = '127.0.0.1:19092'
topic = f'infra-host-{uuid4().hex}'
admin = KafkaAdminClient(bootstrap_servers=bootstrap, request_timeout_ms=20000)
producer = consumer = None
try:
    admin.create_topics([NewTopic(topic, num_partitions=1, replication_factor=1)])
    producer = KafkaProducer(bootstrap_servers=bootstrap, acks='all', request_timeout_ms=20000)
    value = json.dumps({'check': 'host-produce-consume', 'id': uuid4().hex}).encode()
    producer.send(topic, value=value).get(timeout=20)
    producer.flush(timeout=20)
    consumer = KafkaConsumer(topic, bootstrap_servers=bootstrap, auto_offset_reset='earliest',
        enable_auto_commit=False, consumer_timeout_ms=20000, group_id=f'check-{uuid4().hex}')
    records = list(consumer)
    assert len(records) == 1 and records[0].value == value, records
    print('Kafka host produce/consume and advertised listener: ok')
finally:
    if consumer: consumer.close()
    if producer: producer.close(timeout=10)
    admin.delete_topics([topic])
    admin.close()
