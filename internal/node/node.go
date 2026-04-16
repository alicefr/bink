package node

import (
	"context"
	"fmt"

	"github.com/bootc-dev/bink/internal/config"
	"github.com/bootc-dev/bink/internal/podman"
	"github.com/bootc-dev/bink/internal/virsh"
)

// Config holds node configuration options
type Config struct {
	ImagesImage string
	ClusterName string
	APIPort     int // API server port to expose (0 = auto-assign random port)
}

type Node struct {
	Name           string
	ContainerName  string
	ClusterIP      string
	ClusterMAC     string
	IsControlPlane bool
	Memory         int
	VCPUs          int
	BaseDisk       string
	ImagesImage    string
	APIPort        int // Configured API port (0 = auto-assign)
	AssignedAPIPort int // Actual assigned port after container creation

	podman *podman.Client
	virsh  *virsh.Client
}

func New(name string, isControlPlane bool) *Node {
	return NewWithConfig(name, isControlPlane, Config{})
}

func NewWithConfig(name string, isControlPlane bool, cfg Config) *Node {
	// Build container name with cluster name for uniqueness
	var containerName string
	if cfg.ClusterName != "" && cfg.ClusterName != config.DefaultNetworkName {
		// Use cluster-specific name: k8s-{cluster}-{node}
		containerName = config.ContainerNamePrefix + cfg.ClusterName + "-" + name
	} else {
		// Default: k8s-{node}
		containerName = config.ContainerNamePrefix + name
	}

	if cfg.ImagesImage == "" {
		cfg.ImagesImage = config.DefaultBootcImagesImage
	}

	// Default API port to 6443 if not specified, unless explicitly set to 0 for auto-assign
	apiPort := cfg.APIPort
	if apiPort == 0 && !isControlPlane {
		// Non-control-plane nodes don't need API port
		apiPort = 0
	} else if apiPort == 0 && isControlPlane {
		// Control plane nodes default to 6443 unless explicitly set to auto-assign
		// To auto-assign, caller must set APIPort to -1
		apiPort = config.DefaultAPIServerPort
	} else if apiPort == -1 {
		// -1 means auto-assign
		apiPort = 0
	}

	return &Node{
		Name:            name,
		ContainerName:   containerName,
		ClusterIP:       CalculateClusterIP(name),
		ClusterMAC:      CalculateClusterMAC(name),
		IsControlPlane:  isControlPlane,
		Memory:          config.DefaultMemory,
		VCPUs:           config.DefaultVCPUs,
		BaseDisk:        config.DefaultBaseDisk,
		ImagesImage:     cfg.ImagesImage,
		APIPort:         apiPort,
		AssignedAPIPort: 0, // Will be set after container creation
		podman:          podman.NewClient(),
		virsh:           virsh.NewClient(containerName),
	}
}

func (n *Node) Create(ctx context.Context) error {
	if err := n.createContainer(ctx); err != nil {
		return fmt.Errorf("creating container: %w", err)
	}

	if err := n.setupSSHKeys(ctx); err != nil {
		return fmt.Errorf("setting up SSH keys: %w", err)
	}

	if err := n.createOverlayDisk(ctx); err != nil {
		return fmt.Errorf("creating overlay disk: %w", err)
	}

	if err := n.generateCloudInit(ctx); err != nil {
		return fmt.Errorf("generating cloud-init: %w", err)
	}

	if err := n.createVM(ctx); err != nil {
		return fmt.Errorf("creating VM: %w", err)
	}

	return nil
}

func (n *Node) Exists(ctx context.Context) (bool, error) {
	return n.podman.ContainerExists(ctx, n.ContainerName)
}
