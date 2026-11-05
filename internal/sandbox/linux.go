package sandbox

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// BwrapPath is fixed so that a planted bwrap earlier in PATH cannot replace the sandbox.
var BwrapPath = "/usr/bin/bwrap"

// OfflineConfig represents a closed-network execution profile.
// This is a test harness, not a production-grade anonymity boundary.
type OfflineConfig struct {
	Workspace  string
	Home       string
	Executable string
	Args       []string
}

// Plan contains an executable and arguments. It has no side effects.
type Plan struct {
	Executable string
	Args       []string
}

// BuildOfflinePlan builds a bubblewrap invocation without launching a process.
// The sandbox sees only a writable workspace, the selected executable, and system runtimes.
func BuildOfflinePlan(cfg OfflineConfig) (Plan, error) {
	return buildPlan(cfg, nil, []string{"/tool/command"})
}

// GatewayProxy is where the bridge inside the sandbox accepts proxy connections.
const GatewayProxy = "127.0.0.1:8118"

// GatewaySocket is where the host proxy socket appears inside the sandbox.
const GatewaySocket = "/run/veil/proxy.sock"

// BuildGatewayPlan keeps the offline isolation and adds a single egress path:
// the host unix socket proxySocket, reached through `veil bridge` (helper).
func BuildGatewayPlan(cfg OfflineConfig, proxySocket string, helper string) (Plan, error) {
	if _, err := os.Stat(proxySocket); err != nil {
		return Plan{}, fmt.Errorf("proxy socket unavailable: %w", err)
	}
	if info, err := os.Stat(helper); err != nil || !info.Mode().IsRegular() || !filepath.IsAbs(helper) {
		return Plan{}, errors.New("veil helper must be an absolute path to an existing regular file")
	}
	proxyURL := "http://" + GatewayProxy
	extra := []string{
		"--ro-bind", helper, "/tool/veil",
		"--bind", proxySocket, GatewaySocket,
	}
	if _, err := os.Stat("/etc/ssl/certs"); err == nil {
		extra = append(extra, "--ro-bind", "/etc/ssl/certs", "/etc/ssl/certs")
	}
	for _, name := range []string{"HTTPS_PROXY", "HTTP_PROXY", "https_proxy", "http_proxy"} {
		extra = append(extra, "--setenv", name, proxyURL)
	}
	extra = append(extra, "--setenv", "NO_PROXY", "127.0.0.1,localhost", "--setenv", "no_proxy", "127.0.0.1,localhost")
	return buildPlan(cfg, extra, []string{"/tool/veil", "bridge", "--", "/tool/command"})
}

func buildPlan(cfg OfflineConfig, extra []string, command []string) (Plan, error) {
	if runtime.GOOS != "linux" {
		return Plan{}, errors.New("offline backend is currently Linux-only; refusing unisolated execution")
	}
	if info, err := os.Stat(BwrapPath); err != nil || !info.Mode().IsRegular() || info.Mode().Perm()&0o111 == 0 {
		return Plan{}, fmt.Errorf("bubblewrap (bwrap) not found at %s: refusing unisolated execution", BwrapPath)
	}
	workspace, err := safeWorkspace(cfg.Workspace)
	if err != nil {
		return Plan{}, err
	}
	homeMount := []string{"--dir", "/home/veil"}
	if cfg.Home != "" {
		home, err := dedicatedHome(cfg.Home)
		if err != nil {
			return Plan{}, err
		}
		homeMount = []string{"--bind", home, "/home/veil"}
	}
	if cfg.Executable == "" {
		return Plan{}, errors.New("missing executable")
	}
	executable, err := exec.LookPath(cfg.Executable)
	if err != nil {
		return Plan{}, fmt.Errorf("executable not found: %w", err)
	}
	executable, err = filepath.EvalSymlinks(executable)
	if err != nil {
		return Plan{}, fmt.Errorf("cannot resolve executable: %w", err)
	}
	executable, err = filepath.Abs(executable)
	if err != nil {
		return Plan{}, err
	}
	info, err := os.Stat(executable)
	if err != nil || !info.Mode().IsRegular() {
		return Plan{}, errors.New("executable must resolve to an existing regular file")
	}

	args := []string{
		"--unshare-all", "--die-with-parent", "--new-session", "--clearenv",
		"--hostname", "veil-test",
	}
	// Bind only the system libraries/binaries needed by common Linux commands.
	// Do not mount /etc, /home, /var, or the host's network namespace.
	for _, p := range []string{"/usr", "/bin", "/sbin", "/lib", "/lib64"} {
		if _, err := os.Stat(p); err == nil {
			args = append(args, "--ro-bind", p, p)
		}
	}
	args = append(args,
		"--proc", "/proc",
		"--dev", "/dev",
		"--tmpfs", "/tmp",
		"--tmpfs", "/run",
		"--tmpfs", "/home",
	)
	args = append(args, homeMount...)
	args = append(args,
		"--dir", "/tool",
		"--ro-bind", executable, "/tool/command",
		"--bind", workspace, "/workspace",
		"--chdir", "/workspace",
		"--setenv", "HOME", "/home/veil",
		"--setenv", "PATH", "/usr/bin:/bin",
		"--setenv", "TERM", "xterm-256color",
		"--setenv", "LANG", "C.UTF-8",
		"--setenv", "XDG_CONFIG_HOME", "/home/veil/.config",
		"--setenv", "XDG_CACHE_HOME", "/home/veil/.cache",
		"--setenv", "XDG_DATA_HOME", "/home/veil/.local/share",
	)
	args = append(args, extra...)
	args = append(args, "--")
	args = append(args, command...)
	args = append(args, cfg.Args...)
	return Plan{Executable: BwrapPath, Args: args}, nil
}

