"""Fault injection only: no calls to the real API or browser."""
import contextlib
import io
import json
import os
from pathlib import Path
import runpy
import subprocess
import tempfile
import unittest
from unittest.mock import patch
from urllib.parse import parse_qs, urlsplit

ROOT = Path(__file__).resolve().parents[1]


class Response:
    def __init__(self, data, code=200, json_body=True):
        self.code = code
        self.headers = {'Content-Type': 'application/json' if json_body else 'text/html'}
        self.body = json.dumps(data).encode() if json_body else data
    def __enter__(self):
        return self
    def __exit__(self, *args):
        pass
    def read(self):
        return self.body


class HTTPSmokeTest(unittest.TestCase):
    def exercise(self, fault):
        state = {'items': [], 'committed': False, 'failed_query': False, 'deleted': []}
        if fault == 'preexisting':
            state['items'] = [{'id': 'old', 'account_id': 'checking'}]
        def network(req, **kwargs):
            url = urlsplit(req.full_url)
            route = url.path
            if route == '/api/v1/session':
                return Response({'environment': 'development', 'user_name': 'Synthetic Admin'})
            if route in ('/healthz', '/readyz'):
                return Response({'status': 'UP', 'checks': {'postgres': 'UP'}})
            if route == '/':
                return Response(b'<html><script src="/asset.js"></script>', json_body=False)
            if route == '/asset.js':
                return Response(b'javascript', json_body=False)
            if route == '/api/v1/accounts':
                return Response({'items': [{'id': 'checking', 'type': 'CHECKING'}, {'id': 'card', 'type': 'CREDIT_CARD'}]})
            if route.startswith('/api/v1/accounts/'):
                return Response({'balance': '0'})
            if route == '/api/v1/categories':
                return Response({'items': []})
            if route.endswith('/import/csv'):
                state['committed'] = True
                state['items'] = [{'id': 'new', 'account_id': 'checking'}, {'id': 'foreign', 'account_id': 'other'}]
                if fault in ('lost-response', 'offline'):
                    raise OSError('connection lost after commit')
                return Response({'inserted': 100, 'total_parsed': 100})
            if route.endswith('/batch-delete'):
                ids = json.loads(req.data)['ids']
                state['deleted'].extend(ids)
                if fault != 'unconfirmed-delete':
                    state['items'] = [tx for tx in state['items'] if tx['id'] not in ids]
                return Response(None, 204)
            if route == '/api/v1/transactions':
                if state['committed'] and (fault == 'offline' or (fault == 'query-transient' and not state['failed_query'])):
                    state['failed_query'] = True
                    raise OSError('query unavailable')
                account = parse_qs(url.query).get('account_id', [None])[0]
                items = [tx for tx in state['items'] if not account or tx['account_id'] == account]
                return Response({'items': items, 'pagination': {'total': len(items)}})
            raise AssertionError('unexpected fake request: ' + req.full_url)
        output, errors = io.StringIO(), io.StringIO()
        with patch('urllib.request.urlopen', side_effect=network), patch.dict(os.environ, SMOKE_BASE_URL='http://127.0.0.1:1'), contextlib.redirect_stdout(output), contextlib.redirect_stderr(errors):
            try:
                runpy.run_path(str(ROOT / 'scripts/smoke-local.py'), run_name='__main__')
            except SystemExit as error:
                self.assertNotEqual(error.code, 0)
        return state, output.getvalue() + errors.getvalue()

    def test_lost_response_after_commit_recovers_only_scope(self):
        state, output = self.exercise('lost-response')
        self.assertEqual(state['items'], [{'id': 'foreign', 'account_id': 'other'}], output)
        self.assertEqual(state['deleted'], ['new'])
        self.assertNotIn('"PASS"', output)

    def test_transient_query_after_upload_still_cleans(self):
        state, output = self.exercise('query-transient')
        self.assertEqual(state['items'], [{'id': 'foreign', 'account_id': 'other'}], output)

    def test_preexisting_tenant_is_not_mutated(self):
        state, _ = self.exercise('preexisting')
        self.assertEqual(state['items'], [{'id': 'old', 'account_id': 'checking'}])
        self.assertFalse(state['committed'])

    def test_delete_must_be_confirmed_by_read(self):
        state, output = self.exercise('unconfirmed-delete')
        self.assertIn('RECOVERY', output)
        self.assertNotIn('"PASS"', output)
        self.assertTrue(any(tx['id'] == 'new' for tx in state['items']))

    def test_failed_scope_does_not_prevent_other_scope_cleanup(self):
        module = runpy.run_path(str(ROOT / 'scripts/smoke-local.py'), run_name='smoke_test')
        namespace = module['cleanup'].__globals__
        namespace['SCOPES'].update({'checking': set(), 'card': set()})
        remaining = [{'id': 'card-new', 'account_id': 'card'}]
        def request(route, method='GET', data=None, expected=200):
            if 'account_id=checking' in route:
                raise OSError('checking unavailable')
            if method == 'POST':
                self.assertIsNotNone(data)
                assert data is not None
                self.assertEqual(data['ids'], ['card-new'])
                remaining.clear()
                return None
            return {'items': remaining.copy(), 'pagination': {'total': len(remaining)}}
        namespace['request'] = request
        with self.assertRaisesRegex(RuntimeError, 'RECOVERY.*checking'):
            module['cleanup']()
        self.assertEqual(remaining, [])

    def test_offline_reports_recovery_scope_without_success(self):
        state, output = self.exercise('offline')
        self.assertIn('RECOVERY', output)
        self.assertIn('checking', output)
        self.assertNotIn('"PASS"', output)
        self.assertEqual(state['deleted'], [])


