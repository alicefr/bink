package helpers

import (
	"fmt"
	"os/exec"
	"strings"

	. "github.com/onsi/gomega"
)

// SSHExec executes a command on a node via SSH through the container
func SSHExec(clusterName, nodeName, command string) string {
	// Build container name with cluster prefix if not default
	var containerName string
	if clusterName != "" && clusterName != "podman" {
		containerName = fmt.Sprintf("k8s-%s-%s", clusterName, nodeName)
	} else {
		containerName = fmt.Sprintf("k8s-%s", nodeName)
	}

	// Build SSH command to run inside the container
	sshCmd := fmt.Sprintf("ssh -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null -i /var/run/cluster/cluster.key -p 2222 core@localhost '%s'", command)

	// Execute via podman exec
	cmd := exec.Command("podman", "exec", containerName, "sh", "-c", sshCmd)
	output, err := cmd.CombinedOutput()

	// Filter out SSH warnings
	lines := strings.Split(string(output), "\n")
	var filtered []string
	for _, line := range lines {
		if !strings.Contains(line, "Warning:") {
			filtered = append(filtered, line)
		}
	}
	cleanOutput := strings.TrimSpace(strings.Join(filtered, "\n"))

	Expect(err).ToNot(HaveOccurred(), "SSH command failed: %s", cleanOutput)
	return cleanOutput
}

// SSHExecQuiet executes a command but doesn't fail on errors
// Returns output and error
func SSHExecQuiet(clusterName, nodeName, command string) (string, error) {
	// Build container name with cluster prefix if not default
	var containerName string
	if clusterName != "" && clusterName != "podman" {
		containerName = fmt.Sprintf("k8s-%s-%s", clusterName, nodeName)
	} else {
		containerName = fmt.Sprintf("k8s-%s", nodeName)
	}

	sshCmd := fmt.Sprintf("ssh -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null -i /var/run/cluster/cluster.key -p 2222 core@localhost '%s'", command)

	cmd := exec.Command("podman", "exec", containerName, "sh", "-c", sshCmd)
	output, err := cmd.CombinedOutput()

	// Filter out SSH warnings
	lines := strings.Split(string(output), "\n")
	var filtered []string
	for _, line := range lines {
		if !strings.Contains(line, "Warning:") {
			filtered = append(filtered, line)
		}
	}
	cleanOutput := strings.TrimSpace(strings.Join(filtered, "\n"))

	return cleanOutput, err
}
