import os
from pathlib import Path
import shutil
import subprocess
import tempfile
import unittest

ROOT = Path(__file__).resolve().parents[1]


class LauncherTest(unittest.TestCase):
    def run_launcher(self, scenario='new-config', port='8090'):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            (root / 'scripts').mkdir()
            (root / 'bin').mkdir()
            shutil.copy(ROOT / 'scripts/local-dev.sh', root / 'scripts')
            (root / 'scripts/local-port.py').write_text('import os,sys\nsys.exit(1 if os.environ["SCENARIO"] == "occupied" or (os.environ["SCENARIO"] == "foreign-ready" and sys.argv[1] == "owned") else 0)\n')
            (root / 'bin/api').write_text('#!/bin/sh\nexit 0\n')
            (root / 'bin/api').chmod(0o755)
            config = root / 'new.env'
            config.write_text(f'DATABASE_URL=synthetic\nPORT={port}\n')
            controls = root / 'controls'
            controls.mkdir()
            programs = {
                'systemctl': '''#!/bin/bash
printf '%s\\n' "$*" >> "$LOG"
if [[ "$SCENARIO" == show-fails && "$*" == *show* ]]; then exit 1; fi
if [[ "$SCENARIO" == confirm-show-fails && "$*" == *MainPID* && -f "$MARKER" ]]; then echo 4242; exit 1; fi
if [[ "$SCENARIO" == stop-fails && "$*" == *stop* ]]; then exit 1; fi
case "$*" in
 *LoadState*) if [[ "$SCENARIO" == missing-unit ]]; then echo not-found; exit 1; elif [[ -f "$STATE" ]]; then echo not-found; else echo loaded; fi;;
 *MainPID*) if [[ "$SCENARIO" == pid-changed && -f "$MARKER" ]]; then echo 5555; else echo 4242; fi;;
 *stop*) touch "$STATE";;
 *is-active*) exit 0;;
esac
''',
                'systemd-run': '#!/bin/bash\nprintf "%s\\n" "$*" >> "$LOG"\n[[ "$SCENARIO" != run-fails ]]\n',
                'curl': '#!/bin/sh\ntouch "$MARKER"\n[ "$SCENARIO" != curl-fails ] || exit 1\necho healthy\n',
                'sleep': '#!/bin/sh\nexit 0\n',
            }
            for name, body in programs.items():
                p = controls / name
                p.write_text(body)
                p.chmod(0o755)
            env = dict(os.environ, PATH=str(controls) + ':' + os.environ['PATH'], ENV_FILE=str(config), TOOLS_ENV='/nonexistent', LOG=str(root / 'log'), STATE=str(root / 'stopped'), MARKER=str(root / 'curl-done'), SCENARIO=scenario)
            result = subprocess.run(['bash', str(root / 'scripts/local-dev.sh'), 'up'], env=env, capture_output=True, text=True)
            return result, (root / 'log').read_text() if (root / 'log').exists() else '', str(config)

    def test_active_unit_uses_new_environment_file(self):
        result, log, config = self.run_launcher()
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertIn('EnvironmentFile=' + config, log)
        self.assertNotIn('restart', log)

    def test_validated_defaults_are_not_left_to_manager_environment(self):
        result, log, _ = self.run_launcher(port='8091')
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertIn('--setenv=BIND_ADDR=127.0.0.1', log)
        self.assertIn('--setenv=ENVIRONMENT=development', log)
        self.assertIn('--setenv=PORT=8091', log)

    def test_command_failures_cannot_report_success(self):
        for scenario in ('show-fails', 'stop-fails', 'run-fails', 'curl-fails', 'confirm-show-fails'):
            with self.subTest(scenario=scenario):
                result, _, _ = self.run_launcher(scenario)
                self.assertNotEqual(result.returncode, 0)

    def test_missing_unit_can_be_created(self):
        result, log, _ = self.run_launcher('missing-unit')
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertIn('--collect', log)
        self.assertNotIn('stop ', log)

    def test_healthy_foreign_responder_is_not_readiness(self):
        result, _, _ = self.run_launcher('foreign-ready')
        self.assertNotEqual(result.returncode, 0)

    def test_occupied_port_does_not_stop_any_unit(self):
        result, log, _ = self.run_launcher('occupied')
        self.assertNotEqual(result.returncode, 0)
        self.assertNotIn('stop ', log)
        self.assertNotIn('--collect', log)

    def test_invalid_port_is_rejected_before_service_commands(self):
        for port in ('0', '65536', 'abc', '8090/readyz'):
            with self.subTest(port=port):
                result, log, _ = self.run_launcher(port=port)
                self.assertNotEqual(result.returncode, 0)
                self.assertNotIn('stop ', log)
                self.assertNotIn('--collect', log)


if __name__ == '__main__':
    unittest.main()
