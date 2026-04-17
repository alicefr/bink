package helpers

import (
	"encoding/json"
	"os/exec"
	"strings"

	. "github.com/onsi/gomega"
)

// ContainerInfo holds basic container information
type ContainerInfo struct {
	ID     string
	Name   string
	State  string
	Ports  []string
	Labels map[string]string
}

// PodmanCmd executes a podman command and returns output
func PodmanCmd(args ...string) string {
	cmd := exec.Command("podman", args...)
	output, err := cmd.CombinedOutput()
	Expect(err).ToNot(HaveOccurred(), "podman command failed: %s", string(output))
	return string(output)
}

// PodmanExec executes a command inside a container
func PodmanExec(container, command string) string {
	cmd := exec.Command("podman", "exec", container, "sh", "-c", command)
	output, err := cmd.CombinedOutput()
	Expect(err).ToNot(HaveOccurred(), "podman exec failed: %s", string(output))
	return string(output)
}

// GetContainer returns information about a container
// Returns nil if container doesn't exist
// For test usage, name should be the full container name (e.g., "k8s-test-bink-abc123-node1")
func GetContainer(name string) *ContainerInfo {
	cmd := exec.Command("podman", "container", "inspect", name)
	output, err := cmd.CombinedOutput()
	if err != nil {
		// Container doesn't exist
		return nil
	}

	var containers []map[string]interface{}
	if err := json.Unmarshal(output, &containers); err != nil {
		return nil
	}

	if len(containers) == 0 {
		return nil
	}

	c := containers[0]
	info := &ContainerInfo{
		Labels: make(map[string]string),
	}

	if id, ok := c["Id"].(string); ok {
		info.ID = id
	}
	if name, ok := c["Name"].(string); ok {
		info.Name = name
	}
	if state, ok := c["State"].(map[string]interface{}); ok {
		if status, ok := state["Status"].(string); ok {
			info.State = status
		}
	}

	// Parse ports
	if networkSettings, ok := c["NetworkSettings"].(map[string]interface{}); ok {
		if ports, ok := networkSettings["Ports"].(map[string]interface{}); ok {
			for port := range ports {
				info.Ports = append(info.Ports, port)
			}
		}
	}

	// Parse labels
	if config, ok := c["Config"].(map[string]interface{}); ok {
		if labels, ok := config["Labels"].(map[string]interface{}); ok {
			for k, v := range labels {
				if str, ok := v.(string); ok {
					info.Labels[k] = str
				}
			}
		}
	}

	return info
}

// GetContainerID returns the ID of a container by name
func GetContainerID(name string) string {
	cmd := exec.Command("podman", "container", "inspect", name, "--format", "{{.ID}}")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(output))
}

// ContainerExists checks if a container exists
func ContainerExists(name string) bool {
	return GetContainer(name) != nil
}

// GetVolume checks if a volume exists
func GetVolume(name string) bool {
	cmd := exec.Command("podman", "volume", "inspect", name)
	err := cmd.Run()
	return err == nil
}
