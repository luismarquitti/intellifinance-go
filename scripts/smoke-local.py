#!/usr/bin/env python3
"""Real HTTP smoke test against an empty, synthetic loopback development tenant.

Imports repository fixtures and cleans up only new IDs from account scopes
registered before mutation. Refuses nonempty tenants and non-synthetic or remote sessions.
"""
import csv
from decimal import Decimal
import json
import os
from pathlib import Path
import re
import sys
from typing import Any
from urllib.error import HTTPError
from urllib.parse import urlencode, urlsplit
from urllib.request import Request, urlopen
import uuid

BASE = os.environ.get('SMOKE_BASE_URL', 'http://127.0.0.1:8090').rstrip('/')
FIXTURES = Path(__file__).resolve().parents[1] / 'test/fixtures'
CHECKS = []
CREATED = set()
# Registered before any upload; safe only for this empty synthetic tenant with
# no concurrent editors. Never recover IDs from an unregistered account.
SCOPES = {}


def scope_items(account_id):
    items = []
    page = 1
    while True:
        result = request('/api/v1/transactions?' + urlencode(dict(limit=200, page=page, account_id=account_id)))
        batch = result['items']
        if any(tx['account_id'] != account_id for tx in batch):
            raise AssertionError('cleanup query returned another account scope')
        items.extend(batch)
        if len(items) >= result['pagination']['total']:
            return items
        if not batch:
            raise AssertionError('incomplete cleanup pagination')
        page += 1


def cleanup():
    failures = []
    for account_id, baseline in SCOPES.items():
        for attempt in range(3):
            try:
                ids = {tx['id'] for tx in scope_items(account_id)} - baseline
                if ids:
                    request('/api/v1/transactions/batch-delete', 'POST', {'ids': sorted(ids)}, expected=204)
                remaining = {tx['id'] for tx in scope_items(account_id)} - baseline
                if remaining:
                    raise AssertionError('cleanup deletion not confirmed by read')
                break
            except Exception as error:
                if attempt == 2:
                    failures.append(f'RECOVERY required: base={BASE} account_id={account_id} '
                                    f'baseline_ids={sorted(baseline)} known_ids={sorted(CREATED)}; '
                                    f'query this scope and confirm deletion after network recovery: {error}')
    if failures:
        raise RuntimeError('; '.join(failures))


def check(condition, message):
    if not condition:
        raise AssertionError(message)
    CHECKS.append(message)


def request(path, method='GET', data=None, expected=200, headers=None) -> Any:
    headers = dict(headers or {})
    if isinstance(data, dict):
        data = json.dumps(data).encode()
        headers['Content-Type'] = 'application/json'
    req = Request(BASE + path, data=data, method=method, headers=headers)
    try:
        response = urlopen(req, timeout=10)
    except HTTPError as error:
        response = error
    with response:
        body = response.read()
        if response.code != expected:
            raise AssertionError(f'{method} {path}: expected {expected}, got {response.code}: {body.decode()[:400]}')
        if 'json' in response.headers.get('Content-Type', '') and body:
            return json.loads(body)
        return body


def transactions(**filters):
    return request('/api/v1/transactions?' + urlencode(dict(limit=200, **filters)))['items']


def upload(filename, account_id):
    boundary = 'synthetic-' + uuid.uuid4().hex
    body = (f'--{boundary}\r\nContent-Disposition: form-data; name="account_id"\r\n\r\n{account_id}\r\n'
            f'--{boundary}\r\nContent-Disposition: form-data; name="file"; filename="{filename}"\r\n'
            'Content-Type: text/csv\r\n\r\n').encode()
    body += (FIXTURES / filename).read_bytes() + f'\r\n--{boundary}--\r\n'.encode()
    return request('/api/v1/transactions/import/csv', 'POST', body,
                   headers={'Content-Type': f'multipart/form-data; boundary={boundary}'})


def balance(account_id):
    return Decimal(request('/api/v1/accounts/' + account_id)['balance'])


