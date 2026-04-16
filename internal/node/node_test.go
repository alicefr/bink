package node

import (
	"testing"

	"github.com/bootc-dev/bink/internal/config"
)

func TestNodeAPIPortConfiguration(t *testing.T) {
	tests := []struct {
		name           string
		nodeName       string
		isControlPlane bool
		cfg            Config
		expectedPort   int
	}{
		{
			name:           "control plane with default port",
			nodeName:       "node1",
			isControlPlane: true,
			cfg:            Config{},
			expectedPort:   config.DefaultAPIServerPort,
		},
		{
			name:           "control plane with custom port",
			nodeName:       "node1",
			isControlPlane: true,
			cfg:            Config{APIPort: 7443},
			expectedPort:   7443,
		},
		{
			name:           "control plane with auto-assign (-1)",
			nodeName:       "node1",
			isControlPlane: true,
			cfg:            Config{APIPort: -1},
			expectedPort:   0, // 0 means auto-assign in the code
		},
		{
			name:           "worker node should have no API port",
			nodeName:       "node2",
			isControlPlane: false,
			cfg:            Config{},
			expectedPort:   0,
		},
		{
			name:           "worker node with specified port (ignored)",
			nodeName:       "node2",
			isControlPlane: false,
			cfg:            Config{APIPort: 7443},
			expectedPort:   7443, // Actually kept, but not used since not control plane
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			node := NewWithConfig(tt.nodeName, tt.isControlPlane, tt.cfg)
			if node.APIPort != tt.expectedPort {
				t.Errorf("Expected APIPort = %d, got %d", tt.expectedPort, node.APIPort)
			}
		})
	}
}

func TestNodeContainerNamingWithCluster(t *testing.T) {
	tests := []struct {
		name              string
		nodeName          string
		clusterName       string
		expectedContainer string
	}{
		{
			name:              "default cluster",
			nodeName:          "node1",
			clusterName:       "",
			expectedContainer: "k8s-node1",
		},
		{
			name:              "named cluster",
			nodeName:          "node1",
			clusterName:       "test-cluster",
			expectedContainer: "k8s-test-cluster-node1",
		},
		{
			name:              "default network name treated as default",
			nodeName:          "node1",
			clusterName:       config.DefaultNetworkName,
			expectedContainer: "k8s-node1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			node := NewWithConfig(tt.nodeName, true, Config{ClusterName: tt.clusterName})
			if node.ContainerName != tt.expectedContainer {
				t.Errorf("Expected ContainerName = %q, got %q", tt.expectedContainer, node.ContainerName)
			}
		})
	}
}
