"""离线检查已下载样本，不联网、不修改 CSV；输出 JSON 报告。"""
import csv
import hashlib
import json
from collections import Counter
from datetime import date
from decimal import Decimal
from pathlib import Path

ROOT = Path(__file__).resolve().parent
manifest = json.loads((ROOT / 'manifest.json').read_text())
calendar = list(csv.DictReader((ROOT / 'raw/baostock/trade_calendar.csv').open()))
trading_days = {r['calendar_date'] for r in calendar if r['is_trading_day'] == '1'}
report = {'source': manifest['source'], 'calendar_days': len(calendar),
          'trading_days': len(trading_days), 'total_daily_rows': 0, 'files': [], 'issues': []}
for item in manifest['files']:
    path = ROOT / item['path']
    if hashlib.sha256(path.read_bytes()).hexdigest() != item['sha256']:
        report['issues'].append({'file': item['path'], 'type': 'sha256_mismatch'})
    with path.open(encoding='utf-8', newline='') as handle:
        rows = list(csv.DictReader(handle))
    if len(rows) != item['rows']:
        report['issues'].append({'file': item['path'], 'type': 'row_count_mismatch'})
    if not path.name.endswith('_daily.csv'):
        continue
    dates = [r['date'] for r in rows]
    duplicates = [d for d, n in Counter(dates).items() if n > 1]
    missing = sorted(trading_days - set(dates))
    extra = sorted(set(dates) - trading_days)
    for issue, values in [('duplicate_dates', duplicates), ('missing_trading_days', missing), ('nontrading_dates', extra)]:
        if values:
            report['issues'].append({'file': item['path'], 'type': issue, 'dates': values})
    if dates != sorted(dates):
        report['issues'].append({'file': item['path'], 'type': 'unsorted_dates'})
    for row in rows:
        date.fromisoformat(row['date'])
        if row['code'] != item['request']['code'] or row['adjustflag'] != '3':
            report['issues'].append({'file': item['path'], 'date': row['date'], 'type': 'identity_or_adjustment'})
        if row['tradestatus'] != '1':
            continue
        try:
            op, hi, lo, cl = (Decimal(row[k]) for k in ['open', 'high', 'low', 'close'])
            valid = lo > 0 and lo <= min(op, cl) <= max(op, cl) <= hi
            valid = valid and Decimal(row['volume']) >= 0 and Decimal(row['amount']) >= 0
            if not valid:
                raise ValueError('OHLC or volume/amount constraint')
        except Exception as exc:
            report['issues'].append({'file': item['path'], 'date': row['date'], 'type': 'invalid_numeric_or_ohlc', 'detail': str(exc)})
    report['total_daily_rows'] += len(rows)
    report['files'].append({'file': item['path'], 'rows': len(rows), 'first_date': min(dates),
                            'last_date': max(dates), 'suspended_rows': sum(r['tradestatus'] == '0' for r in rows)})
report['passed'] = not report['issues']
print(json.dumps(report, ensure_ascii=False, indent=2))
raise SystemExit(0 if report['passed'] else 1)
