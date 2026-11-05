#!/bin/bash
set -euo pipefail
[ -f wgcf-account.toml ] || wgcf register --accept-tos
[ -f wgcf-profile.conf ] || wgcf generate
sed -i -e '/^DNS/d' -e 's/^Endpoint = engage\.cloudflareclient\.com:/Endpoint = 162.159.192.1:/' wgcf-profile.conf
