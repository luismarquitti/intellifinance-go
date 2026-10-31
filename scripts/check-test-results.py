#!/usr/bin/env python3
"""Fail the local quality gate on failed OR skipped Go tests."""
import collections
import json
import os
from pathlib import Path
import sys

counts = collections.Counter()
packages = 0
failed = False
path = Path(os.environ.get('TEST_RESULTS_PATH', 'bin/test-results.jsonl'))
path.parent.mkdir(parents=True, exist_ok=True)
with path.open('w') as evidence:
    for line in sys.stdin:
        evidence.write(line)
        event = json.loads(line)
        action = event.get('Action')
        if event.get('Test') and action in ('pass', 'fail', 'skip'):
            counts[action] += 1
        if not event.get('Test') and action in ('pass', 'fail', 'skip'):
            packages += 1
        if action == 'fail' or (action == 'skip' and event.get('Test')):
            failed = True
            print(json.dumps(event, ensure_ascii=False), file=sys.stderr)
        if action == 'output' and any(word in event.get('Output', '') for word in ('Error:', 'Error Trace:', 'expected:', 'actual  :', 'Skipping integration')):
            print(event['Output'], end='', file=sys.stderr)
print(json.dumps({'test_events': dict(counts), 'packages': packages, 'evidence': str(path)}))
if failed or not packages or not counts['pass']:
    sys.exit(1)
