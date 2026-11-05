#!/bin/bash
set -euo pipefail
cookie=$(od -An -tx1 /var/lib/tor-control/cookie | tr -d ' \n')
exec 3<>/dev/tcp/127.0.0.1/9051
printf 'AUTHENTICATE %s\r\nSIGNAL NEWNYM\r\nQUIT\r\n' "$cookie" >&3
reply=$(tr -d '\r' <&3)
if [ "$(grep -c '^250 OK$' <<<"$reply")" -lt 2 ]; then
  echo "rotate failed: $reply" >&2
  exit 1
fi
echo "rotate: new Tor identity requested"