def run():
    check(urlsplit(BASE).hostname in ('127.0.0.1', 'localhost', '::1'), 'loopback target')
    session = request('/api/v1/session')
    check(session['environment'] == 'development' and session['user_name'] == 'Synthetic Admin', 'synthetic development session')
    check(not transactions(), 'empty synthetic tenant required')
    check(request('/healthz')['status'] == 'UP', 'liveness')
    report = request('/readyz')
    check(report['status'] == 'UP' and report['checks']['postgres'] == 'UP', 'real PostgreSQL readiness')
    html = request('/').decode()
    asset = re.search(r'src="([^"]+\.js)"', html)
    check(asset is not None and b'<html' not in request(asset.group(1)), 'embedded dashboard and JavaScript asset')
    accounts = request('/api/v1/accounts')['items']
    categories = request('/api/v1/categories')['items']
    imports = []
    for filename, account_type in [('extrato_itau.csv', 'CHECKING'), ('fatura_nubank.csv', 'CREDIT_CARD')]:
        account_id = next(account['id'] for account in accounts if account['type'] == account_type)
        before = balance(account_id)
        with (FIXTURES / filename).open() as fixture:
            rows = list(csv.DictReader(fixture, delimiter=';' if account_type == 'CHECKING' else ','))
        if account_type == 'CHECKING':
            expected_delta = sum((Decimal(row['Valor'].replace('.', '').replace(',', '.')) for row in rows), Decimal(0))
        else:
            expected_delta = sum((-Decimal(row['amount']) for row in rows), Decimal(0))
        SCOPES[account_id] = {tx['id'] for tx in scope_items(account_id)}
        check(not SCOPES[account_id], 'empty upload account scope required')
        result = upload(filename, account_id)
        CREATED.update(tx['id'] for tx in transactions(account_id=account_id))
        check(result['inserted'] == len(rows) and result['total_parsed'] == len(rows), filename + ' imports every row')
        check(balance(account_id) == before + expected_delta, filename + ' updates balance exactly once')
        duplicate = upload(filename, account_id)
        check(duplicate['inserted'] == 0 and duplicate['duplicates'] == len(rows), filename + ' deduplicates on reimport')
        check(balance(account_id) == before + expected_delta, filename + ' duplicate import preserves balance')
        imports.append({'fixture': filename, 'inserted': result['inserted'], 'duplicates_on_reimport': duplicate['duplicates']})
    all_transactions = transactions()
    check(len(all_transactions) == len(CREATED), 'list and pagination cover all imported records')
    income = sum((Decimal(tx['amount']) for tx in all_transactions if tx['type'] == 'INCOME'), Decimal(0))
    expense = sum((-Decimal(tx['amount']) for tx in all_transactions if tx['type'] == 'EXPENSE'), Decimal(0))
    summary = request('/api/v1/transactions/summary')
    check(Decimal(summary['total_income']) == income and Decimal(summary['total_expense']) == expense
          and Decimal(summary['net_balance']) == income - expense, 'KPI totals reconcile with ledger')
    found = transactions(search='SUPERMERCADO')
    check(bool(found) and all('SUPERMERCADO' in tx['description'].upper() for tx in found), 'description filter')
    sorted_transactions = transactions(sort_by='amount', sort_order='asc')
    amounts = [Decimal(tx['amount']) for tx in sorted_transactions]
    check(amounts == sorted(amounts), 'monetary sorting')
    expenses = [tx for tx in all_transactions if tx['type'] == 'EXPENSE']
    category_id = next(category['id'] for category in categories if category['type'] == 'EXPENSE')
    ids = [tx['id'] for tx in expenses[:2]]
    request('/api/v1/transactions/batch-category', 'POST', {'ids': ids, 'category_id': category_id})
    check(all(request('/api/v1/transactions/' + id_)['category_id'] == category_id for id_ in ids), 'batch category classification read back')
    tx = expenses[0]
    old_balance = balance(tx['account_id'])
    request('/api/v1/transactions/' + tx['id'], 'PATCH', {'description': 'SYNTHETIC SMOKE EDIT', 'amount': '-11.25'})
    edited = request('/api/v1/transactions/' + tx['id'])
    check(edited['description'] == 'SYNTHETIC SMOKE EDIT' and edited['amount'] == '-11.25', 'transaction edit read back')
    check(balance(tx['account_id']) == old_balance - Decimal(tx['amount']) - Decimal('11.25'), 'edit applies one balance delta')
    request('/api/v1/transactions/' + tx['id'], 'DELETE', expected=204)
    request('/api/v1/transactions/' + tx['id'], expected=404)
    CREATED.remove(tx['id'])
    check(True, 'single deletion read back')
    request('/api/v1/transactions/batch-delete', 'POST', {'ids': sorted(CREATED)}, expected=204)
    CREATED.clear()
    check(not transactions(), 'batch deletion read back')
    check(all(balance(account['id']) == Decimal('0') for account in accounts), 'balances return to zero after deletion')
    return imports


if __name__ == '__main__':
    try:
        try:
            results = run()
        finally:
            cleanup()
        print(json.dumps({'status': 'PASS', 'checks': CHECKS, 'imports': results}, ensure_ascii=False, indent=2))
    except Exception as error:
        print(json.dumps({'status': 'FAIL', 'checks_completed': CHECKS, 'error': str(error)}, ensure_ascii=False), file=sys.stderr)
        sys.exit(1)
