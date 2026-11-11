"""Guardas de launcher: nunca chegam a binário/serviço/banco protegido."""
import os
from pathlib import Path
import subprocess
import tempfile
import unittest

ROOT=Path(__file__).resolve().parents[1]

class ProtectedLauncherTest(unittest.TestCase):
    def test_refuse_personal_before_mutating_commands(self):
        for action in ('up','seed','test'):
            for mode,url in (
                ('personal','postgres://personal_app@127.0.0.1:55433/intellifinance_personal'),
                ('development','postgres://app@127.0.0.1:55433/disguised_test'),
                ('development','postgres://app@127.0.0.1:55432/intellifinance_personal_test'),
            ):
                with self.subTest(action=action,mode=mode,url=url), tempfile.TemporaryDirectory() as tmp:
                    p=Path(tmp); envfile=p/'qa.env'
                    envfile.write_text(f"DATABASE_URL='{url}'\nENVIRONMENT={mode}\n")
                    controls=p/'controls';controls.mkdir()
                    marker=p/'called'
                    for command in ('go','systemctl','systemd-run'):
                        exe=controls/command
                        exe.write_text('#!/bin/sh\ntouch "$GUARD_MARKER"\nexit 0\n');exe.chmod(0o755)
                    env=dict(os.environ,ENV_FILE=str(envfile),TEST_ENV_FILE=str(envfile),TOOLS_ENV='/nonexistent',PATH=str(controls)+':'+os.environ['PATH'],GUARD_MARKER=str(marker))
                    result=subprocess.run(['bash',str(ROOT/'scripts/local-dev.sh'),action],env=env,text=True,capture_output=True)
                    self.assertNotEqual(result.returncode,0)
                    self.assertFalse(marker.exists(),'protected target reached a mutating command')
                    self.assertIn('protected',result.stderr.lower())

if __name__=='__main__':
    unittest.main()
