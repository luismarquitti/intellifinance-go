import os
from pathlib import Path
import socket
import subprocess
import unittest

HELPER = Path(__file__).resolve().parents[1] / 'scripts/local-port.py'


class PortTest(unittest.TestCase):
    def probe(self, mode, port, pid=0):
        return subprocess.run(['python3', str(HELPER), mode, str(port), str(pid)], capture_output=True, text=True)

    def test_listener_must_belong_to_exact_pid_and_be_loopback(self):
        with socket.socket() as sock:
            sock.bind(('127.0.0.1', 0))
            sock.listen()
            port = sock.getsockname()[1]
            self.assertEqual(self.probe('owned', port, os.getpid()).returncode, 0)
            self.assertNotEqual(self.probe('owned', port, 1).returncode, 0)
            self.assertNotEqual(self.probe('preflight', port).returncode, 0)
            self.assertEqual(self.probe('preflight', port, os.getpid()).returncode, 0)
        self.assertEqual(self.probe('preflight', port).returncode, 0)
        self.assertNotEqual(self.probe('owned', port, os.getpid()).returncode, 0)

    def test_recently_closed_reusable_listener_is_available(self):
        with socket.socket() as server:
            server.setsockopt(socket.SOL_SOCKET, socket.SO_REUSEADDR, 1)
            server.bind(('127.0.0.1', 0))
            server.listen()
            port = server.getsockname()[1]
            with socket.create_connection(('127.0.0.1', port)) as client:
                connection, _ = server.accept()
                connection.close()  # Server-side TIME_WAIT, as after a graceful API stop.
                self.assertEqual(client.recv(1), b'')
        with socket.socket() as reusable:
            reusable.setsockopt(socket.SOL_SOCKET, socket.SO_REUSEADDR, 1)
            reusable.bind(('127.0.0.1', port))
        result = self.probe('preflight', port)
        self.assertEqual(result.returncode, 0, result.stderr)

    def test_wildcard_is_not_accepted_as_loopback(self):
        with socket.socket() as sock:
            sock.bind(('0.0.0.0', 0))
            sock.listen()
            self.assertNotEqual(self.probe('owned', sock.getsockname()[1], os.getpid()).returncode, 0)


if __name__ == '__main__':
    unittest.main()
