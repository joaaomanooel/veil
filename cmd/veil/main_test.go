package main

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"testing"

	"github.com/joaaomanooel/veil/internal/sandbox"
)

func TestMain(m *testing.M) {
	// Inside the gateway sandbox this test binary is mounted as /tool/veil and must act as veil.
	if filepath.Base(os.Args[0]) == "veil" {
		os.Exit(run(os.Args[1:]))
	}
	os.Exit(m.Run())
}

func useFakeBwrap(t *testing.T, script string) {
	t.Helper()
	fake := filepath.Join(t.TempDir(), "bwrap")
	if err := os.WriteFile(fake, []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	original := sandbox.BwrapPath
	sandbox.BwrapPath = fake
	t.Cleanup(func() { sandbox.BwrapPath = original })
}

func TestRun_RejectsInvalidCommands(t *testing.T) {
	// Arrange: no execution required.
	cases := [][]string{
		nil, {"bogus"}, {"run"}, {"run", "--profile", "unknown", "--", "echo"},
		{"run", "--missing-flag"},
		{"run", "--profile", "gateway", "--", "echo"},
		{"run", "--profile", "offline", "--gateway", "127.0.0.1:8118", "--", "echo"},
		{"bridge"}, {"bridge", "echo"},
	}
	for _, args := range cases {
		// Act
		code := run(args)
		// Assert
		if code != 2 {
			t.Errorf("expected usage error 2 for %v, got %d", args, code)
		}
	}
}

func TestRun_DoctorSucceeds(t *testing.T) {
	// Arrange
	// Act
	code := run([]string{"doctor"})
	// Assert
	if code != 0 {
		t.Fatalf("unexpected doctor status: %d", code)
	}
}

func TestRun_FailsClosedWithoutSandbox(t *testing.T) {
	// Arrange
	original := sandbox.BwrapPath
	sandbox.BwrapPath = filepath.Join(t.TempDir(), "missing-bwrap")
	t.Cleanup(func() { sandbox.BwrapPath = original })
	// Act
	code := run([]string{"run", "--profile", "offline", "--workspace", t.TempDir(), "--", "/bin/echo", "hello"})
	// Assert
	if code == 0 {
		t.Fatal("must not execute without sandbox")
	}
}

func TestRun_PreservesIsolatedExitCode(t *testing.T) {
	// Arrange: fake bwrap exits 17; the host command itself is never run.
	if runtime.GOOS != "linux" {
		t.Skip("Linux sandbox implementation")
	}
	useFakeBwrap(t, "#!/bin/sh\nexit 17\n")
	// Act
	code := run([]string{"run", "--profile", "offline", "--workspace", t.TempDir(), "--", "/bin/echo", "hello"})
	// Assert
	if code != 17 {
		t.Fatalf("got %d, want 17", code)
	}
}

func TestRun_ReportsSignalDeathAs128PlusSignal(t *testing.T) {
	// Arrange: fake bwrap is killed by SIGKILL, as the OOM killer would do.
	if runtime.GOOS != "linux" {
		t.Skip("Linux sandbox implementation")
	}
	useFakeBwrap(t, "#!/bin/sh\nkill -9 $$\n")
	// Act
	code := run([]string{"run", "--profile", "offline", "--workspace", t.TempDir(), "--", "/bin/echo"})
	// Assert
	if code != 128+int(syscall.SIGKILL) {
		t.Fatalf("got %d, want %d", code, 128+int(syscall.SIGKILL))
	}
}

func TestRun_SucceedsOnlyWhenSandboxExecutorSucceeds(t *testing.T) {
	// Arrange: fake bwrap succeeds, allowing verification of the successful execution path.
	if runtime.GOOS != "linux" {
		t.Skip("Linux sandbox implementation")
	}
	useFakeBwrap(t, "#!/bin/sh\nexit 0\n")

	// Act
	code := run([]string{"run", "--profile", "offline", "--workspace", t.TempDir(), "--", "/bin/echo"})

	// Assert
	if code != 0 {
		t.Fatalf("got status %d, expected success", code)
	}
}

func TestRun_FailsWhenSandboxBinaryCannotStart(t *testing.T) {
	// Arrange: executable bwrap mock with a non-existent interpreter; not a child exit status.
	if runtime.GOOS != "linux" {
		t.Skip("Linux sandbox implementation")
	}
	useFakeBwrap(t, "#!/missing/veil-interpreter\n")

	// Act
	code := run([]string{"run", "--profile", "offline", "--workspace", t.TempDir(), "--", "/bin/echo"})

	// Assert
	if code != 1 {
		t.Fatalf("got status %d, expected sandbox startup error 1", code)
	}
}

func TestRun_GatewayFailsClosedWhenProxyUnreachable(t *testing.T) {
	// Arrange
	if runtime.GOOS != "linux" {
		t.Skip("Linux sandbox implementation")
	}
	marker := filepath.Join(t.TempDir(), "sandbox-started")
	useFakeBwrap(t, "#!/bin/sh\ntouch "+marker+"\n")

	// Act
	code := run([]string{"run", "--profile", "gateway", "--gateway", "127.0.0.1:1", "--workspace", t.TempDir(), "--", "/bin/echo"})

	// Assert
	if code == 0 {
		t.Fatal("must not succeed without a reachable gateway")
	}
	if _, err := os.Stat(marker); err == nil {
		t.Fatal("sandbox started although the gateway is unreachable")
	}
}

func trustGateway(t *testing.T) {
	t.Helper()
	original := verifyGateway
	verifyGateway = func(gateway string) (string, error) { return gateway, nil }
	t.Cleanup(func() { verifyGateway = original })
}

func requireRealBwrap(t *testing.T) {
	t.Helper()
	if runtime.GOOS != "linux" {
		t.Skip("Linux sandbox implementation")
	}
	if _, err := os.Stat(sandbox.BwrapPath); err != nil {
		t.Skip("bubblewrap not installed at " + sandbox.BwrapPath)
	}
}

func TestRun_PersistsSandboxHomeInDedicatedDirectory(t *testing.T) {
	// Arrange
	requireRealBwrap(t)
	home := t.TempDir()

	// Act
	code := run([]string{"run", "--home", home, "--workspace", t.TempDir(), "--", "/bin/sh", "-c", `touch "$HOME/credentials"`})

	// Assert
	if code != 0 {
		t.Fatalf("sandboxed command failed with status %d", code)
	}
	if _, err := os.Stat(filepath.Join(home, "credentials")); err != nil {
		t.Fatalf("file written to the sandbox HOME did not persist: %v", err)
	}
}

func TestGatewaySandbox_ReachesProxyThroughSocket(t *testing.T) {
	// Arrange
	requireRealBwrap(t)
	trustGateway(t)
	proxy, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer proxy.Close()
	go func() {
		for {
			conn, err := proxy.Accept()
			if err != nil {
				return
			}
			_, _ = conn.Write([]byte("HTTP/1.1 200 Connection established\r\n\r\n"))
			conn.Close()
		}
	}()
	workspace := t.TempDir()
	script := `exec 3<>/dev/tcp/127.0.0.1/8118 || exit 20
printf 'CONNECT opencode.ai:443 HTTP/1.1\r\n\r\n' >&3
head -n 1 <&3 > /workspace/reply
printf '%s' "$HTTPS_PROXY" > /workspace/proxy-env`

	// Act
	code := run([]string{"run", "--profile", "gateway", "--gateway", proxy.Addr().String(), "--workspace", workspace, "--", "/bin/bash", "-c", script})

	// Assert
	if code != 0 {
		t.Fatalf("sandboxed client failed with status %d", code)
	}
	reply, _ := os.ReadFile(filepath.Join(workspace, "reply"))
	if !strings.HasPrefix(string(reply), "HTTP/1.1 200") {
		t.Fatalf("got reply %q, want the host proxy answer", reply)
	}
	env, _ := os.ReadFile(filepath.Join(workspace, "proxy-env"))
	if string(env) != "http://127.0.0.1:8118" {
		t.Fatalf("got HTTPS_PROXY %q", env)
	}
}

func TestGatewaySandbox_BlocksDirectConnections(t *testing.T) {
	// Arrange
	requireRealBwrap(t)
	trustGateway(t)
	proxy, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer proxy.Close()
	workspace := t.TempDir()
	script := `(exec 3<>/dev/tcp/1.1.1.1/443) 2> /workspace/direct-error && exit 30
exit 0`

	// Act
	code := run([]string{"run", "--profile", "gateway", "--gateway", proxy.Addr().String(), "--workspace", workspace, "--", "/bin/bash", "-c", script})

	// Assert
	if code != 0 {
		t.Fatalf("direct connection from the sandbox was not blocked (status %d)", code)
	}
	direct, _ := os.ReadFile(filepath.Join(workspace, "direct-error"))
	if !strings.Contains(string(direct), "unreachable") {
		t.Fatalf("got %q, want a network unreachable error", direct)
	}
}

func TestForwardSignals_DeliversSignalToSandboxedChildReportedByBwrap(t *testing.T) {
	// Arrange
	sandboxed := exec.Command("sleep", "30")
	if err := sandboxed.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sandboxed.Process.Kill() })
	infoR, infoW, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	fmt.Fprintf(infoW, `{"child-pid": %d}`, sandboxed.Process.Pid)
	infoW.Close()
	signals := make(chan os.Signal, 1)
	signals <- syscall.SIGTERM
	close(signals)

	// Act
	forwardSignals(signals, infoR, nil)

	// Assert
	err = sandboxed.Wait()
	status, ok := sandboxed.ProcessState.Sys().(syscall.WaitStatus)
	if !ok || !status.Signaled() || status.Signal() != syscall.SIGTERM {
		t.Fatalf("expected sandboxed child to die from SIGTERM, got %v", err)
	}
}
