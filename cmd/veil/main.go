package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"syscall"
	"time"

	"github.com/joaaomanooel/veil/internal/bridge"
	"github.com/joaaomanooel/veil/internal/sandbox"
	"github.com/joaaomanooel/veil/internal/warpcheck"
)

func main() { os.Exit(run(os.Args[1:])) }

func run(args []string) int {
	if len(args) == 0 {
		usage()
		return 2
	}
	switch args[0] {
	case "doctor":
		fmt.Printf("Host: %s/%s\n", runtime.GOOS, runtime.GOARCH)
		for _, name := range []string{"bwrap", "opencode"} {
			if path, err := exec.LookPath(name); err == nil {
				fmt.Printf("%s: %s\n", name, path)
			} else {
				fmt.Printf("%s: not found\n", name)
			}
		}
		fmt.Println("No application was executed, and no credentials were read.")
		return 0
	case "run":
		fs := flag.NewFlagSet("run", flag.ContinueOnError)
		fs.SetOutput(os.Stderr)
		profile := fs.String("profile", "offline", "isolation profile: offline or gateway")
		gateway := fs.String("gateway", "", "HOST:PORT of the allowlisted HTTP proxy (gateway profile only)")
		cwd, _ := os.Getwd()
		workspace := fs.String("workspace", cwd, "workspace directory to mount read-write")
		home := fs.String("home", "", "dedicated directory mounted as the sandbox HOME (default: empty, discarded)")
		if err := fs.Parse(args[1:]); err != nil {
			return 2
		}
		if *profile != "offline" && *profile != "gateway" {
			fmt.Fprintln(os.Stderr, "profile unavailable: refusing execution without isolation")
			return 2
		}
		if (*profile == "gateway") != (*gateway != "") {
			fmt.Fprintln(os.Stderr, "--gateway HOST:PORT is required with, and only with, --profile gateway")
			return 2
		}
		command := fs.Args()
		if len(command) == 0 {
			fmt.Fprintln(os.Stderr, "missing command: use veil run --profile offline -- COMMAND [ARGS...]")
			return 2
		}
		cfg := sandbox.OfflineConfig{Workspace: *workspace, Home: *home, Executable: command[0], Args: command[1:]}
		if *profile == "gateway" {
			return runGateway(cfg, *gateway)
		}
		plan, err := sandbox.BuildOfflinePlan(cfg)
		if err != nil {
			fmt.Fprintln(os.Stderr, "isolation error:", err)
			return 1
		}
		fmt.Fprintln(os.Stderr, "Offline sandbox: no external network, no real HOME; workspace:", filepath.Clean(*workspace))
		return runIsolated(plan)
	case "bridge":
		return runBridge(args[1:])
	default:
		usage()
		return 2
	}
}

var verifyGateway = func(gateway string) (string, error) {
	return warpcheck.Verify(gateway, warpcheck.TraceURL, nil)
}

func runGateway(cfg sandbox.OfflineConfig, gateway string) int {
	dir, err := os.MkdirTemp("", "veil-")
	if err != nil {
		fmt.Fprintln(os.Stderr, "isolation error:", err)
		return 1
	}
	defer os.RemoveAll(dir)
	ln, err := net.Listen("unix", filepath.Join(dir, "proxy.sock"))
	if err != nil {
		fmt.Fprintln(os.Stderr, "isolation error:", err)
		return 1
	}
	defer ln.Close()
	helper, err := os.Executable()
	if err != nil {
		fmt.Fprintln(os.Stderr, "isolation error:", err)
		return 1
	}
	plan, err := sandbox.BuildGatewayPlan(cfg, ln.Addr().String(), helper)
	if err != nil {
		fmt.Fprintln(os.Stderr, "isolation error:", err)
		return 1
	}
	pinned, err := verifyGateway(gateway)
	if err != nil {
		fmt.Fprintln(os.Stderr, "refusing execution:", err)
		return 1
	}
	go bridge.Serve(ln, func() (net.Conn, error) { return net.DialTimeout("tcp", pinned, 10*time.Second) })
	fmt.Fprintln(os.Stderr, "Gateway sandbox: egress only through", gateway, "; workspace:", filepath.Clean(cfg.Workspace))
	return runIsolated(plan)
}

// runBridge runs inside the gateway sandbox: it exposes the host proxy socket
// on the loopback proxy address and then runs the isolated command.
func runBridge(args []string) int {
	if len(args) < 2 || args[0] != "--" {
		usage()
		return 2
	}
	ln, err := net.Listen("tcp", sandbox.GatewayProxy)
	if err != nil {
		fmt.Fprintln(os.Stderr, "bridge error:", err)
		return 1
	}
	defer ln.Close()
	go bridge.Serve(ln, func() (net.Conn, error) { return net.Dial("unix", sandbox.GatewaySocket) })

	child := exec.Command(args[1], args[2:]...)
	child.Stdin, child.Stdout, child.Stderr = os.Stdin, os.Stdout, os.Stderr
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM, syscall.SIGHUP)
	defer signal.Stop(signals)
	if err := child.Start(); err != nil {
		fmt.Fprintln(os.Stderr, "execution error:", err)
		return 1
	}
	go func() {
		for sig := range signals {
			_ = child.Process.Signal(sig)
		}
	}()
	return exitStatus(child.Wait())
}

func runIsolated(plan sandbox.Plan) int {
	infoR, infoW, err := os.Pipe()
	if err != nil {
		fmt.Fprintln(os.Stderr, "execution error:", err)
		return 1
	}
	defer infoR.Close()
	child := exec.Command(plan.Executable, append([]string{"--info-fd", "3"}, plan.Args...)...)
	child.Stdin, child.Stdout, child.Stderr = os.Stdin, os.Stdout, os.Stderr
	child.ExtraFiles = []*os.File{infoW}
	child.SysProcAttr = ownProcessGroup()

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM, syscall.SIGHUP)
	defer signal.Stop(signals)

	err = child.Start()
	infoW.Close()
	if err != nil {
		fmt.Fprintln(os.Stderr, "execution error:", err)
		return 1
	}
	go forwardSignals(signals, infoR, child.Process)
	return exitStatus(child.Wait())
}

func exitStatus(err error) int {
	if err == nil {
		return 0
	}
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		fmt.Fprintln(os.Stderr, "execution error:", err)
		return 1
	}
	if status, ok := exitErr.Sys().(syscall.WaitStatus); ok && status.Signaled() {
		return 128 + int(status.Signal())
	}
	return exitErr.ExitCode()
}

func forwardSignals(signals <-chan os.Signal, info *os.File, bwrap *os.Process) {
	var sandboxInfo struct {
		ChildPid int `json:"child-pid"`
	}
	_ = json.NewDecoder(info).Decode(&sandboxInfo)
	for sig := range signals {
		if sandboxInfo.ChildPid > 0 {
			_ = signalPid(sandboxInfo.ChildPid, sig)
		} else {
			_ = bwrap.Signal(sig)
		}
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, "Usage: veil doctor | veil run [--profile offline|gateway] [--gateway HOST:PORT] [--workspace DIR] [--home DIR] -- COMMAND [ARGS...]")
}
