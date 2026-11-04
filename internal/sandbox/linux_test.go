package sandbox

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func useFakeBwrap(t *testing.T, script string) string {
	t.Helper()
	fake := filepath.Join(t.TempDir(), "bwrap")
	if err := os.WriteFile(fake, []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	original := BwrapPath
	BwrapPath = fake
	t.Cleanup(func() { BwrapPath = original })
	return fake
}

func TestSafeWorkspace_RejectsRootAndHome(t *testing.T) {
	// Arrange
	home, _ := os.UserHomeDir()
	candidates := []string{"/", home}

	for _, candidate := range candidates {
		// Act
		_, err := safeWorkspace(candidate)
		// Assert
		if err == nil {
			t.Errorf("expected rejection of %q", candidate)
		}
	}
}

func TestSafeWorkspace_RejectsHiddenDirectoriesInsideHome(t *testing.T) {
	// Arrange
	home := t.TempDir()
	t.Setenv("HOME", home)
	candidates := []string{filepath.Join(home, ".ssh"), filepath.Join(home, ".config", "opencode")}
	for _, candidate := range candidates {
		if err := os.MkdirAll(candidate, 0700); err != nil {
			t.Fatal(err)
		}
	}

	for _, candidate := range candidates {
		// Act
		_, err := safeWorkspace(candidate)
		// Assert
		if err == nil {
			t.Errorf("expected rejection of %q", candidate)
		}
	}
}

func TestSafeWorkspace_AcceptsProjectInsideHome(t *testing.T) {
	// Arrange
	home := t.TempDir()
	t.Setenv("HOME", home)
	project := filepath.Join(home, "workspace", "project")
	if err := os.MkdirAll(project, 0700); err != nil {
		t.Fatal(err)
	}

	// Act
	actual, err := safeWorkspace(project)

	// Assert
	if err != nil {
		t.Fatal(err)
	}
	expected, _ := filepath.EvalSymlinks(project)
	if actual != expected {
		t.Fatalf("got %q want %q", actual, expected)
	}
}

func TestBuildOfflinePlan_RejectsMissingBubblewrap(t *testing.T) {
	// Arrange
	original := BwrapPath
	BwrapPath = filepath.Join(t.TempDir(), "missing-bwrap")
	t.Cleanup(func() { BwrapPath = original })
	cfg := OfflineConfig{Workspace: t.TempDir(), Executable: "/bin/echo"}

	// Act
	_, err := BuildOfflinePlan(cfg)

	// Assert
	if err == nil || (runtime.GOOS == "linux" && !strings.Contains(err.Error(), "bubblewrap")) {
		t.Fatalf("expected fail-closed bubblewrap error, got %v", err)
	}
}

func TestBuildOfflinePlan_IgnoresBwrapPlantedInPath(t *testing.T) {
	// Arrange
	if runtime.GOOS != "linux" {
		t.Skip("Linux sandbox implementation")
	}
	fake := useFakeBwrap(t, "#!/bin/sh\nexit 0\n")
	planted := t.TempDir()
	if err := os.WriteFile(filepath.Join(planted, "bwrap"), []byte("#!/bin/sh\nexit 0\n"), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", planted)

	// Act
	plan, err := BuildOfflinePlan(OfflineConfig{Workspace: t.TempDir(), Executable: "/bin/echo"})

	// Assert
	if err != nil {
		t.Fatal(err)
	}
	if plan.Executable != fake {
		t.Fatalf("got sandbox executor %q, want fixed path %q", plan.Executable, fake)
	}
}

func TestBuildOfflinePlan_ConstructsIsolatedInvocation(t *testing.T) {
	// Arrange
	if runtime.GOOS != "linux" {
		t.Skip("Linux sandbox implementation")
	}
	useFakeBwrap(t, "#!/bin/sh\nexit 0\n")
	workspace := t.TempDir()
	cfg := OfflineConfig{Workspace: workspace, Executable: "/bin/echo", Args: []string{"hello"}}

	// Act
	plan, err := BuildOfflinePlan(cfg)

	// Assert
	if err != nil {
		t.Fatal(err)
	}
	joined := " " + strings.Join(plan.Args, " ") + " "
	for _, item := range []string{" --unshare-all ", " --clearenv ", " --new-session ", " --die-with-parent ", " --tmpfs /home ", " --setenv HOME /home/veil ", " --bind " + workspace + " /workspace ", " -- /tool/command hello "} {
		if !strings.Contains(joined, item) {
			t.Errorf("expected argument sequence %q", item)
		}
	}
	if strings.Contains(joined, " --ro-bind /home ") {
		t.Fatal("host HOME bind must be absent")
	}
}

func TestBuildOfflinePlan_MountsDedicatedHome(t *testing.T) {
	// Arrange
	if runtime.GOOS != "linux" {
		t.Skip("Linux sandbox implementation")
	}
	useFakeBwrap(t, "#!/bin/sh\nexit 0\n")
	home := t.TempDir()
	cfg := OfflineConfig{Workspace: t.TempDir(), Home: home, Executable: "/bin/echo"}

	// Act
	plan, err := BuildOfflinePlan(cfg)

	// Assert
	if err != nil {
		t.Fatal(err)
	}
	joined := " " + strings.Join(plan.Args, " ") + " "
	if !strings.Contains(joined, " --bind "+home+" /home/veil ") {
		t.Fatalf("expected dedicated home bind in %q", joined)
	}
	if strings.Contains(joined, " --dir /home/veil ") {
		t.Fatal("dedicated home must replace the empty home directory")
	}
}

func TestBuildOfflinePlan_RejectsRealHomeAsDedicatedHome(t *testing.T) {
	// Arrange
	if runtime.GOOS != "linux" {
		t.Skip("Linux sandbox implementation")
	}
	useFakeBwrap(t, "#!/bin/sh\nexit 0\n")
	realHome := t.TempDir()
	t.Setenv("HOME", realHome)
	workspace := filepath.Join(realHome, "project")
	if err := os.MkdirAll(workspace, 0700); err != nil {
		t.Fatal(err)
	}

	for _, candidate := range []string{realHome, filepath.Dir(realHome), "/"} {
		// Act
		_, err := BuildOfflinePlan(OfflineConfig{Workspace: workspace, Home: candidate, Executable: "/bin/echo"})
		// Assert
		if err == nil {
			t.Errorf("expected rejection of %q as dedicated home", candidate)
		}
	}
}

func TestBuildGatewayPlan_KeepsNetworkUnsharedAndMountsOnlyTheProxySocket(t *testing.T) {
	// Arrange
	if runtime.GOOS != "linux" {
		t.Skip("Linux sandbox implementation")
	}
	fake := useFakeBwrap(t, "#!/bin/sh\nexit 0\n")
	socket := filepath.Join(t.TempDir(), "proxy.sock")
	if err := os.WriteFile(socket, nil, 0600); err != nil {
		t.Fatal(err)
	}
	cfg := OfflineConfig{Workspace: t.TempDir(), Executable: "/bin/echo", Args: []string{"hello"}}

	// Act
	plan, err := BuildGatewayPlan(cfg, socket, fake)

	// Assert
	if err != nil {
		t.Fatal(err)
	}
	joined := " " + strings.Join(plan.Args, " ") + " "
	for _, item := range []string{
		" --unshare-all ", " --ro-bind " + fake + " /tool/veil ", " --bind " + socket + " /run/veil/proxy.sock ",
		" --setenv HTTPS_PROXY http://127.0.0.1:8118 ", " --setenv NO_PROXY 127.0.0.1,localhost ",
		" -- /tool/veil bridge -- /tool/command hello ",
	} {
		if !strings.Contains(joined, item) {
			t.Errorf("expected argument sequence %q", item)
		}
	}
	if strings.Contains(joined, " --share-net ") {
		t.Fatal("gateway sandbox must not share the host network")
	}
}

func TestBuildGatewayPlan_RejectsMissingSocketOrHelper(t *testing.T) {
	// Arrange
	if runtime.GOOS != "linux" {
		t.Skip("Linux sandbox implementation")
	}
	fake := useFakeBwrap(t, "#!/bin/sh\nexit 0\n")
	cfg := OfflineConfig{Workspace: t.TempDir(), Executable: "/bin/echo"}
	missing := filepath.Join(t.TempDir(), "missing")

	// Act
	_, socketErr := BuildGatewayPlan(cfg, missing, fake)
	_, helperErr := BuildGatewayPlan(cfg, fake, missing)

	// Assert
	if socketErr == nil || helperErr == nil {
		t.Fatalf("expected rejection, got socket=%v helper=%v", socketErr, helperErr)
	}
}

func TestBuildOfflinePlan_RejectsBadInputs(t *testing.T) {
	// Arrange
	if runtime.GOOS != "linux" {
		t.Skip("Linux sandbox implementation")
	}
	fake := useFakeBwrap(t, "#!/bin/sh\nexit 0\n")
	base := OfflineConfig{Workspace: t.TempDir(), Executable: "/bin/echo"}
	cases := []struct {
		name   string
		change func(*OfflineConfig)
	}{
		{"no executable", func(c *OfflineConfig) { c.Executable = "" }},
		{"missing executable", func(c *OfflineConfig) { c.Executable = "/nonexistent/veil-exe" }},
		{"missing workspace", func(c *OfflineConfig) { c.Workspace = "/nonexistent/veil-workspace" }},
		{"non-directory workspace", func(c *OfflineConfig) { c.Workspace = fake }},
		{"root workspace", func(c *OfflineConfig) { c.Workspace = "/" }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// Arrange
			cfg := base
			tc.change(&cfg)
			// Act
			_, err := BuildOfflinePlan(cfg)
			// Assert
			if err == nil {
				t.Fatal("expected an input validation error")
			}
		})
	}
}

func TestSafeWorkspace_EmptyDefaultsToCurrentProject(t *testing.T) {
	// Arrange
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}

	// Act
	actual, err := safeWorkspace("")

	// Assert
	if err != nil {
		t.Fatal(err)
	}
	expected, err := filepath.EvalSymlinks(cwd)
	if err != nil {
		t.Fatal(err)
	}
	if actual != expected {
		t.Fatalf("got %q, expected %q", actual, expected)
	}
}

