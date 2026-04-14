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

	podman *podman.Client
	virsh  *virsh.Client
}

func New(name string, isControlPlane bool) *Node {
	return NewWithConfig(name, isControlPlane, Config{})
}

func NewWithConfig(name string, isControlPlane bool, cfg Config) *Node {
	containerName := config.ContainerNamePrefix + name

	if cfg.ImagesImage == "" {
		cfg.ImagesImage = config.DefaultBootcImagesImage
	}

	return &Node{
		Name:           name,
		ContainerName:  containerName,
		ClusterIP:      CalculateClusterIP(name),
		ClusterMAC:     CalculateClusterMAC(name),
		IsControlPlane: isControlPlane,
		Memory:         config.DefaultMemory,
		VCPUs:          config.DefaultVCPUs,
		BaseDisk:       config.DefaultBaseDisk,
		ImagesImage:    cfg.ImagesImage,
		podman:         podman.NewClient(),
		virsh:          virsh.NewClient(containerName),
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
