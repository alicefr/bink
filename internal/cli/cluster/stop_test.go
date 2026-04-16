package cluster

import (
	"testing"

	"github.com/bootc-dev/bink/internal/config"
)

func TestHasPrefix(t *testing.T) {
	tests := []struct {
		name     string
		s        string
		prefix   string
		expected bool
	}{
		{"exact match", "k8s-", "k8s-", true},
		{"string with prefix", "k8s-node1", "k8s-", true},
		{"string without prefix", "node1", "k8s-", false},
		{"empty string", "", "k8s-", false},
		{"empty prefix", "k8s-node1", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := hasPrefix(tt.s, tt.prefix)
			if result != tt.expected {
				t.Errorf("hasPrefix(%q, %q) = %v, want %v", tt.s, tt.prefix, result, tt.expected)
			}
		})
	}
}

func TestIsDefaultClusterContainer(t *testing.T) {
	tests := []struct {
		name          string
		containerName string
		expected      bool
	}{
		// Default cluster containers (should return true)
		{"default cluster node1", "k8s-node1", true},
		{"default cluster node2", "k8s-node2", true},
		{"default cluster control", "k8s-control", true},

		// Named cluster containers (should return false)
		{"named cluster mytest-node1", "k8s-mytest-node1", false},
		{"named cluster prod-node1", "k8s-prod-node1", false},
		{"named cluster dev-control", "k8s-dev-control", false},

		// Edge cases
		{"no k8s prefix", "node1", false},
		{"empty string", "", false},
		{"just k8s prefix", "k8s-", true}, // No hyphen in remainder
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isDefaultClusterContainer(tt.containerName)
			if result != tt.expected {
				t.Errorf("isDefaultClusterContainer(%q) = %v, want %v", tt.containerName, result, tt.expected)
			}
		})
	}
}

func TestContainerFiltering(t *testing.T) {
	// Simulate what the filtering logic should do
	allContainers := []string{
		"k8s-node1",           // default cluster
		"k8s-node2",           // default cluster
		"k8s-mytest-node1",    // named cluster "mytest"
		"k8s-mytest-node2",    // named cluster "mytest"
		"k8s-prod-control",    // named cluster "prod"
		"other-container",     // not a k8s container
	}

	t.Run("filter default cluster", func(t *testing.T) {
		var defaultClusterContainers []string
		for _, container := range allContainers {
			if hasPrefix(container, config.ContainerNamePrefix) && isDefaultClusterContainer(container) {
				defaultClusterContainers = append(defaultClusterContainers, container)
			}
		}

		expected := []string{"k8s-node1", "k8s-node2"}
		if len(defaultClusterContainers) != len(expected) {
			t.Errorf("Expected %d containers, got %d", len(expected), len(defaultClusterContainers))
		}

		for i, container := range defaultClusterContainers {
			if container != expected[i] {
				t.Errorf("Expected container %q at position %d, got %q", expected[i], i, container)
			}
		}
	})

	t.Run("filter named cluster mytest", func(t *testing.T) {
		clusterName := "mytest"
		prefix := config.ContainerNamePrefix + clusterName + "-"
		var namedClusterContainers []string
		for _, container := range allContainers {
			if hasPrefix(container, prefix) {
				namedClusterContainers = append(namedClusterContainers, container)
			}
		}

		expected := []string{"k8s-mytest-node1", "k8s-mytest-node2"}
		if len(namedClusterContainers) != len(expected) {
			t.Errorf("Expected %d containers, got %d", len(expected), len(namedClusterContainers))
		}

		for i, container := range namedClusterContainers {
			if container != expected[i] {
				t.Errorf("Expected container %q at position %d, got %q", expected[i], i, container)
			}
		}
	})

	t.Run("filter named cluster prod", func(t *testing.T) {
		clusterName := "prod"
		prefix := config.ContainerNamePrefix + clusterName + "-"
		var namedClusterContainers []string
		for _, container := range allContainers {
			if hasPrefix(container, prefix) {
				namedClusterContainers = append(namedClusterContainers, container)
			}
		}

		expected := []string{"k8s-prod-control"}
		if len(namedClusterContainers) != len(expected) {
			t.Errorf("Expected %d containers, got %d", len(expected), len(namedClusterContainers))
		}

		for i, container := range namedClusterContainers {
			if container != expected[i] {
				t.Errorf("Expected container %q at position %d, got %q", expected[i], i, container)
			}
		}
	})
}
