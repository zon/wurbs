package maketest_test

import (
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// repoRoot returns the absolute path to the repository root.
func repoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	require.True(t, ok, "could not determine test file path")
	return filepath.Join(filepath.Dir(file), "..")
}

// dryRun executes make -n for a target and returns the output.
func dryRun(t *testing.T, target string) string {
	t.Helper()
	root := repoRoot(t)
	cmd := exec.Command("make", "-n", "-f", filepath.Join(root, "Makefile"), target)
	cmd.Dir = root
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "make -n %s failed: %s", target, string(out))
	return string(out)
}

func TestMakefile_RestTarget_UsesAir(t *testing.T) {
	out := dryRun(t, "rest")
	assert.Contains(t, out, "air", "rest target should use air")
}

func TestMakefile_RestTarget_BuildsServer(t *testing.T) {
	out := dryRun(t, "rest")
	assert.Contains(t, out, "./server", "rest target should build the server directory")
}

func TestMakefile_RestTarget_TestMode(t *testing.T) {
	out := dryRun(t, "rest")
	assert.Contains(t, out, "--test", "rest target should pass --test flag")
}

func TestMakefile_SocketTarget_UsesAir(t *testing.T) {
	out := dryRun(t, "socket")
	assert.Contains(t, out, "air", "socket target should use air")
}

func TestMakefile_SocketTarget_BuildsSocketServer(t *testing.T) {
	out := dryRun(t, "socket")
	assert.Contains(t, out, "./socketserver", "socket target should build the socketserver directory")
}

func TestMakefile_SocketTarget_TestMode(t *testing.T) {
	out := dryRun(t, "socket")
	assert.Contains(t, out, "--test", "socket target should pass --test flag")
}

func TestMakefile_InstallTarget_GoInstall(t *testing.T) {
	out := dryRun(t, "install")
	assert.Contains(t, out, "go install", "install target should run go install")
}

func TestMakefile_InstallTarget_Wurbctl(t *testing.T) {
	out := dryRun(t, "install")
	assert.Contains(t, out, "wurbctl", "install target should install wurbctl")
}

func TestMakefile_TestTarget_GoTest(t *testing.T) {
	out := dryRun(t, "test")
	assert.Contains(t, out, "go test", "test target should run go test")
}

func TestMakefile_TestTarget_AllPackages(t *testing.T) {
	out := dryRun(t, "test")
	assert.Contains(t, out, "./...", "test target should run all packages")
}

func TestMakefile_PushTarget_DockerBuild(t *testing.T) {
	out := dryRun(t, "push")
	assert.Contains(t, out, "docker build", "push target should build a docker image")
}

func TestMakefile_PushTarget_DockerPush(t *testing.T) {
	out := dryRun(t, "push")
	assert.Contains(t, out, "docker push", "push target should push the docker image")
}

func TestMakefile_PushTarget_DockerLogin(t *testing.T) {
	out := dryRun(t, "push")
	assert.Contains(t, out, "docker login", "push target should login to docker")
}

func TestMakefile_PushTarget_TokenFromFile(t *testing.T) {
	out := dryRun(t, "push")
	assert.Contains(t, out, ".docker-token", "push target should read token from .docker-token file")
}

func TestMakefile_PushTarget_ImageTag(t *testing.T) {
	out := dryRun(t, "push")
	assert.Contains(t, out, "zvonimir/wurbs", "push target should use the correct image name")
}

func TestMakefile_RestTarget_WatchesCoreDir(t *testing.T) {
	out := dryRun(t, "rest")
	assert.Contains(t, out, "core", "rest target should watch the core directory")
}

func TestMakefile_SocketTarget_WatchesCoreDir(t *testing.T) {
	out := dryRun(t, "socket")
	assert.Contains(t, out, "core", "socket target should watch the core directory")
}

func TestMakefile_RestAndSocketTargets_DifferentBinaries(t *testing.T) {
	restOut := dryRun(t, "rest")
	socketOut := dryRun(t, "socket")
	// They should build different binaries.
	restLines := strings.TrimSpace(restOut)
	socketLines := strings.TrimSpace(socketOut)
	assert.NotEqual(t, restLines, socketLines, "rest and socket targets should produce different commands")
}
