# veil

`veil` runs an AI coding CLI in a sandbox, so the tool's vendor does not see your home IP address and the tool only sees the files you choose. It wraps any command: you run `veil run ... -- <tool>` when you want the extra privacy, and run the tool directly otherwise.

The sandbox has no network of its own. In the `gateway` profile, its only way out is an allowlisting proxy that you host. The proxy sends traffic through Cloudflare WARP, so the vendor sees a Cloudflare address instead of yours. If the gateway is missing, `veil` refuses to start the tool. It never falls back to a direct connection.

OpenCode is the first tool tested end to end, and the examples below use it. Any CLI that honors `HTTPS_PROXY` can work the same way.

> **Status: experimental.** This is not an anonymity tool, and it has not had an external security review. Read [Threat model](#threat-model) and [Known limitations](#known-limitations) before relying on it.

## How it works

```
host                                   sandbox (bubblewrap, no network)
veil run ──bind──> /run/veil/proxy.sock <── veil bridge 127.0.0.1:8118 <── tool (HTTPS_PROXY)
   │
   └── TCP ──> gateway: Privoxy (domain allowlist) ──> wireproxy (SOCKS) ──> Cloudflare WARP ──> vendor
```

- **Linux sandbox.** `veil` uses [bubblewrap](https://github.com/containers/bubblewrap) to unshare all namespaces and clear the environment. It mounts the system directories read-only, one workspace read-write at `/workspace`, and an empty or dedicated `HOME`.
- **Bridge.** The sandbox sees only a unix socket. A copy of `veil` inside the sandbox exposes it as `127.0.0.1:8118` and sets `HTTP_PROXY`, `HTTPS_PROXY` and `NO_PROXY=127.0.0.1,localhost`.
- **Gateway.** A Docker Compose stack allows only the domains you list and sends them through a WARP WireGuard tunnel. DNS is resolved inside the tunnel, never by the client.

## Platforms

| Platform | Status |
|---|---|
| Linux | Supported, with bubblewrap and unprivileged user namespaces. Tested end to end on arm64. |
| macOS | Planned: a Seatbelt backend that allows only the local proxy |
| Windows | Not supported; see [#3](https://github.com/joaaomanooel/veil/issues/3) |

On other systems `veil run` refuses to start. You can still run the Linux build of `veil` and of your tool inside a Linux VM or container.

## Install

Requirements:
- Go 1.22 or later
- Linux with `bwrap` installed at `/usr/bin/bwrap`
- Unprivileged user namespaces enabled

```sh
git clone https://github.com/joaaomanooel/veil.git
cd veil
CGO_ENABLED=0 go build -o veil ./cmd/veil
./veil doctor
```

Build with `CGO_ENABLED=0`, because the same static binary also runs inside the sandbox. `veil doctor` reports whether `bwrap` and `opencode` are on the `PATH`. It runs nothing and reads no credentials.

## Usage

```
veil run [--profile offline|gateway] [--gateway HOST:PORT] [--workspace DIR] [--home DIR] -- COMMAND [ARGS...]
```

| Flag | Default | Meaning |
|---|---|---|
| `--profile` | `offline` | `offline` has no network at all. `gateway` allows only the gateway. |
| `--gateway` | none | `HOST:PORT` of the gateway proxy. It is required with `--profile gateway`, and only then. |
| `--workspace` | current directory | The only host directory mounted read-write. |
| `--home` | empty, discarded | A dedicated directory mounted as `HOME`, so logins and tool state survive between runs. |

`veil` refuses `/`, your real home directory and its parents as a workspace or home. It also refuses hidden directories directly under your home as a workspace. Never run it with `sudo`.

### Example: OpenCode through the gateway

```sh
mkdir -p ~/veil-homes/opencode
./veil run --profile gateway --gateway gateway.local:8118 \
  --workspace ~/projects/demo --home ~/veil-homes/opencode \
  -- opencode run "add a subtract function to calc.py"
```

Replace `gateway.local` with the name or IP of the machine that runs the gateway. The dedicated home keeps OpenCode's login and configuration away from your real `HOME`.

### Example: offline smoke test

```sh
mkdir -p /tmp/veil-demo
./veil run --workspace /tmp/veil-demo -- /bin/sh -c 'echo "HOME=$HOME"; hostname; ls /workspace'
```

### Using another tool

1. Find the domains the tool needs. Check its documentation, or watch the gateway's `403` responses.
2. Add the domains to the gateway allowlist (see below).
3. Use a separate `--home` directory for each tool.
4. Make sure the tool honors `HTTPS_PROXY`. A tool that ignores it cannot reach the network in the `gateway` profile: it fails instead of leaking.

## Gateway

The stack in [`deploy/pi/gateway`](deploy/pi/gateway) runs on any Docker host on your local network. It has been tested on a Raspberry Pi 3B with Ubuntu 24.04 arm64. The images build from pinned sources on the host itself.

| Service | Role |
|---|---|
| `warp-init` | Registers a WARP device on first start. The account stays in the `warp-data` volume. |
| `warp` | Runs wireproxy, the WARP WireGuard tunnel exposed as SOCKS5 on an internal network. It also runs a watchdog that restarts the tunnel when it stops answering. |
| `privoxy` | Listens on port 8118, allows only the listed domains and answers `403` to everything else. |

Before the first start, edit two files:
- `privoxy/config`: set `permit-access` to your local subnet (the example uses `192.168.0.0/24`).
- `privoxy/allowlist.action`: list your tool's domains as `.example.com:443`. The default allows `.opencode.ai:443` and `www.cloudflare.com:443`. Keep the Cloudflare entry: veil fetches its trace through the gateway before each session and refuses to start unless it reports `warp=on`.

Then start the stack and test it from a client:

```sh
cd deploy/pi/gateway
docker compose up -d --build
curl -sS -o /dev/null -w '%{http_code}\n' -x http://gateway.local:8118 https://opencode.ai/   # 200
curl -s -o /dev/null -w '%{http_connect}\n' -x http://gateway.local:8118 https://example.com/   # 403
```

Privoxy must stay on a network that is not internal, so the port can be published. To make sure it can only reach WARP, install a firewall rule that rejects new outbound connections from that network, and a service that restores the rule at boot:

```sh
sudo install -m 0755 egress-lock.sh /usr/local/sbin/veil-egress-lock
sudo install -m 0644 veil-egress-lock.service /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable --now veil-egress-lock
```

> **Warning:** the gateway has no client authentication yet. Run it only on a network you trust, and never publish port 8118 to the internet. Client authentication with mTLS is planned in [#2](https://github.com/joaaomanooel/veil/issues/2).

Never copy, print or commit the contents of the `warp-data` volume. It holds the WARP private key.

## Threat model

`veil` is designed for these goals:
- The vendor of the tool sees a Cloudflare WARP address, not your home IP.
- The tool cannot reach any host outside the allowlist, and has no network when the gateway is down.
- The tool cannot read or write host files outside the workspace and the dedicated home.
- The tool does not inherit your environment, such as API tokens, `SSH_AUTH_SOCK` or proxy variables.

`veil` does not protect against these:
- **Cloudflare.** Cloudflare sees your home IP and the traffic metadata.
- **Linking sessions.** Accounts, prompts, code and behavior can still link your sessions. The WARP exit address does not rotate.
- **Your local network.** Anyone on your local network can use or impersonate the gateway until [#2](https://github.com/joaaomanooel/veil/issues/2) is done.
- **Kernel exploits, a compromised host or privileged processes.**
- **Files left in the workspace.** Anything you put in the workspace is visible to the tool. A file the tool writes there, such as `.git/hooks/*`, `Makefile` or `.envrc`, runs on the host if you execute it later.

## Known limitations

- Linux only for now.
- bubblewrap and user namespaces depend on kernel settings. When they are unavailable, `veil` refuses to start.
- Interactive TUIs and signal handling have had little testing.
- `veil doctor` looks for `bwrap` on the `PATH`, while `veil run` requires `/usr/bin/bwrap`.

Open problems and planned work are tracked in [issues](https://github.com/joaaomanooel/veil/issues). Design decisions are recorded in [`docs/`](docs).

## Contributing and security

- See [CONTRIBUTING.md](CONTRIBUTING.md) to build, test and propose changes.
- Report vulnerabilities privately as described in [SECURITY.md](SECURITY.md), never in a public issue.

## License

[GPL-3.0](LICENSE)
