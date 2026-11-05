# Security policy

`veil` is experimental. Only the latest commit on `main` gets security fixes.

## Reporting a vulnerability

Report vulnerabilities privately through [GitHub private vulnerability reporting](https://github.com/joaaomanooel/veil/security/advisories/new). Do not open a public issue, pull request or discussion for them.

Include:
- the affected component (CLI or gateway) and the commit;
- the steps to reproduce;
- what an attacker gains, such as leaking the home IP, escaping the sandbox or using the gateway without permission.

Do not include real IP addresses, keys, WARP account files, tokens or other personal data. Replace them with placeholders.

You should get a first answer within 7 days. When a fix is ready, the advisory is published with credit to you, unless you prefer to stay anonymous.

## In scope

- Traffic from the sandbox that leaves without passing through the gateway, including DNS and IPv6.
- Reading or writing host files outside the workspace and the dedicated home.
- Environment variables or file descriptors leaking into the sandbox.
- Gateway behavior that allows non-allowlisted destinations, direct egress without WARP, or exposure of the WARP account.

## Out of scope

The limits listed under [Threat model](README.md#threat-model), such as Cloudflare seeing the home IP, sessions linked by account or content, and kernel exploits.
