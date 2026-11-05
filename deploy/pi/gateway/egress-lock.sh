#!/bin/sh
set -eu
rule="-i veil-edge ! -o veil-edge -m conntrack --ctstate NEW -j REJECT"
iptables -C DOCKER-USER $rule 2>/dev/null || iptables -I DOCKER-USER $rule
