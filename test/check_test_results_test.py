#!/usr/bin/env python3
"""Regression tests for the strict Go-test event gate."""
import json
import os
from pathlib import Path
import subprocess
import tempfile
import unittest

SCRIPT = Path(__file__).resolve().parents[1] / 'scripts/check-test-results.py'


class TestGoResultGate(unittest.TestCase):
    def run_gate(self, events):
        with tempfile.TemporaryDirectory(dir=os.environ.get('TMPDIR')) as directory:
            env = dict(os.environ, TEST_RESULTS_PATH=str(Path(directory) / 'events.jsonl'))
            return subprocess.run(['python3', str(SCRIPT)], input=''.join(json.dumps(e) + '\n' for e in events),
                                  capture_output=True, text=True, env=env, check=False)

    def test_package_without_tests_is_not_a_skipped_test(self):
        result = self.run_gate([{'Action': 'pass', 'Package': 'app', 'Test': 'TestReal'},
                                {'Action': 'pass', 'Package': 'app'},
                                {'Action': 'skip', 'Package': 'generated'}])
        self.assertEqual(0, result.returncode, result.stderr)

    def test_skipped_integration_test_fails(self):
        result = self.run_gate([{'Action': 'skip', 'Package': 'db', 'Test': 'TestIntegration'},
                                {'Action': 'pass', 'Package': 'db'}])
        self.assertNotEqual(0, result.returncode)

    def test_failed_package_fails(self):
        self.assertNotEqual(0, self.run_gate([{'Action': 'fail', 'Package': 'app'}]).returncode)

    def test_empty_stream_fails(self):
        self.assertNotEqual(0, self.run_gate([]).returncode)


if __name__ == '__main__':
    unittest.main()