func TestOfflineSandbox_HidesHostFilesSecretsAndNetworkFromCommandAndItsChildren(t *testing.T) {
	// Arrange
	if runtime.GOOS != "linux" {
		t.Skip("Linux sandbox implementation")
	}
	if _, err := os.Stat(BwrapPath); err != nil {
		t.Skip("bubblewrap not installed at " + BwrapPath)
	}
	hostFile := filepath.Join(t.TempDir(), "host-secret")
	if err := os.WriteFile(hostFile, []byte("secret"), 0600); err != nil {
		t.Fatal(err)
	}
	realHome, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("SSH_AUTH_SOCK", hostFile)
	t.Setenv("VEIL_TEST_API_TOKEN", "secret")
	workspace := t.TempDir()
	script := `set -e
[ ! -e "$1" ] || { echo "host file outside workspace is visible"; exit 10; }
[ ! -e "$2" ] || { echo "real home is visible"; exit 11; }
[ -z "$SSH_AUTH_SOCK$VEIL_TEST_API_TOKEN" ] || { echo "inherited secrets are visible"; exit 12; }
[ "$(tail -n +2 /proc/net/route; grep -v ' lo$' /proc/net/ipv6_route)" = "" ] || { echo "network route exists"; exit 13; }
sh -c '[ ! -e "$1" ]' child "$1" || { echo "child process sees host file"; exit 14; }
touch /workspace/written-inside-sandbox`
	plan, err := BuildOfflinePlan(OfflineConfig{
		Workspace: workspace, Executable: "/bin/sh", Args: []string{"-c", script, "sh", hostFile, realHome},
	})
	if err != nil {
		t.Fatal(err)
	}

	// Act
	output, err := exec.Command(plan.Executable, plan.Args...).CombinedOutput()

	// Assert
	if err != nil {
		t.Fatalf("sandbox check failed: %v\n%s", err, output)
	}
	if _, err := os.Stat(filepath.Join(workspace, "written-inside-sandbox")); err != nil {
		t.Fatalf("workspace write did not reach the host: %v", err)
	}
}