func safeWorkspace(dir string) (string, error) {
	if dir == "" {
		dir = "."
	}
	actual, err := filepath.EvalSymlinks(dir)
	if err != nil {
		return "", fmt.Errorf("cannot resolve workspace: %w", err)
	}
	actual, err = filepath.Abs(actual)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(actual)
	if err != nil || !info.IsDir() {
		return "", errors.New("workspace must be an existing directory")
	}
	if actual == "/" {
		return "", errors.New("refusing to mount entire filesystem as workspace")
	}
	if home, err := os.UserHomeDir(); err == nil {
		if homeAbs, err := filepath.EvalSymlinks(home); err == nil {
			if actual == homeAbs || isParent(actual, homeAbs) {
				return "", errors.New("refusing to mount home directory or its parent as workspace")
			}
			if isInsideHiddenHomeDir(homeAbs, actual) {
				return "", errors.New("refusing to mount a hidden directory inside home (such as ~/.ssh or ~/.config) as workspace")
			}
		}
	}
	return actual, nil
}

func dedicatedHome(dir string) (string, error) {
	actual, err := filepath.EvalSymlinks(dir)
	if err != nil {
		return "", fmt.Errorf("cannot resolve dedicated home: %w", err)
	}
	actual, err = filepath.Abs(actual)
	if err != nil {
		return "", err
	}
	if info, err := os.Stat(actual); err != nil || !info.IsDir() {
		return "", errors.New("dedicated home must be an existing directory")
	}
	realHome, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("cannot determine real home: %w", err)
	}
	realHome, err = filepath.EvalSymlinks(realHome)
	if err != nil {
		return "", fmt.Errorf("cannot resolve real home: %w", err)
	}
	if actual == "/" || actual == realHome || isParent(actual, realHome) {
		return "", errors.New("refusing to mount the real home directory or its parent as the sandbox home")
	}
	if isInsideHiddenHomeDir(realHome, actual) {
		return "", errors.New("refusing to mount a hidden directory inside home (such as ~/.ssh or ~/.config) as the sandbox home")
	}
	return actual, nil
}

func isInsideHiddenHomeDir(home, dir string) bool {
	rel, err := filepath.Rel(home, dir)
	if err != nil {
		return false
	}
	first := strings.SplitN(rel, string(filepath.Separator), 2)[0]
	return first != "." && first != ".." && strings.HasPrefix(first, ".")
}

func isParent(parent, child string) bool {
	rel, err := filepath.Rel(parent, child)
	return err == nil && rel != ".." && rel != "." && !filepath.IsAbs(rel) && rel[:min(len(rel), 3)] != "../"
}
