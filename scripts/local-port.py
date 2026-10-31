#!/usr/bin/env python3
"""Linux-only development launcher: fail closed on unknown listener ownership."""
from pathlib import Path
import socket
import sys


def verify(mode, port, pid):
    if mode not in ('preflight', 'owned') or not 1 <= port <= 65535 or pid < 0:
        raise ValueError('invalid port probe arguments')
    listeners = []
    for protocol in ('tcp', 'tcp6'):
        for line in Path('/proc/net/' + protocol).read_text().splitlines()[1:]:
            fields = line.split()
            address, local_port = fields[1].split(':')
            if fields[3] == '0A' and int(local_port, 16) == port:
                listeners.append((protocol, address, fields[9]))
    sockets = set()
    if pid:
        for fd in Path(f'/proc/{pid}/fd').iterdir():
            try:
                sockets.add(fd.readlink().as_posix())
            except FileNotFoundError:
                continue
    if listeners:
        if not pid or any(protocol != 'tcp' or address != '0100007F' or f'socket:[{inode}]' not in sockets
                          for protocol, address, inode in listeners):
            raise RuntimeError(f'Port {port} occupied by a foreign or non-loopback listener; no process was stopped')
    elif mode == 'owned':
        raise RuntimeError(f'MainPID {pid} is not listening on 127.0.0.1:{port}')
    else:
        # Also catch sockets bound but not yet listening.
        with socket.socket() as probe:
            # Match Go's TCP listener: TIME_WAIT alone is not an active owner.
            probe.setsockopt(socket.SOL_SOCKET, socket.SO_REUSEADDR, 1)
            probe.bind(('127.0.0.1', port))


if __name__ == '__main__':
    try:
        verify(sys.argv[1], int(sys.argv[2]), int(sys.argv[3]))
    except (OSError, ValueError, RuntimeError) as error:
        print(str(error), file=sys.stderr)
        sys.exit(1)
