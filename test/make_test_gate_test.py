import os
from pathlib import Path
import shutil
import subprocess
import tempfile
import unittest

ROOT = Path(__file__).resolve().parents[1]


class MakeTestGate(unittest.TestCase):
    def run_make(self, failed=False):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            shutil.copy(ROOT / 'Makefile', root)
            (root / 'scripts').mkdir()
            (root / 'scripts/local-dev.sh').write_text('printf "GO_GATE\\n"\n')
            (root / 'python3').write_text('#!/bin/sh\nprintf "PYTHON_GATE %s\\n" "$*"\nexit ' + ('1' if failed else '0') + '\n')
            (root / 'python3').chmod(0o755)
            return subprocess.run(['make', 'test'], cwd=root, env=dict(os.environ, PATH=str(root) + ':' + os.environ['PATH']), capture_output=True, text=True)

    def test_python_gate_runs_before_go_gate(self):
        result = self.run_make()
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertIn('PYTHON_GATE -m unittest discover -s test -p *_test.py', result.stdout)
        self.assertLess(result.stdout.index('PYTHON_GATE'), result.stdout.index('GO_GATE'))

    def test_python_gate_failure_is_not_suppressed(self):
        result = self.run_make(failed=True)
        self.assertNotEqual(result.returncode, 0)
        self.assertNotIn('GO_GATE', result.stdout)


if __name__ == '__main__':
    unittest.main()
