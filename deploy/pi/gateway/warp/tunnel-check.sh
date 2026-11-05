#!/bin/bash
set -euo pipefail
exec 3<>/dev/tcp/10.0.89.4/1080
printf '\x05\x01\x00' >&3
head -c 2 <&3 >/dev/null
printf '\x05\x01\x00\x03\x0fone.one.one.one\x01\xbb' >&3
[ "$(head -c 2 <&3 | od -An -tx1 | tr -d ' ')" = 0500 ]
