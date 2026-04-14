package node

import (
	"testing"
)

func TestCalculateClusterIP(t *testing.T) {
	tests := []struct {
		nodeName string
		expected string
	}{
		{"node1", "10.0.0.32"},
		{"node2", "10.0.0.130"},
		{"node3", "10.0.0.29"},
	}

	for _, tt := range tests {
		t.Run(tt.nodeName, func(t *testing.T) {
			result := CalculateClusterIP(tt.nodeName)
			if result != tt.expected {
				t.Errorf("CalculateClusterIP(%q) = %q, want %q", tt.nodeName, result, tt.expected)
			}
		})
	}
}

func TestCalculateClusterMAC(t *testing.T) {
	tests := []struct {
		nodeName string
	}{
		{"node1"},
		{"node2"},
		{"node3"},
	}

	for _, tt := range tests {
		t.Run(tt.nodeName, func(t *testing.T) {
			result := CalculateClusterMAC(tt.nodeName)
			if len(result) != 17 {
				t.Errorf("CalculateClusterMAC(%q) = %q, invalid length", tt.nodeName, result)
			}
			if result[:8] != "52:54:01" {
				t.Errorf("CalculateClusterMAC(%q) = %q, wrong prefix", tt.nodeName, result)
			}
		})
	}
}
