package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPulumiYamlExists(t *testing.T) {
	_, err := os.Stat("Pulumi.yaml")
	if err != nil {
		t.Fatalf("Pulumi.yaml not found: %v", err)
	}
}

func TestPulumiYamlProjectName(t *testing.T) {
	content, err := os.ReadFile("Pulumi.yaml")
	if err != nil {
		t.Fatalf("Failed to read Pulumi.yaml: %v", err)
	}

	if !strings.Contains(string(content), "name: wurbs") {
		t.Error("Pulumi.yaml should contain 'name: wurbs'")
	}
}

func TestPulumiYamlRuntime(t *testing.T) {
	content, err := os.ReadFile("Pulumi.yaml")
	if err != nil {
		t.Fatalf("Failed to read Pulumi.yaml: %v", err)
	}

	if !strings.Contains(string(content), "runtime: go") {
		t.Error("Pulumi.yaml should contain 'runtime: go'")
	}
}

func TestMainGoExists(t *testing.T) {
	_, err := os.Stat("main.go")
	if err != nil {
		t.Fatalf("main.go not found: %v", err)
	}
}

func TestMainGoContainsRalphNamespace(t *testing.T) {
	content, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatalf("Failed to read main.go: %v", err)
	}

	if !strings.Contains(string(content), `"ralph-wurbs"`) {
		t.Error("main.go should contain ralph-wurbs namespace")
	}
}

func TestMainGoContainsWurbsNamespace(t *testing.T) {
	content, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatalf("Failed to read main.go: %v", err)
	}

	if !strings.Contains(string(content), `"wurbs"`) {
		t.Error("main.go should contain wurbs namespace")
	}
}

func TestMainGoContainsSecret(t *testing.T) {
	content, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatalf("Failed to read main.go: %v", err)
	}

	if !strings.Contains(string(content), "postgres-secret") {
		t.Error("main.go should contain postgres-secret")
	}
}

func TestMainGoContainsPostgresCluster(t *testing.T) {
	content, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatalf("Failed to read main.go: %v", err)
	}

	if !strings.Contains(string(content), "postgres-cluster") {
		t.Error("main.go should contain postgres-cluster")
	}
}

func TestMainGoContainsCloudNativePG(t *testing.T) {
	content, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatalf("Failed to read main.go: %v", err)
	}

	if !strings.Contains(string(content), "postgresql.cnpg.io/v1") {
		t.Error("main.go should contain CloudNativePG API version")
	}

	if !strings.Contains(string(content), "Cluster") {
		t.Error("main.go should contain Cluster kind")
	}
}

func TestMainGoUsesKubernetesProvider(t *testing.T) {
	content, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatalf("Failed to read main.go: %v", err)
	}

	if !strings.Contains(string(content), "kubernetes.NewProvider") {
		t.Error("main.go should use kubernetes provider")
	}
}

func TestMainGoCreatesUsernamePasswordSecret(t *testing.T) {
	content, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatalf("Failed to read main.go: %v", err)
	}

	if !strings.Contains(string(content), "username") {
		t.Error("main.go should create username in secret")
	}

	if !strings.Contains(string(content), "password") {
		t.Error("main.go should create password in secret")
	}
}

func TestInfraDirectoryExists(t *testing.T) {
	info, err := os.Stat(".")
	if err != nil {
		t.Fatalf("Failed to stat infra directory: %v", err)
	}

	if !info.IsDir() {
		t.Error("infra should be a directory")
	}
}

func TestGoModExists(t *testing.T) {
	_, err := os.Stat("go.mod")
	if err != nil {
		t.Fatalf("go.mod not found: %v", err)
	}
}

func TestGoModContainsPulumiKubernetes(t *testing.T) {
	content, err := os.ReadFile("go.mod")
	if err != nil {
		t.Fatalf("Failed to read go.mod: %v", err)
	}

	if !strings.Contains(string(content), "pulumi-kubernetes") {
		t.Error("go.mod should contain pulumi-kubernetes")
	}
}

func TestGoModContainsPulumiSDK(t *testing.T) {
	content, err := os.ReadFile("go.mod")
	if err != nil {
		t.Fatalf("Failed to read go.mod: %v", err)
	}

	if !strings.Contains(string(content), "pulumi/pulumi") {
		t.Error("go.mod should contain pulumi/pulumi")
	}
}

func TestMainGoPostgresClusterInWurbsNamespace(t *testing.T) {
	content, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatalf("Failed to read main.go: %v", err)
	}

	clusterSpec := strings.Contains(string(content), "wurbs-postgres")
	if !clusterSpec {
		t.Error("main.go should create postgres cluster named wurbs-postgres")
	}
}

func TestMainGoClusterHasStorage(t *testing.T) {
	content, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatalf("Failed to read main.go: %v", err)
	}

	if !strings.Contains(string(content), "storage") {
		t.Error("main.go should configure storage for the cluster")
	}
}

func TestAbsolutePath(t *testing.T) {
	absPath, err := filepath.Abs(".")
	if err != nil {
		t.Fatalf("Failed to get absolute path: %v", err)
	}

	if !strings.Contains(absPath, "infra") {
		t.Error("Test should run from infra directory")
	}
}
