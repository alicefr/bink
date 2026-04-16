package helpers

import (
	"os"
	"os/exec"
	"strings"

	. "github.com/onsi/gomega"
)

// RequireCommand verifies that a command exists on the system
func RequireCommand(cmd string) {
	_, err := exec.LookPath(cmd)
	Expect(err).ToNot(HaveOccurred(), "%s command not found in PATH", cmd)
}

// RequireBink verifies that the bink binary exists in the project root
func RequireBink() {
	// Check from project root (two levels up from test/integration)
	_, err := os.Stat("../../bink")
	Expect(err).ToNot(HaveOccurred(), "bink binary not found. Run 'make build-bink' first")
}

// RequireImage verifies that a container image exists
func RequireImage(image string) {
	cmd := exec.Command("podman", "image", "exists", image)
	err := cmd.Run()
	Expect(err).ToNot(HaveOccurred(), "Image %s not found. Run 'make build-cluster-image' and 'make build-images-container'", image)
}

// CleanupAllTestClusters removes all test clusters (containers with prefix test-bink-)
func CleanupAllTestClusters() {
	// List all k8s-* containers
	cmd := exec.Command("podman", "ps", "-a", "--filter", "name=k8s-", "--format", "{{.Names}}")
	output, err := cmd.CombinedOutput()
	if err != nil {
		// Ignore errors - may be no containers
		return
	}

	containers := strings.Split(strings.TrimSpace(string(output)), "\n")
	for _, name := range containers {
		if name == "" {
			continue
		}
		// Only remove test containers
		if strings.HasPrefix(name, "k8s-test-") {
			rmCmd := exec.Command("podman", "rm", "-f", name)
			_ = rmCmd.Run() // Ignore errors
		}
	}

	// Clean up test volumes
	volCmd := exec.Command("podman", "volume", "ls", "--filter", "name=test-", "--format", "{{.Name}}")
	volOutput, err := volCmd.CombinedOutput()
	if err == nil {
		volumes := strings.Split(strings.TrimSpace(string(volOutput)), "\n")
		for _, vol := range volumes {
			if vol != "" && strings.HasPrefix(vol, "test-") {
				rmVolCmd := exec.Command("podman", "volume", "rm", "-f", vol)
				_ = rmVolCmd.Run() // Ignore errors
			}
		}
	}
}