class BrowserSmokeTest(unittest.TestCase):
    def exercise(self, fault):
        with tempfile.TemporaryDirectory() as tmp:
            state_file = Path(tmp) / 'state.json'
            env = dict(os.environ, PLAYWRIGHT_MODULE=str(ROOT / 'test/fake-playwright.cjs'),
                       FAULT=fault, STATE_FILE=str(state_file), SMOKE_BASE_URL='http://127.0.0.1:1')
            result = subprocess.run(['node', str(ROOT / 'scripts/smoke-browser.cjs')], env=env, capture_output=True, text=True)
            self.assertNotEqual(result.returncode, 0, result.stdout)
            return json.loads(state_file.read_text()), result.stdout + result.stderr

    def test_lost_response_after_commit_recovers_only_scope(self):
        state, output = self.exercise('lost-response')
        self.assertEqual(state['items'], [{'id': 'foreign', 'account_id': 'other'}], output)
        self.assertEqual(state['deleted'], ['new'])
        self.assertTrue(state['closed'])

    def test_transient_query_after_upload_still_cleans(self):
        state, output = self.exercise('query-transient')
        self.assertEqual(state['items'], [{'id': 'foreign', 'account_id': 'other'}], output)
        self.assertTrue(state['closed'])

    def test_preexisting_tenant_is_not_mutated(self):
        state, _ = self.exercise('preexisting')
        self.assertEqual(state['items'], [{'id': 'old', 'account_id': 'checking'}])
        self.assertFalse(state['committed'])
        self.assertTrue(state['closed'])

    def test_offline_reports_scope_and_always_closes_browser(self):
        state, output = self.exercise('offline')
        self.assertIn('RECOVERY', output)
        self.assertIn('checking', output)
        self.assertNotIn('"PASS"', output)
        self.assertTrue(state['closed'])

    def test_delete_must_be_confirmed_by_read(self):
        state, output = self.exercise('unconfirmed-delete')
        self.assertIn('RECOVERY', output)
        self.assertNotIn('"PASS"', output)
        self.assertTrue(state['closed'])
        self.assertTrue(any(tx['id'] == 'new' for tx in state['items']))
        self.assertNotIn('"foreign"', output, 'recovery IDs must not include another account')


if __name__ == '__main__':
    unittest.main()
